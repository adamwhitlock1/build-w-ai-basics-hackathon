package widgets_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// fakeCurator returns canned answers and records which methods were called.
// The handler calls it from one goroutine, so it needs no locking.
type fakeCurator struct {
	articleQueries []string
	articleErr     error
	eventQueries   []string
	eventErr       error
	events         []llm.Event
	extractErr     error

	called        []string         // method names, in call order
	extractedFrom []sources.Result // the results passed to ExtractEvents
	planTimeLeft  time.Duration    // how long the planning call was given
	interests     string           // what the last planning call was given
	location      string
}

func (f *fakeCurator) CurateArticles(ctx context.Context, interests string) ([]string, error) {
	f.called = append(f.called, "CurateArticles")
	f.notePlanDeadline(ctx)
	f.interests = interests
	return f.articleQueries, f.articleErr
}

func (f *fakeCurator) CurateEvents(ctx context.Context, interests, location string, _ time.Time) ([]string, error) {
	f.called = append(f.called, "CurateEvents")
	f.notePlanDeadline(ctx)
	f.interests, f.location = interests, location
	return f.eventQueries, f.eventErr
}

func (f *fakeCurator) notePlanDeadline(ctx context.Context) {
	if deadline, ok := ctx.Deadline(); ok {
		f.planTimeLeft = time.Until(deadline)
	}
}

func (f *fakeCurator) ExtractEvents(_ context.Context, _, _ string, results []sources.Result, _ time.Time) ([]llm.Event, error) {
	f.called = append(f.called, "ExtractEvents")
	f.extractedFrom = results
	return f.events, f.extractErr
}

// querySource answers each query from canned maps and records what it was
// asked. The handler searches concurrently, so the record needs a mutex.
type querySource struct {
	name    string
	results map[string][]sources.Result
	errs    map[string]error

	mu    sync.Mutex
	asked []string
}

func (s *querySource) Name() string  { return s.name }
func (s *querySource) Shape() string { return "test results" }

func (s *querySource) Search(_ context.Context, query string) ([]sources.Result, error) {
	s.mu.Lock()
	s.asked = append(s.asked, query)
	s.mu.Unlock()
	return s.results[query], s.errs[query]
}

// queries returns what the source was asked, sorted: the searches run at
// the same time, so they arrive in no particular order.
func (s *querySource) queries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := slices.Clone(s.asked)
	slices.Sort(q)
	return q
}

// fakeProfiles is a widgets.ProfileStore holding one profile.
type fakeProfiles struct {
	profile store.MemberProfile
	err     error
}

func (f fakeProfiles) GetMemberProfile(context.Context, int64) (store.MemberProfile, error) {
	return f.profile, f.err
}

// article returns a search result whose URL and title are built from id.
func article(id string) sources.Result {
	return sources.Result{Title: "Story " + id, URL: "https://example.com/" + id, Snippet: "About " + id, Site: "example.com"}
}

// curatedCase is what both curated endpoints' tests share.
type curatedCase struct {
	name       string
	profile    fakeProfiles
	curator    *fakeCurator
	results    map[string][]sources.Result
	errs       map[string]error
	anonymous  bool
	wantStatus int
	wantError  string // substring of the error message, for error responses
	wantHint   string // the error's exact hint, if set
	wantCalls  []string
	wantAsked  []string // the queries searched, sorted; nil means none
}

