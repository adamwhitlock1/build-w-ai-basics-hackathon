package widgets_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// countingSource is a sources.Source with canned results that counts its
// searches.
type countingSource struct {
	fakeSource
	searches atomic.Int32
}

func (c *countingSource) Search(ctx context.Context, q string) ([]sources.Result, error) {
	c.searches.Add(1)
	return c.fakeSource.Search(ctx, q)
}

// oldLaunches is Alice's saved widget, built 7 hours ago (so stale) and
// never refreshed.
func oldLaunches(prompt string) store.CustomWidget {
	return store.CustomWidget{
		ID: 1, UserID: alice.ID, Name: "SpaceX Launches", Source: "events_search", Query: "upcoming SpaceX launches",
		Prompt:     prompt,
		RenderSpec: json.RawMessage(`{"title":"SpaceX Launches","archetype":"list","items":[{"title":"Old launch"}]}`),
		CreatedAt:  time.Now().Add(-7 * time.Hour).Truncate(time.Second),
	}
}

func TestRefreshCustom(t *testing.T) {
	found := []sources.Result{{Title: "Raw result", URL: "https://example.com/raw"}}
	tests := []struct {
		name        string
		id          string
		widget      store.CustomWidget
		refreshedAt time.Duration // how long ago the widget was last refreshed; 0 for never
		planner     *fakePlanner
		results     []sources.Result
		searchErr   error
		anonymous   bool
		wantStatus  int
		wantError   string   // substring of an error body
		wantItems   []string // titles; for 200s
		wantSaved   bool     // the saved items were replaced
		wantPrompt  string   // what ShapeItems was given, when it ran
		wantSearch  bool
	}{
		{
			name: "refreshes with the saved prompt", id: "1", widget: oldLaunches("show me upcoming SpaceX launches"),
			planner: &fakePlanner{items: shapedItems}, results: found,
			wantStatus: 200, wantItems: []string{"Starlink 10-12"}, wantSaved: true, wantPrompt: "show me upcoming SpaceX launches", wantSearch: true,
		},
		{
			name: "older widget uses its title as the prompt", id: "1", widget: oldLaunches(""),
			planner: &fakePlanner{items: shapedItems}, results: found,
			wantStatus: 200, wantItems: []string{"Starlink 10-12"}, wantSaved: true, wantPrompt: "SpaceX Launches", wantSearch: true,
		},
		{
			name: "shaping fails, raw results are used", id: "1", widget: oldLaunches("p"),
			planner: &fakePlanner{shapeErr: llm.ErrNoItems}, results: found,
			wantStatus: 200, wantItems: []string{"Raw result"}, wantSaved: true, wantPrompt: "p", wantSearch: true,
		},
		{
			name: "everything found already happened", id: "1", widget: oldLaunches("p"),
			planner: &fakePlanner{items: []llm.Item{}}, results: found,
			wantStatus: 200, wantItems: []string{}, wantSaved: true, wantPrompt: "p", wantSearch: true,
		},
		{
			name: "search finds nothing, items are kept", id: "1", widget: oldLaunches("p"),
			planner: &fakePlanner{items: shapedItems},
			wantStatus: 200, wantItems: []string{"Old launch"}, wantSearch: true,
		},
		{
			name: "search fails, items are kept", id: "1", widget: oldLaunches("p"),
			planner: &fakePlanner{}, searchErr: errors.New("connection reset"),
			wantStatus: 502, wantError: "Couldn't fetch results from Brave Search.", wantSearch: true,
		},
		{
			name: "search rate limited", id: "1", widget: oldLaunches("p"),
			planner: &fakePlanner{}, searchErr: sources.ErrRateLimited,
			wantStatus: 502, wantError: "Brave Search is busy right now.", wantSearch: true,
		},
		{
			name: "refreshed under a minute ago, sent back as is", id: "1", widget: oldLaunches("p"), refreshedAt: 20 * time.Second,
			planner: &fakePlanner{items: shapedItems}, results: found,
			wantStatus: 200, wantItems: []string{"Old launch"},
		},
		{
			name: "refreshed a few minutes ago, refreshes", id: "1", widget: oldLaunches("p"), refreshedAt: 5 * time.Minute,
			planner: &fakePlanner{items: shapedItems}, results: found,
			wantStatus: 200, wantItems: []string{"Starlink 10-12"}, wantSaved: true, wantPrompt: "p", wantSearch: true,
		},
		{
			name: "someone else's widget", id: "1", widget: func() store.CustomWidget { w := oldLaunches("p"); w.UserID = 99; return w }(),
			planner: &fakePlanner{}, results: found, wantStatus: 404, wantError: "That widget doesn't exist.",
		},
		{
			name: "missing widget", id: "2", widget: oldLaunches("p"),
			planner: &fakePlanner{}, results: found, wantStatus: 404, wantError: "That widget doesn't exist.",
		},
		{
			name: "not a number", id: "abc", widget: oldLaunches("p"),
			planner: &fakePlanner{}, wantStatus: 404, wantError: "That widget doesn't exist.",
		},
		{
			name: "source no longer exists", id: "1", widget: func() store.CustomWidget { w := oldLaunches("p"); w.Source = "gone_search"; return w }(),
			planner: &fakePlanner{}, wantStatus: 422, wantError: "isn't available any more",
		},
		{
			name: "not logged in", id: "1", widget: oldLaunches("p"), anonymous: true,
			planner: &fakePlanner{}, wantStatus: 401,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			widget := tt.widget
			if tt.refreshedAt != 0 {
				widget.RefreshedAt = time.Now().Add(-tt.refreshedAt).Truncate(time.Second)
			}
			saved := &fakeWidgetStore{widgets: []store.CustomWidget{widget}}
			src := &countingSource{fakeSource: fakeSource{name: "events_search", results: tt.results, err: tt.searchErr}}
			h := widgets.NewHandler(tt.planner, sources.NewRegistry(src), saved, &fakeLayouts{}, discardLogger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/widgets/custom/{id}/refresh", h.RefreshCustom)
			req := httptest.NewRequest(http.MethodPost, "/api/widgets/custom/"+tt.id+"/refresh", nil)
			if !tt.anonymous {
				req = req.WithContext(auth.WithUser(req.Context(), alice))
			}
			before := time.Now().Truncate(time.Second)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantError != "" && !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Errorf("body = %s, want it to contain %q", rec.Body, tt.wantError)
			}
			if searched := src.searches.Load() > 0; searched != tt.wantSearch {
				t.Errorf("searched = %v, want %v", searched, tt.wantSearch)
			}
			if tt.wantPrompt != "" {
				if tt.planner.shapedFor != tt.wantPrompt {
					t.Errorf("shaped for %q, want %q", tt.planner.shapedFor, tt.wantPrompt)
				}
				if tt.planner.shapedSpec.Title != "SpaceX Launches" || tt.planner.shapedSpec.Archetype != llm.ArchetypeList || tt.planner.shapedSpec.Query != "upcoming SpaceX launches" {
					t.Errorf("shaped with spec %+v, want the saved title, archetype and query", tt.planner.shapedSpec)
				}
			}

			stored := saved.widgets[0]
			if replaced := string(stored.RenderSpec) != string(widget.RenderSpec); replaced != tt.wantSaved {
				t.Errorf("saved items replaced = %v, want %v (render spec now %s)", replaced, tt.wantSaved, stored.RenderSpec)
			}
			if tt.wantSaved && stored.RefreshedAt.Before(before) {
				t.Errorf("refreshed at %v, want now", stored.RefreshedAt)
			}
			if !tt.wantSaved && !stored.RefreshedAt.Equal(widget.RefreshedAt) {
				t.Errorf("refreshed at changed to %v without a refresh", stored.RefreshedAt)
			}

			if rec.Code != http.StatusOK {
				return
			}
			var got widgets.SavedWidget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(titles(got.Items), tt.wantItems) {
				t.Errorf("items = %v, want %v", titles(got.Items), tt.wantItems)
			}
			if got.Title != "SpaceX Launches" || got.Archetype != llm.ArchetypeList || got.ID != 1 {
				t.Errorf("widget = %+v, want the same widget, title and archetype kept", got)
			}
			// A refreshed widget is fresh; one sent back unchanged keeps its age.
			switch {
			case tt.wantSaved && (got.Stale || got.UpdatedAt.Before(before)):
				t.Errorf("refreshed: updatedAt %v, stale %v; want now and fresh", got.UpdatedAt, got.Stale)
			case !tt.wantSaved && tt.refreshedAt == 0 && (!got.Stale || !got.UpdatedAt.Equal(widget.CreatedAt)):
				t.Errorf("kept: updatedAt %v, stale %v; want the build time %v and stale", got.UpdatedAt, got.Stale, widget.CreatedAt)
			}
		})
	}
}
