package widgets_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/widgets"
)

// fakePlanner returns canned answers and records how it was called.
type fakePlanner struct {
	spec        llm.WidgetSpec
	specErr     error
	items       []llm.Item
	shapeErr    error
	shapeCalled bool
	hadDeadline bool // whether PickWidgetSpec's context carried a deadline
}

func (f *fakePlanner) PickWidgetSpec(ctx context.Context, _ string, _ llm.Catalog) (llm.WidgetSpec, error) {
	_, f.hadDeadline = ctx.Deadline()
	return f.spec, f.specErr
}

func (f *fakePlanner) ShapeItems(context.Context, string, llm.WidgetSpec, []sources.Result) ([]llm.Item, error) {
	f.shapeCalled = true
	return f.items, f.shapeErr
}

// fakeSource is a sources.Source with canned results.
type fakeSource struct {
	name    string
	results []sources.Result
	err     error
}

func (f fakeSource) Name() string  { return f.name }
func (f fakeSource) Shape() string { return "test results" }
func (f fakeSource) Search(context.Context, string) ([]sources.Result, error) {
	return f.results, f.err
}

var (
	launchSpec = llm.WidgetSpec{Title: "SpaceX Launches", Source: "events_search", Query: "upcoming SpaceX launches", Archetype: llm.ArchetypeList}
	results    = []sources.Result{
		{Title: "Launch schedule", URL: "https://example.com/schedule", Snippet: "Starlink next", Date: "2 days ago", Thumbnail: "https://img.example.com/1.jpg"},
	}
	shapedItems = []llm.Item{{Title: "Starlink 10-12", URL: "https://example.com/schedule", Date: "Jun 12"}}
)

func TestFromPrompt(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		planner    *fakePlanner
		source     fakeSource
		cancelled  bool // the client has gone away before the handler runs
		wantStatus int
		wantError  string     // substring of the error message, if set
		wantItems  []llm.Item // checked on 200 responses
		wantShape  bool       // whether ShapeItems should be called
	}{
		{
			name:       "success",
			body:       `{"prompt":"  show me upcoming SpaceX launches "}`,
			planner:    &fakePlanner{spec: launchSpec, items: shapedItems},
			source:     fakeSource{results: results},
			wantStatus: http.StatusOK,
			wantItems:  shapedItems,
			wantShape:  true,
		},
		{
			name:       "shaping fails, falls back to raw results",
			body:       `{"prompt":"show me upcoming SpaceX launches"}`,
			planner:    &fakePlanner{spec: launchSpec, shapeErr: llm.ErrNoItems},
			source:     fakeSource{results: results},
			wantStatus: http.StatusOK,
			wantItems: []llm.Item{{
				Title: "Launch schedule", URL: "https://example.com/schedule", Snippet: "Starlink next",
				Date: "2 days ago", Thumbnail: "https://img.example.com/1.jpg",
			}},
			wantShape: true,
		},
		{
			name:       "empty search results skip shaping",
			body:       `{"prompt":"show me upcoming SpaceX launches"}`,
			planner:    &fakePlanner{spec: launchSpec},
			source:     fakeSource{results: nil},
			wantStatus: http.StatusOK,
			wantItems:  []llm.Item{},
		},
		{
			name:       "llm can't pick a valid source",
			body:       `{"prompt":"what's the weather"}`,
			planner:    &fakePlanner{specErr: fmt.Errorf("pick: %w", llm.ErrInvalidSpec)},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "spec names a source that isn't registered",
			body:       `{"prompt":"what's the weather"}`,
			planner:    &fakePlanner{spec: llm.WidgetSpec{Title: "Weather", Source: "weather_api", Query: "weather", Archetype: llm.ArchetypeList}},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "llm upstream failure",
			body:       `{"prompt":"show me upcoming SpaceX launches"}`,
			planner:    &fakePlanner{specErr: &llm.UpstreamError{Reason: "OpenRouter is rate limiting requests", Err: errors.New("429")}},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "search failure",
			body:       `{"prompt":"show me upcoming SpaceX launches"}`,
			planner:    &fakePlanner{spec: launchSpec},
			source:     fakeSource{err: fmt.Errorf("events_search: %w", sources.ErrRateLimited)},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "empty prompt",
			body:       `{"prompt":"   "}`,
			planner:    &fakePlanner{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "prompt too long",
			body:       `{"prompt":"` + strings.Repeat("a", 501) + `"}`,
			planner:    &fakePlanner{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not JSON",
			body:       `prompt=hi`,
			planner:    &fakePlanner{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "trailing data after the JSON object",
			body:       `{"prompt":"hi"} {"x":`,
			planner:    &fakePlanner{},
			wantStatus: http.StatusBadRequest,
			wantError:  "Send JSON like",
		},
		{
			name:       "body over the size limit",
			body:       `{"prompt":"` + strings.Repeat("a", 20000) + `"}`,
			planner:    &fakePlanner{},
			wantStatus: http.StatusBadRequest,
			wantError:  "too long",
		},
		{
			name:       "client went away",
			body:       `{"prompt":"show me upcoming SpaceX launches"}`,
			planner:    &fakePlanner{specErr: &llm.UpstreamError{Reason: "the request was cancelled", Err: context.Canceled}},
			cancelled:  true,
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.source.name = "events_search"
			h := widgets.NewHandler(tt.planner, sources.NewRegistry(tt.source), slog.New(slog.NewTextHandler(io.Discard, nil)))

			req := httptest.NewRequest(http.MethodPost, "/api/widgets/from-prompt", strings.NewReader(tt.body))
			if tt.cancelled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			h.FromPrompt(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.planner.shapeCalled != tt.wantShape {
				t.Errorf("ShapeItems called = %v, want %v", tt.planner.shapeCalled, tt.wantShape)
			}

			if rec.Code != http.StatusOK {
				var body struct {
					Error *string `json:"error"`
					Hint  *string `json:"hint"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == nil || *body.Error == "" || body.Hint == nil {
					t.Fatalf(`error body = %s, want {"error": "...", "hint": "..."}`, rec.Body)
				}
				if !strings.Contains(*body.Error, tt.wantError) {
					t.Errorf("error = %q, want it to mention %q", *body.Error, tt.wantError)
				}
				return
			}

			if !tt.planner.hadDeadline {
				t.Error("pipeline ran without a deadline; want RequestBudget applied")
			}

			// Items must always encode as an array, never null.
			if !strings.Contains(rec.Body.String(), `"items":[`) {
				t.Errorf("items is not a JSON array: %s", rec.Body)
			}
			var got widgets.Widget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Title != launchSpec.Title || got.Source != launchSpec.Source || got.Query != launchSpec.Query || got.Archetype != launchSpec.Archetype {
				t.Errorf("widget header = %+v, want fields from %+v", got, launchSpec)
			}
			if !slices.Equal(got.Items, tt.wantItems) {
				t.Errorf("items = %+v, want %+v", got.Items, tt.wantItems)
			}
		})
	}
}
