// Package widgets serves the widget endpoints: the kernel, POST
// /api/widgets/from-prompt, which turns a sentence into a live widget and
// saves it for the user, and the curated Articles and Local Events widgets,
// which pick their own searches from the member's interests and location.
package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// maxPromptRunes is the longest prompt we accept, counted in characters.
const maxPromptRunes = 500

// maxBodyBytes caps the request body; 8 KiB is plenty for 500 characters.
const maxBodyBytes = 8 << 10

// saveTimeout bounds saving a finished widget to the database.
const saveTimeout = 5 * time.Second

// RequestBudget caps how long one widget request (FromPrompt, Articles, or
// Events) works. Without it the worst case (two 40s attempts for each of the
// two LLM steps, plus Brave searches and their retries) is over three
// minutes. When it runs out, FromPrompt's planning fails with a 502 and its
// shaping falls back to the raw search results. The server's WriteTimeout
// must be longer than this.
const RequestBudget = 100 * time.Second

// Planner is the LLM half of the pipeline. *llm.Client satisfies it; tests
// use a fake.
type Planner interface {
	PickWidgetSpec(ctx context.Context, prompt string, catalog llm.Catalog, aud sources.Audience) (llm.WidgetSpec, error)
	ShapeItems(ctx context.Context, prompt string, spec llm.WidgetSpec, results []sources.Result, aud sources.Audience) ([]llm.Item, error)
}

// SourceFinder lists sources for the LLM and looks one up by name.
// *sources.Registry satisfies it.
type SourceFinder interface {
	llm.Catalog // embedding an interface adds its methods to this one
	Get(name string) (sources.Source, bool)
}

// WidgetStore saves, loads and deletes users' custom widgets. *store.Store
// satisfies it.
type WidgetStore interface {
	CreateCustomWidget(ctx context.Context, w store.CustomWidget) (store.CustomWidget, error)
	GetCustomWidget(ctx context.Context, userID, widgetID int64) (store.CustomWidget, error)
	DeleteCustomWidget(ctx context.Context, userID, widgetID int64) error
}

// Layouts puts newly saved widgets on the user's homepage layout, and takes
// deleted ones off it. *layout.Handler satisfies it. (The layout package
// imports this one, so this one can't import it back: Go forbids import
// cycles. An interface declared here breaks the cycle.)
type Layouts interface {
	AddCustomWidget(ctx context.Context, userID, widgetID int64) error
	RemoveCustomWidget(ctx context.Context, userID, widgetID int64) error
}

// Handler serves the widget endpoints.
type Handler struct {
	planner Planner
	sources SourceFinder
	saved   WidgetStore
	layouts Layouts
	logger  *slog.Logger
}

// NewHandler returns a Handler that builds widgets with planner and srcs,
// keeps them in saved, and keeps the user's layout in step as widgets are
// added and deleted.
func NewHandler(planner Planner, srcs SourceFinder, saved WidgetStore, layouts Layouts, logger *slog.Logger) *Handler {
	return &Handler{planner: planner, sources: srcs, saved: saved, layouts: layouts, logger: logger}
}

// Widget is a widget ready to draw: where its data came from, how to
// display it, and its items.
type Widget struct {
	Title     string        `json:"title"`
	Source    string        `json:"source"`
	Query     string        `json:"query"`
	Archetype llm.Archetype `json:"archetype"`
	Items     []llm.Item    `json:"items"` // never nil, so it encodes as [] not null
	// KidSafe is true when the content was searched and chosen for a kid
	// (see audienceFor).
	KidSafe bool `json:"kidSafe"`
}

// audienceFor says who content made for member is for: everyone who isn't a
// parent is a kid.
func audienceFor(member store.User) sources.Audience {
	if member.IsAdmin {
		return sources.Everyone
	}
	return sources.Kids
}

