// Package widgets serves the widget endpoints, including the kernel:
// POST /api/widgets/from-prompt, which turns a sentence into a live widget.
package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
)

// maxPromptRunes is the longest prompt we accept, counted in characters.
const maxPromptRunes = 500

// maxBodyBytes caps the request body; 8 KiB is plenty for 500 characters.
const maxBodyBytes = 8 << 10

// RequestBudget caps how long FromPrompt works on one request. Without it the
// worst case (two 40s attempts for each of the two LLM steps, plus a Brave
// search and its retry) is over three minutes. When the budget runs out,
// planning fails with a 502 and shaping falls back to the raw search results.
// The server's WriteTimeout must be longer than this.
const RequestBudget = 100 * time.Second

// Planner is the LLM half of the pipeline. *llm.Client satisfies it; tests
// use a fake.
type Planner interface {
	PickWidgetSpec(ctx context.Context, prompt string, catalog llm.Catalog) (llm.WidgetSpec, error)
	ShapeItems(ctx context.Context, prompt string, spec llm.WidgetSpec, results []sources.Result) ([]llm.Item, error)
}

// SourceFinder lists sources for the LLM and looks one up by name.
// *sources.Registry satisfies it.
type SourceFinder interface {
	llm.Catalog // embedding an interface adds its methods to this one
	Get(name string) (sources.Source, bool)
}

// Handler serves the widget endpoints.
type Handler struct {
	planner Planner
	sources SourceFinder
	logger  *slog.Logger
}

// NewHandler returns a Handler that uses planner and srcs.
func NewHandler(planner Planner, srcs SourceFinder, logger *slog.Logger) *Handler {
	return &Handler{planner: planner, sources: srcs, logger: logger}
}

// Widget is the response body of FromPrompt.
type Widget struct {
	Title     string        `json:"title"`
	Source    string        `json:"source"`
	Query     string        `json:"query"`
	Archetype llm.Archetype `json:"archetype"`
	Items     []llm.Item    `json:"items"` // never nil, so it encodes as [] not null
}

type fromPromptRequest struct {
	Prompt string `json:"prompt"`
}

// FromPrompt handles POST /api/widgets/from-prompt: it asks the LLM for a
// widget spec, searches the chosen source, shapes the results into items,
// and returns the widget.
func (h *Handler) FromPrompt(w http.ResponseWriter, r *http.Request) {
	prompt, ok := decodePrompt(w, r)
	if !ok {
		return
	}

	// One deadline for the whole pipeline, so the reply is always written
	// before the server's WriteTimeout would cut the connection.
	ctx, cancel := context.WithTimeout(r.Context(), RequestBudget)
	defer cancel()

	// 1. Plan: which source, what query, which archetype.
	spec, err := h.planner.PickWidgetSpec(ctx, prompt, h.sources)
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
	h.logger.InfoContext(ctx, "widget planned", "source", spec.Source, "query", spec.Query, "archetype", spec.Archetype)

	// 2. Search.
	results, err := src.Search(ctx, spec.Query)
	if err != nil {
		h.writeSearchError(w, r, err)
		return
	}

	// 3. Shape results into items, falling back to the raw results.
	items := []llm.Item{}
	if len(results) > 0 {
		shaped, err := h.planner.ShapeItems(ctx, prompt, spec, results)
		if err != nil {
			h.logger.WarnContext(ctx, "shaping failed; using raw search results", "err", err)
			items = itemsFromResults(results)
		} else {
			items = shaped
		}
	}

	// 4. Respond.
	respond.JSON(w, http.StatusOK, Widget{
		Title:     spec.Title,
		Source:    spec.Source,
		Query:     spec.Query,
		Archetype: spec.Archetype,
		Items:     items,
	})
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
func (h *Handler) writeIfCancelled(w http.ResponseWriter, r *http.Request, err error) bool {
	if r.Context().Err() == nil {
		return false
	}
	h.logger.InfoContext(r.Context(), "request cancelled before the widget was ready", "err", err)
	respond.Error(w, http.StatusServiceUnavailable, "The server stopped working on that request.", "Try again in a moment.")
	return true
}

// writeLLMError maps a planner error to an HTTP response.
func (h *Handler) writeLLMError(w http.ResponseWriter, r *http.Request, err error) {
	if h.writeIfCancelled(w, r, err) {
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

// writeSearchError maps a search error to an HTTP response.
func (h *Handler) writeSearchError(w http.ResponseWriter, r *http.Request, err error) {
	if h.writeIfCancelled(w, r, err) {
		return
	}
	h.logger.ErrorContext(r.Context(), "search failed", "err", err)
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