// serveCurated runs one request against a CuratedHandler and returns the
// decoded widget (for 200s) and the raw body.
func serveCurated(t *testing.T, tt curatedCase, path string) (widgets.CuratedWidget, string) {
	t.Helper()
	news := &querySource{name: "news_search", results: tt.results, errs: tt.errs}
	events := &querySource{name: "events_search", results: tt.results, errs: tt.errs}
	h := widgets.NewCuratedHandler(tt.curator, sources.NewRegistry(news, events), tt.profile, discardLogger)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if !tt.anonymous {
		req = req.WithContext(auth.WithUser(req.Context(), alice))
	}
	rec := httptest.NewRecorder()
	if path == "/api/widgets/articles" {
		h.Articles(rec, req)
	} else {
		h.Events(rec, req)
	}

	body := rec.Body.String()
	if rec.Code != tt.wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, body)
	}
	if !slices.Equal(tt.curator.called, tt.wantCalls) {
		t.Errorf("curator calls = %v, want %v", tt.curator.called, tt.wantCalls)
	}
	asked := append(news.queries(), events.queries()...)
	if !slices.Equal(asked, tt.wantAsked) { // nil and empty count as equal
		t.Errorf("searched %q, want %q", asked, tt.wantAsked)
	}

	if rec.Code != http.StatusOK {
		if !strings.Contains(body, tt.wantError) || !strings.Contains(body, `"hint"`) {
			t.Errorf(`error body = %s, want {"error": "...%s...", "hint": "..."}`, body, tt.wantError)
		}
		if tt.wantHint != "" && !strings.Contains(body, `"hint":"`+tt.wantHint+`"`) {
			t.Errorf("error body = %s, want hint %q", body, tt.wantHint)
		}
		return widgets.CuratedWidget{}, body
	}
	if !strings.Contains(body, `"items":[`) || !strings.Contains(body, `"emptyReason":`) {
		t.Errorf("body = %s, want an items array and an emptyReason field", body)
	}
	var got widgets.CuratedWidget
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got, body
}