// SavedWidget is a Widget that belongs to a user. It's the response body of
// FromPrompt and GetCustom.
type SavedWidget struct {
	ID int64 `json:"id"`
	// Widget is embedded without a JSON tag, so encoding/json flattens its
	// fields into this object: {"id", "title", ..., "items", "createdAt"}.
	Widget
	CreatedAt time.Time `json:"createdAt"` // encodes as RFC 3339
}

// renderSpec is the JSON kept in custom_widgets.render_spec: everything
// needed to draw the widget again without searching.
type renderSpec struct {
	Title     string        `json:"title"`
	Archetype llm.Archetype `json:"archetype"`
	Items     []llm.Item    `json:"items"`
	// KidSafe is left out when false, so widgets saved before it existed
	// read the same as ones made for a parent.
	KidSafe bool `json:"kidSafe,omitempty"`
}

// FromStore turns a saved custom widget back into a SavedWidget.
func FromStore(cw store.CustomWidget) (SavedWidget, error) {
	var spec renderSpec
	if err := json.Unmarshal(cw.RenderSpec, &spec); err != nil {
		return SavedWidget{}, fmt.Errorf("decode render spec of widget %d: %w", cw.ID, err)
	}
	if spec.Items == nil {
		spec.Items = []llm.Item{}
	}
	return SavedWidget{
		ID: cw.ID,
		Widget: Widget{
			Title:     spec.Title,
			Source:    cw.Source,
			Query:     cw.Query,
			Archetype: spec.Archetype,
			Items:     spec.Items,
			KidSafe:   spec.KidSafe,
		},
		CreatedAt: cw.CreatedAt.UTC(),
	}, nil
}

type fromPromptRequest struct {
	Prompt string `json:"prompt"`
}

