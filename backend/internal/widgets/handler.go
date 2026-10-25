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
	PickWidgetSpec(ctx context.Context, prompt string, catalog llm.Catalog, aud sources.Audience, location string) (llm.WidgetSpec, error)
	ShapeItems(ctx context.Context, prompt string, spec llm.WidgetSpec, results []sources.Result, aud sources.Audience) ([]llm.Item, error)
}

// SourceFinder lists sources for the LLM and looks one up by name.
// *sources.Registry satisfies it.
type SourceFinder interface {
	llm.Catalog // embedding an interface adds its methods to this one
	Get(name string) (sources.Source, bool)
}

// WidgetStore saves, loads, refreshes and deletes users' custom widgets.
// *store.Store satisfies it.
type WidgetStore interface {
	CreateCustomWidget(ctx context.Context, w store.CustomWidget) (store.CustomWidget, error)
	GetCustomWidget(ctx context.Context, userID, widgetID int64) (store.CustomWidget, error)
	UpdateCustomWidgetItems(ctx context.Context, userID, widgetID int64, renderSpec json.RawMessage, refreshedAt time.Time) (store.CustomWidget, error)
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

// Handler serves the widget endpoints. Describing and refreshing a widget
// are a WidgetBuilder's work; the rest is a single store call each.
type Handler struct {
	builder *WidgetBuilder
	saved   WidgetStore
	layouts Layouts
	logger  *slog.Logger
}

// NewHandler returns a Handler that builds widgets with planner and srcs
// (telling the planner where the member lives, from profiles), keeps them in
// saved, and keeps the user's layout in step as widgets are added and
// deleted.
func NewHandler(planner Planner, srcs SourceFinder, saved WidgetStore, profiles ProfileStore, layouts Layouts, logger *slog.Logger) *Handler {
	return &Handler{
		builder: NewWidgetBuilder(planner, srcs, saved, profiles, layouts, logger),
		saved:   saved,
		layouts: layouts,
		logger:  logger,
	}
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
	// Freshness says when the items were last refreshed (when the widget
	// was built, if never) and whether they're due for a refresh.
	Freshness
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

// FromStore turns a saved custom widget back into a SavedWidget, with its
// freshness as of now.
func FromStore(cw store.CustomWidget, now time.Time) (SavedWidget, error) {
	var spec renderSpec
	if err := json.Unmarshal(cw.RenderSpec, &spec); err != nil {
		return SavedWidget{}, fmt.Errorf("decode render spec of widget %d: %w", cw.ID, err)
	}
	if spec.Items == nil {
		spec.Items = []llm.Item{}
	}
	updatedAt := cw.RefreshedAt
	if updatedAt.IsZero() { // never refreshed: the items date from when it was built
		updatedAt = cw.CreatedAt
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
		Freshness: freshness(updatedAt, now, customFreshFor(cw.Source)),
	}, nil
}

type fromPromptRequest struct {
	Prompt string `json:"prompt"`
}

// FromPrompt handles POST /api/widgets/from-prompt: it builds a widget from
// the sentence in the body for the logged-in user (see WidgetBuilder.Build),
// adding it to the end of their saved layout, and responds 201 with it. It
// must sit behind auth.RequireAuth.
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
	saved, err := h.builder.Build(r.Context(), user, prompt)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	respond.JSON(w, http.StatusCreated, saved)
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
	saved, err := FromStore(cw, time.Now())
	if err != nil {
		h.serverError(w, r, "load widget", err)
		return
	}
	respond.JSON(w, http.StatusOK, saved)
}

// RefreshCustom handles POST /api/widgets/custom/{id}/refresh: it gets new
// items for one of the logged-in user's saved widgets (see
// WidgetBuilder.Refresh) and responds 200 with the widget. A failed search
// answers with the same errors as FromPrompt, and another user's widget
// gets the same 404 as a missing one. It must sit behind auth.RequireAuth.
func (h *Handler) RefreshCustom(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeWidgetNotFound(w)
		return
	}
	refreshed, err := h.builder.Refresh(r.Context(), user, id)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	respond.JSON(w, http.StatusOK, refreshed)
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

// serverError logs err and writes a generic 500. what says what we were
// doing, e.g. "load widget".
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	if clientGone(r.Context(), err) {
		writeIfCancelled(w, r, h.logger, err)
		return
	}
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	writeInternalError(w)
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