func titles(items []llm.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func TestArticles(t *testing.T) {
	interested := fakeProfiles{profile: store.MemberProfile{Interests: "space exploration, soccer and cooking"}}
	four := []string{"q1", "q2", "q3", "q4"}

	tests := []struct {
		curatedCase
		wantTitles []string
		wantQuery  string
		wantEmpty  widgets.EmptyReason
	}{
		{
			curatedCase: curatedCase{
				name:       "blank interests: explains why, without calling the LLM or searching",
				profile:    fakeProfiles{profile: store.MemberProfile{Interests: "  ", Location: "Denver"}},
				curator:    &fakeCurator{articleQueries: four},
				wantStatus: http.StatusOK,
			},
			wantTitles: []string{},
			wantEmpty:  widgets.EmptyNoInterests,
		},
		{
			curatedCase: curatedCase{
				name:       "no profile yet reads as blank interests",
				profile:    fakeProfiles{err: fmt.Errorf("get profile: %w", store.ErrNotFound)},
				curator:    &fakeCurator{},
				wantStatus: http.StatusOK,
			},
			wantTitles: []string{},
			wantEmpty:  widgets.EmptyNoInterests,
		},
		{
			curatedCase: curatedCase{
				name:    "best result per query, never the same URL twice",
				profile: interested,
				curator: &fakeCurator{articleQueries: four},
				results: map[string][]sources.Result{
					"q1": {article("a"), article("b")},
					"q2": {article("a"), article("c")}, // a is taken, so q2's best is c
					"q3": {article("d")},
					"q4": {article("e"), article("f")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  four,
			},
			wantTitles: []string{"Story a", "Story c", "Story d", "Story e"},
			wantQuery:  "q1 · q2 · q3 · q4",
		},
		{
			curatedCase: curatedCase{
				name:    "tops up to 3 from the remaining results in turn",
				profile: interested,
				curator: &fakeCurator{articleQueries: []string{"q1", "q2"}},
				results: map[string][]sources.Result{
					"q1": {article("a"), article("b"), article("c")},
					"q2": {article("d"), article("e")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  []string{"q1", "q2"},
			},
			wantTitles: []string{"Story a", "Story d", "Story b"},
			wantQuery:  "q1 · q2",
		},
		{
			curatedCase: curatedCase{
				name:    "caps at 5",
				profile: interested,
				curator: &fakeCurator{articleQueries: []string{"q1", "q2", "q3", "q4", "q5", "q6"}},
				results: map[string][]sources.Result{
					"q1": {article("a")}, "q2": {article("b")}, "q3": {article("c")},
					"q4": {article("d")}, "q5": {article("e")}, "q6": {article("f")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  []string{"q1", "q2", "q3", "q4", "q5", "q6"},
			},
			wantTitles: []string{"Story a", "Story b", "Story c", "Story d", "Story e"},
			wantQuery:  "q1 · q2 · q3 · q4 · q5 · q6",
		},
		{
			curatedCase: curatedCase{
				name:    "a failed query is skipped",
				profile: interested,
				curator: &fakeCurator{articleQueries: four},
				results: map[string][]sources.Result{
					"q2": {article("b")}, "q3": {article("c")}, "q4": {article("d")},
				},
				errs:       map[string]error{"q1": errors.New("brave returned HTTP 500")},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  four,
			},
			wantTitles: []string{"Story b", "Story c", "Story d"},
			wantQuery:  "q1 · q2 · q3 · q4",
		},
		{
			curatedCase: curatedCase{
				name:    "every query fails",
				profile: interested,
				curator: &fakeCurator{articleQueries: []string{"q1", "q2"}},
				errs: map[string]error{
					"q1": errors.New("brave returned HTTP 500"),
					"q2": fmt.Errorf("news_search: %w", sources.ErrRateLimited),
				},
				wantStatus: http.StatusBadGateway,
				wantError:  "Brave Search is busy right now.",
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  []string{"q1", "q2"},
			},
		},
		{
			curatedCase: curatedCase{
				name:    "LLM fails: the interests themselves become the queries",
				profile: interested,
				curator: &fakeCurator{articleErr: &llm.UpstreamError{Reason: "OpenRouter is rate limiting requests", Err: errors.New("429")}},
				results: map[string][]sources.Result{
					"space exploration": {article("a")}, "soccer": {article("b")}, "cooking": {article("c")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  []string{"cooking", "soccer", "space exploration"},
			},
			wantTitles: []string{"Story a", "Story b", "Story c"},
			wantQuery:  "space exploration · soccer · cooking",
		},
		{
			curatedCase: curatedCase{
				name:       "nothing found",
				profile:    interested,
				curator:    &fakeCurator{articleQueries: []string{"q1"}},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateArticles"},
				wantAsked:  []string{"q1"},
			},
			wantTitles: []string{},
			wantQuery:  "q1",
		},
		{
			curatedCase: curatedCase{
				name:       "not logged in",
				profile:    interested,
				curator:    &fakeCurator{},
				anonymous:  true,
				wantStatus: http.StatusUnauthorized,
				wantError:  "Please log in.",
			},
		},
		{
			curatedCase: curatedCase{
				name:       "database down",
				profile:    fakeProfiles{err: errors.New("disk on fire")},
				curator:    &fakeCurator{},
				wantStatus: http.StatusInternalServerError,
				wantError:  "Something went wrong",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := serveCurated(t, tt.curatedCase, "/api/widgets/articles")
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got.Title != "Articles for you" || got.Archetype != llm.ArchetypeCards || got.Source != "news_search" {
				t.Errorf("widget header = %q, %q, %q", got.Title, got.Archetype, got.Source)
			}
			if got.Query != tt.wantQuery || got.EmptyReason != tt.wantEmpty {
				t.Errorf("query, emptyReason = %q, %q; want %q, %q", got.Query, got.EmptyReason, tt.wantQuery, tt.wantEmpty)
			}
			if !slices.Equal(titles(got.Items), tt.wantTitles) {
				t.Errorf("items = %q, want %q", titles(got.Items), tt.wantTitles)
			}
		})
	}
}

func TestArticleItemFields(t *testing.T) {
	long := strings.Repeat("Astronauts trained for the mission ", 10) // 350 characters
	result := sources.Result{
		Title: "Artemis II crew named", URL: "https://space.example/artemis", Snippet: long,
		Date: "2 hours ago", Site: "space.example", Thumbnail: "https://img.example/a.jpg",
	}
	got, _ := serveCurated(t, curatedCase{
		profile:    fakeProfiles{profile: store.MemberProfile{Interests: "space"}},
		curator:    &fakeCurator{articleQueries: []string{"artemis"}},
		results:    map[string][]sources.Result{"artemis": {result}},
		wantStatus: http.StatusOK,
		wantCalls:  []string{"CurateArticles"},
		wantAsked:  []string{"artemis"},
	}, "/api/widgets/articles")

	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	it := got.Items[0]
	// Cut to under 160 characters, at a word boundary.
	wantSnippet := strings.Repeat("Astronauts trained for the mission ", 4) + "Astronauts trained…"
	want := llm.Item{
		Title: "Artemis II crew named", URL: "https://space.example/artemis", Snippet: wantSnippet,
		Date: "2 hours ago", Label: "space.example", Thumbnail: "https://img.example/a.jpg",
	}
	if it != want {
		t.Errorf("item =\n%+v\nwant\n%+v", it, want)
	}
}

func TestEvents(t *testing.T) {
	located := fakeProfiles{profile: store.MemberProfile{Interests: "indie music", Location: "Denver, CO"}}
	tomorrow := time.Now().AddDate(0, 0, 1).Format(time.DateOnly)
	concert := llm.Event{
		Name: "Indie Night", Date: "Sat, Oct 12, 7 PM", SortDate: tomorrow, Venue: "Red Rocks",
		Description: "Three local bands.", URL: "https://example.com/a", Thumbnail: "https://img.example/a.jpg",
	}
	rateLimited := fmt.Errorf("events_search: %w", sources.ErrRateLimited)

	tests := []struct {
		curatedCase
		wantItems     []llm.Item
		wantQuery     string
		wantEmpty     widgets.EmptyReason
		wantExtracted []string // URLs passed to ExtractEvents, in order
	}{
		{
			curatedCase: curatedCase{
				name:       "blank location: explains why, without calling the LLM or searching",
				profile:    fakeProfiles{profile: store.MemberProfile{Interests: "indie music", Location: " "}},
				curator:    &fakeCurator{eventQueries: []string{"e1"}},
				wantStatus: http.StatusOK,
			},
			wantItems: []llm.Item{},
			wantEmpty: widgets.EmptyNoLocation,
		},
		{
			curatedCase: curatedCase{
				name:    "events become list items; results are merged in turn without repeats",
				profile: fakeProfiles{profile: store.MemberProfile{Location: "Denver, CO"}}, // no interests is fine
				curator: &fakeCurator{eventQueries: []string{"e1", "e2"}, events: []llm.Event{concert}},
				results: map[string][]sources.Result{
					"e1": {article("a"), article("b")},
					"e2": {article("b"), article("c")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateEvents", "ExtractEvents"},
				wantAsked:  []string{"e1", "e2"},
			},
			wantItems: []llm.Item{{
				Title: "Indie Night", URL: "https://example.com/a", Snippet: "Three local bands.",
				Date: "Sat, Oct 12, 7 PM", Label: "Red Rocks", Thumbnail: "https://img.example/a.jpg",
			}},
			wantQuery:     "e1 · e2",
			wantExtracted: []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"},
		},
		{
			curatedCase: curatedCase{
				name:    "LLM can't plan: general queries for the location",
				profile: located,
				curator: &fakeCurator{eventErr: llm.ErrNoQueries, events: []llm.Event{concert}},
				results: map[string][]sources.Result{
					llm.FallbackEventQueries("Denver, CO", time.Now())[0]: {article("a")},
				},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateEvents", "ExtractEvents"},
				wantAsked:  sorted(llm.FallbackEventQueries("Denver, CO", time.Now())),
			},
			wantItems: []llm.Item{{
				Title: "Indie Night", URL: "https://example.com/a", Snippet: "Three local bands.",
				Date: "Sat, Oct 12, 7 PM", Label: "Red Rocks", Thumbnail: "https://img.example/a.jpg",
			}},
			wantQuery:     strings.Join(llm.FallbackEventQueries("Denver, CO", time.Now()), " · "),
			wantExtracted: []string{"https://example.com/a"},
		},
		{
			curatedCase: curatedCase{
				name:       "nothing upcoming",
				profile:    located,
				curator:    &fakeCurator{eventQueries: []string{"e1"}, events: []llm.Event{}},
				results:    map[string][]sources.Result{"e1": {article("a")}},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateEvents", "ExtractEvents"},
				wantAsked:  []string{"e1"},
			},
			wantItems:     []llm.Item{},
			wantQuery:     "e1",
			wantExtracted: []string{"https://example.com/a"},
		},
		{
			curatedCase: curatedCase{
				name:       "no search results: nothing to extract from",
				profile:    located,
				curator:    &fakeCurator{eventQueries: []string{"e1"}},
				wantStatus: http.StatusOK,
				wantCalls:  []string{"CurateEvents"},
				wantAsked:  []string{"e1"},
			},
			wantItems: []llm.Item{},
			wantQuery: "e1",
		},
		{
			curatedCase: curatedCase{
				name:    "extraction fails",
				profile: located,
				curator: &fakeCurator{
					eventQueries: []string{"e1"},
					extractErr:   &llm.UpstreamError{Reason: "the AI model took too long to answer", Err: context.DeadlineExceeded},
				},
				results:    map[string][]sources.Result{"e1": {article("a")}},
				wantStatus: http.StatusBadGateway,
				wantError:  "the AI model took too long to answer",
				wantCalls:  []string{"CurateEvents", "ExtractEvents"},
				wantAsked:  []string{"e1"},
			},
		},
		{
			curatedCase: curatedCase{
				name:    "extraction declined: a hint that suits a card nobody described",
				profile: located,
				curator: &fakeCurator{
					eventQueries: []string{"e1"},
					extractErr: &llm.UpstreamError{
						Reason: "the AI model declined to answer", Hint: "Try describing the widget differently.",
						Err: errors.New("finish_reason=content_filter"),
					},
				},
				results:    map[string][]sources.Result{"e1": {article("a")}},
				wantStatus: http.StatusBadGateway,
				wantError:  "the AI model declined to answer",
				wantHint:   "Try refreshing in a moment.",
				wantCalls:  []string{"CurateEvents", "ExtractEvents"},
				wantAsked:  []string{"e1"},
			},
		},
		{
			curatedCase: curatedCase{
				// Finding the events would fail the same way, so the
				// fallback queries aren't searched: that would waste credit.
				name:    "OpenRouter refuses the planning call: no searches",
				profile: located,
				curator: &fakeCurator{eventErr: &llm.UpstreamError{
					Reason: "OpenRouter rejected the API key", Hint: "Check OPENROUTER_API_KEY in backend/.env, then restart the backend.",
					Status: http.StatusUnauthorized, Err: errors.New("401"),
				}},
				wantStatus: http.StatusBadGateway,
				wantError:  "OpenRouter rejected the API key",
				wantHint:   "Check OPENROUTER_API_KEY in backend/.env, then restart the backend.",
				wantCalls:  []string{"CurateEvents"},
			},
		},
		{
			curatedCase: curatedCase{
				name:       "every search fails",
				profile:    located,
				curator:    &fakeCurator{eventQueries: []string{"e1", "e2"}},
				errs:       map[string]error{"e1": rateLimited, "e2": rateLimited},
				wantStatus: http.StatusBadGateway,
				wantError:  "Brave Search is busy right now.",
				wantCalls:  []string{"CurateEvents"},
				wantAsked:  []string{"e1", "e2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := serveCurated(t, tt.curatedCase, "/api/widgets/events")
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got.Title != "Happening near you" || got.Archetype != llm.ArchetypeList || got.Source != "events_search" {
				t.Errorf("widget header = %q, %q, %q", got.Title, got.Archetype, got.Source)
			}
			if got.Query != tt.wantQuery || got.EmptyReason != tt.wantEmpty {
				t.Errorf("query, emptyReason = %q, %q; want %q, %q", got.Query, got.EmptyReason, tt.wantQuery, tt.wantEmpty)
			}
			if !slices.Equal(got.Items, tt.wantItems) {
				t.Errorf("items =\n%+v\nwant\n%+v", got.Items, tt.wantItems)
			}
			var extracted []string
			for _, r := range tt.curator.extractedFrom {
				extracted = append(extracted, r.URL)
			}
			if !slices.Equal(extracted, tt.wantExtracted) {
				t.Errorf("ExtractEvents got results %q, want %q", extracted, tt.wantExtracted)
			}
		})
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

func TestCuratedHandlerNeedsBothSources(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewCuratedHandler without a news_search source didn't panic")
		}
	}()
	widgets.NewCuratedHandler(&fakeCurator{}, sources.NewRegistry(&querySource{name: "events_search"}), fakeProfiles{}, discardLogger)
}

// Planning has a free fallback, so it gets less time than the whole request.
func TestPlanningHasItsOwnDeadline(t *testing.T) {
	profile := fakeProfiles{profile: store.MemberProfile{Interests: "chess", Location: "Denver"}}
	for _, path := range []string{"/api/widgets/articles", "/api/widgets/events"} {
		t.Run(path, func(t *testing.T) {
			curator := &fakeCurator{articleQueries: []string{"q"}, eventQueries: []string{"q"}}
			serveCurated(t, curatedCase{
				profile:    profile,
				curator:    curator,
				wantStatus: http.StatusOK,
				wantCalls:  planCallFor(path),
				wantAsked:  []string{"q"},
			}, path)
			if curator.planTimeLeft <= 0 || curator.planTimeLeft >= widgets.RequestBudget/2 {
				t.Errorf("planning call had %v left, want a deadline well inside the %v request budget", curator.planTimeLeft, widgets.RequestBudget)
			}
		})
	}
}

// planCallFor is all the curator is asked by a request to path whose
// searches find nothing.
func planCallFor(path string) []string {
	if path == "/api/widgets/articles" {
		return []string{"CurateArticles"}
	}
	return []string{"CurateEvents"}
}

// barrierSource holds every search until n of them have started, then
// answers them all. It only answers in time if the handler runs the
// searches at the same time: run one after another, the first would wait
// for the rest forever (well, for 2 seconds, then fail).
type barrierSource struct {
	name string
	n    int

	mu      sync.Mutex
	started int
	all     chan struct{} // closed when the nth search starts
}

func newBarrierSource(name string, n int) *barrierSource {
	return &barrierSource{name: name, n: n, all: make(chan struct{})}
}

func (s *barrierSource) Name() string  { return s.name }
func (s *barrierSource) Shape() string { return "test results" }

func (s *barrierSource) Search(ctx context.Context, query string) ([]sources.Result, error) {
	s.mu.Lock()
	s.started++
	if s.started == s.n {
		close(s.all)
	}
	s.mu.Unlock()

	select {
	case <-s.all:
		return []sources.Result{article(query)}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(2 * time.Second):
		return nil, errors.New("the other searches never started: they aren't running at the same time")
	}
}

func TestCuratedSearchesRunConcurrently(t *testing.T) {
	queries := []string{"q1", "q2", "q3", "q4", "q5"}
	news := newBarrierSource("news_search", len(queries))
	h := widgets.NewCuratedHandler(&fakeCurator{articleQueries: queries},
		sources.NewRegistry(news, &querySource{name: "events_search"}),
		fakeProfiles{profile: store.MemberProfile{Interests: "chess"}}, discardLogger)

	req := httptest.NewRequest(http.MethodGet, "/api/widgets/articles", nil)
	req = req.WithContext(auth.WithUser(req.Context(), alice))
	rec := httptest.NewRecorder()
	h.Articles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got widgets.CuratedWidget
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != len(queries) {
		t.Errorf("got %d items, want %d (one per query): the searches didn't all run at once", len(got.Items), len(queries))
	}
}

func TestCuratedStopsWhenTheClientLeaves(t *testing.T) {
	// One more than the number of queries: the searches wait until the
	// request is cancelled.
	news := newBarrierSource("news_search", 3)
	h := widgets.NewCuratedHandler(&fakeCurator{articleQueries: []string{"q1", "q2"}},
		sources.NewRegistry(news, &querySource{name: "events_search"}),
		fakeProfiles{profile: store.MemberProfile{Interests: "chess"}}, discardLogger)

	ctx, cancel := context.WithCancel(auth.WithUser(context.Background(), alice))
	defer cancel()
	time.AfterFunc(20*time.Millisecond, cancel) // the browser tab closes
	req := httptest.NewRequest(http.MethodGet, "/api/widgets/articles", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Articles(rec, req)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("handler took %v to notice the cancelled request", elapsed)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503; body: %s", rec.Code, rec.Body)
	}
}

// profilesByUser is a widgets.ProfileStore holding a profile per user id.
type profilesByUser map[int64]store.MemberProfile

func (p profilesByUser) GetMemberProfile(_ context.Context, userID int64) (store.MemberProfile, error) {
	profile, ok := p[userID]
	if !ok {
		return store.MemberProfile{}, store.ErrNotFound
	}
	return profile, nil
}

// When a parent looks at a family member's page, the curated widgets are
// planned from the member's interests and location, not the parent's, and
// the log records both people.
func TestCuratedWidgetsFollowTheSubject(t *testing.T) {
	kid := store.User{ID: 8, Email: "kid@example.com"}
	profiles := profilesByUser{
		alice.ID: {Interests: "space, soccer", Location: "Denver, CO"},
		kid.ID:   {Interests: "minecraft", Location: "Boulder, CO"},
	}

	for _, path := range []string{"/api/members/8/widgets/articles", "/api/members/8/widgets/events"} {
		t.Run(path, func(t *testing.T) {
			curator := &fakeCurator{
				articleQueries: []string{"minecraft news"},
				eventQueries:   []string{"Boulder events"},
				events:         []llm.Event{{Name: "Build-off", Date: "Sat", URL: "https://example.com/1"}},
			}
			found := map[string][]sources.Result{"minecraft news": {article("1")}, "Boulder events": {article("1")}}
			news := &querySource{name: "news_search", results: found}
			events := &querySource{name: "events_search", results: found}
			var logs bytes.Buffer
			h := widgets.NewCuratedHandler(curator, sources.NewRegistry(news, events), profiles,
				slog.New(slog.NewTextHandler(&logs, nil)))

			// Alice is logged in; RequireMember made her kid the subject.
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req = req.WithContext(auth.WithSubject(auth.WithUser(req.Context(), alice), kid))
			rec := httptest.NewRecorder()
			if strings.HasSuffix(path, "/articles") {
				h.Articles(rec, req)
			} else {
				h.Events(rec, req)
			}

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
			}
			if curator.interests != "minecraft" {
				t.Errorf("planned from interests %q, want the kid's %q", curator.interests, "minecraft")
			}
			if strings.HasSuffix(path, "/events") && curator.location != "Boulder, CO" {
				t.Errorf("planned for location %q, want the kid's %q", curator.location, "Boulder, CO")
			}
			if line := logs.String(); !strings.Contains(line, "user_id=8 viewer_id=7") {
				t.Errorf("log = %q, want the kid as user_id and alice as viewer_id", line)
			}
		})
	}
}