// FromPrompt handles POST /api/widgets/from-prompt: it asks the LLM for a
// widget spec, searches the chosen source, shapes the results into items,
// saves the widget for the logged-in user (adding it to the end of their
// saved layout), and responds 201 with it. It must sit behind
// auth.RequireAuth.
func (h *Handler) FromPrompt(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	prompt, ok := decodePrompt(w, r)
	if !ok {
		return
	}

	// One deadline for the whole pipeline, so the reply is always written
	// before the server's WriteTimeout would cut the connection.
	ctx, cancel := context.WithTimeout(r.Context(), RequestBudget)
	defer cancel()
	// The widget goes on the user's own page, so they decide what's suitable.
	aud := audienceFor(user)

	// 1. Plan: which source, what query, which archetype.
	spec, err := h.planner.PickWidgetSpec(ctx, prompt, h.sources, aud)
	if err != nil {
		h.writeLLMError(w, r, err)
		return
	}
	src, ok := h.sources.Get(spec.Source)
	if !ok { // the planner validates this, but don't trust a dependency blindly
		h.logger.WarnContext(ctx, "planner chose an unknown source", "source", spec.Source)
		respond.Error(w, http.StatusUnprocessableEntity, "Couldn't turn that into a widget.", "Couldn't pick a source for that prompt — try rephrasing")
		return
	}
	h.logger.InfoContext(ctx, "widget planned", "source", spec.Source, "query", spec.Query, "archetype", spec.Archetype, "kid_safe", aud == sources.Kids)

	// 2. Search.
	results, err := src.Search(ctx, spec.Query, aud)
	if err != nil {
		writeSearchError(w, r, h.logger, err)
		return
	}

	// 3. Shape results into items, falling back to the raw results.
	items := []llm.Item{}
	if len(results) > 0 {
		shaped, err := h.planner.ShapeItems(ctx, prompt, spec, results, aud)
		if err != nil {
			h.logger.WarnContext(ctx, "shaping failed; using raw search results", "err", err)
			items = itemsFromResults(results)
		} else {
			items = shaped
		}
	}

	// 4. Save it for the user, and respond.
	saved, err := h.save(r.Context(), user.ID, Widget{
		Title:     spec.Title,
		Source:    spec.Source,
		Query:     spec.Query,
		Archetype: spec.Archetype,
		Items:     items,
		KidSafe:   aud == sources.Kids,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "save widget", "user_id", user.ID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Your widget was ready, but we couldn't save it.", "Try again in a moment.")
		return
	}
	respond.JSON(w, http.StatusCreated, saved)
}

// save stores widget for userID and puts it at the end of their saved
// layout.
//
// By now the LLM and search calls are done and paid for, so the save uses
// neither the pipeline's deadline (which may have just run out) nor the
// request's cancellation (the browser tab may have closed): the widget will
// be on the page next time. context.WithoutCancel keeps ctx's values but
// drops its cancellation, and saveTimeout bounds the writes instead.
func (h *Handler) save(ctx context.Context, userID int64, widget Widget) (SavedWidget, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()

	spec, err := json.Marshal(renderSpec{Title: widget.Title, Archetype: widget.Archetype, Items: widget.Items, KidSafe: widget.KidSafe})
	if err != nil {
		return SavedWidget{}, fmt.Errorf("encode render spec: %w", err)
	}
	cw, err := h.saved.CreateCustomWidget(ctx, store.CustomWidget{
		UserID:     userID,
		Name:       widget.Title,
		Source:     widget.Source,
		Query:      widget.Query,
		RenderSpec: spec,
	})
	if err != nil {
		return SavedWidget{}, err
	}

	// If this fails the widget is still saved, and GET /api/me/homepage adds
	// a card for any widget the layout is missing, so it's not worth failing
	// the request over: log it and carry on.
	if err := h.layouts.AddCustomWidget(ctx, userID, cw.ID); err != nil {
		h.logger.WarnContext(ctx, "couldn't add the new widget to the layout", "user_id", userID, "widget_id", cw.ID, "err", err)
	}
	return SavedWidget{ID: cw.ID, Widget: widget, CreatedAt: cw.CreatedAt.UTC()}, nil
}

// GetCustom handles GET /api/widgets/custom/{id}: one of the logged-in
// user's saved widgets. Another user's widget gets the same 404 as a
// missing one, so ids can't be probed. It must sit behind auth.RequireAuth.
func (h *Handler) GetCustom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	// PathValue reads the {id} wildcard from the route pattern (Go 1.22+).
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeWidgetNotFound(w)
		return
	}

	cw, err := h.saved.GetCustomWidget(ctx, user.ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeWidgetNotFound(w)
		return
	case err != nil:
		h.serverError(w, r, "load widget", err)
		return
	}
	saved, err := FromStore(cw)
	if err != nil {
		h.serverError(w, r, "load widget", err)
		return
	}
	respond.JSON(w, http.StatusOK, saved)
}

// DeleteCustom handles DELETE /api/widgets/custom/{id}: it deletes one of
// the logged-in user's saved widgets for good, takes its card off their
// saved layout, and responds 204 No Content. Another user's widget gets the
// same 404 as a missing one, and is left alone. It must sit behind
// auth.RequireAuth.
//
// Deleting is its own request on purpose: a layout saved without a widget's
// card doesn't delete the widget (GET /api/me/homepage puts the card back),
// because a layout saved from an old tab could otherwise wipe out widgets
// made since.
func (h *Handler) DeleteCustom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeWidgetNotFound(w)
		return
	}

	err = h.saved.DeleteCustomWidget(ctx, user.ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeWidgetNotFound(w)
		return
	case err != nil:
		h.serverError(w, r, "delete widget", err)
		return
	}

	// The widget is gone either way; GET /api/me/homepage drops the card of
	// a widget that no longer exists, so a failure here is only logged.
	if err := h.layouts.RemoveCustomWidget(ctx, user.ID, id); err != nil {
		h.logger.WarnContext(ctx, "couldn't take the deleted widget off the layout", "user_id", user.ID, "widget_id", id, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeWidgetNotFound(w http.ResponseWriter) {
	respond.Error(w, http.StatusNotFound, "That widget doesn't exist.", "")
}

// serverError logs err and writes a generic 500. what says what we were
// doing, e.g. "load widget".
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}

// decodePrompt reads the request body and returns the trimmed prompt. On a
// bad request it writes a 400 and returns ok == false.
func decodePrompt(w http.ResponseWriter, r *http.Request) (prompt string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	var req fromPromptRequest
	err := dec.Decode(&req)
	if err == nil {
		// Decode reads one JSON value; a second call must find only EOF, or
		// the body had trailing junk.
		if err = dec.Decode(&struct{}{}); errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("more than one JSON value")
		}
	}
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		respond.Error(w, http.StatusBadRequest, "That description is too long.", "Keep it under 500 characters.")
		return "", false
	case err != nil:
		respond.Error(w, http.StatusBadRequest, `Send JSON like {"prompt": "show me upcoming SpaceX launches"}.`, "")
		return "", false
	}

	prompt = strings.TrimSpace(req.Prompt)
	switch n := utf8.RuneCountInString(prompt); {
	case n == 0:
		respond.Error(w, http.StatusBadRequest, "Describe the widget you'd like in a sentence.", `For example: "show me upcoming SpaceX launches".`)
		return "", false
	case n > maxPromptRunes:
		respond.Error(w, http.StatusBadRequest, "That description is too long.", "Keep it under 500 characters.")
		return "", false
	}
	return prompt, true
}

// writeIfCancelled handles a request whose own context has ended because the
// client went away or the server is shutting down. Neither is an upstream
// failure, so it logs at Info rather than Error. It reports whether it wrote
// a response.
func writeIfCancelled(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) bool {
	if r.Context().Err() == nil {
		return false
	}
	logger.InfoContext(r.Context(), "request cancelled before the widget was ready", "err", err)
	respond.Error(w, http.StatusServiceUnavailable, "The server stopped working on that request.", "Try again in a moment.")
	return true
}

// writeLLMError maps a planner error to an HTTP response.
func (h *Handler) writeLLMError(w http.ResponseWriter, r *http.Request, err error) {
	if writeIfCancelled(w, r, h.logger, err) {
		return
	}
	var upstream *llm.UpstreamError
	switch {
	case errors.Is(err, llm.ErrInvalidSpec):
		h.logger.WarnContext(r.Context(), "no valid widget spec", "err", err)
		respond.Error(w, http.StatusUnprocessableEntity, "Couldn't turn that into a widget.", "Couldn't pick a source for that prompt — try rephrasing")
	case errors.As(err, &upstream):
		h.logger.ErrorContext(r.Context(), "llm call failed", "err", err)
		respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", upstream.Hint)
	default:
		h.logger.ErrorContext(r.Context(), "unexpected planner error", "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
	}
}

// writeSearchError maps a search error to an HTTP response. It's a function
// rather than a Handler method so the curated widgets can share it.
func writeSearchError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	if writeIfCancelled(w, r, logger, err) {
		return
	}
	logger.ErrorContext(r.Context(), "search failed", "err", err)
	switch {
	case errors.Is(err, sources.ErrRateLimited):
		respond.Error(w, http.StatusBadGateway, "Brave Search is busy right now.", "Wait a few seconds and try again.")
	case errors.Is(err, context.DeadlineExceeded):
		respond.Error(w, http.StatusBadGateway, "Brave Search took too long to answer.", "Try again in a moment.")
	default:
		respond.Error(w, http.StatusBadGateway, "Couldn't fetch results from Brave Search.",
			"Try again in a moment. If it keeps failing, check BRAVE_API_KEY in backend/.env.")
	}
}

// itemsFromResults maps search results straight to items. It's the fallback
// when the LLM can't shape them.
func itemsFromResults(results []sources.Result) []llm.Item {
	items := make([]llm.Item, 0, min(len(results), llm.MaxItems))
	for _, r := range results {
		items = append(items, llm.Item{
			Title:     r.Title,
			URL:       r.URL,
			Snippet:   r.Snippet,
			Date:      r.Date,
			Thumbnail: r.Thumbnail,
		})
		if len(items) == llm.MaxItems {
			break
		}
	}
	return items
}
