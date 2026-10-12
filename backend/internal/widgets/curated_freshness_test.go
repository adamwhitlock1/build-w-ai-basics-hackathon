package widgets_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// freshnessRig is a CuratedHandler wired to fakes, for the remembered-result
// tests. Alice's interests are "Space,  Soccer" (normalized to "space,
// soccer") and her location is "Denver, CO".
type freshnessRig struct {
	curator *fakeCurator
	news    *querySource
	events  *querySource
	results *memResults
	h       *widgets.CuratedHandler
}

func newFreshnessRig(profiles widgets.ProfileStore) *freshnessRig {
	found := map[string][]sources.Result{
		"space news":    {article("new-1")},
		"denver events": {article("ev-1")},
	}
	rig := &freshnessRig{
		curator: &fakeCurator{
			articleQueries: []string{"space news"},
			eventQueries:   []string{"denver events"},
			events:         []llm.Event{{Name: "Star party", Date: "Sat", URL: "https://example.com/ev-1"}},
		},
		news:    &querySource{name: "news_search", results: found},
		events:  &querySource{name: "events_search", results: found},
		results: newMemResults(),
	}
	if profiles == nil {
		profiles = fakeProfiles{profile: store.MemberProfile{Interests: "Space,  Soccer", Location: "Denver, CO"}}
	}
	rig.h = widgets.NewCuratedHandler(rig.curator, sources.NewRegistry(rig.news, rig.events), profiles, rig.results, discardLogger)
	return rig
}

// remember stores a result for userID as if it had been picked age ago.
func (rig *freshnessRig) remember(userID int64, kind, inputs string, age time.Duration, titles ...string) time.Time {
	items := make([]llm.Item, len(titles))
	for i, title := range titles {
		items[i] = llm.Item{Title: title, URL: "https://example.com/" + title}
	}
	payload, _ := json.Marshal(widgets.Widget{Title: "Remembered", Source: "news_search", Archetype: llm.ArchetypeCards, Items: items})
	at := time.Now().Add(-age).Truncate(time.Second)
	rig.results.rows[resultKey(userID, kind)] = store.CuratedResult{UserID: userID, Kind: kind, Inputs: inputs, Payload: payload, UpdatedAt: at}
	return at
}

// get sends a GET for path as Alice (with subject, if set) and decodes a
// 200's widget.
func (rig *freshnessRig) get(t *testing.T, path string, subject *store.User) (*httptest.ResponseRecorder, widgets.CuratedWidget) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := auth.WithUser(req.Context(), alice)
	if subject != nil {
		ctx = auth.WithSubject(ctx, *subject)
	}
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	if strings.Contains(path, "/articles") {
		rig.h.Articles(rec, req)
	} else {
		rig.h.Events(rec, req)
	}
	var got widgets.CuratedWidget
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
	}
	return rec, got
}

// searches is every query the rig's sources were asked.
func (rig *freshnessRig) searches() []string {
	return append(rig.news.queries(), rig.events.queries()...)
}

const aliceArticleInputs = "space, soccer"
const aliceEventInputs = "space, soccer\ndenver, co"

func TestRememberedArticles(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		inputs       string // of the remembered result; "" for none
		age          time.Duration
		wantSearch   bool
		wantStale    bool
		wantRemember bool // a new result is saved
	}{
		{name: "first visit picks and remembers", path: "/api/widgets/articles", wantSearch: true, wantRemember: true},
		{name: "fresh result is sent as is", path: "/api/widgets/articles", inputs: aliceArticleInputs, age: time.Hour},
		{name: "stale result is sent, marked stale", path: "/api/widgets/articles", inputs: aliceArticleInputs, age: 4 * time.Hour, wantStale: true},
		{name: "changed interests pick again", path: "/api/widgets/articles", inputs: "chess", age: time.Minute * 5, wantSearch: true, wantRemember: true},
		{name: "refresh picks again", path: "/api/widgets/articles?refresh=1", inputs: aliceArticleInputs, age: 10 * time.Minute, wantSearch: true, wantRemember: true},
		{name: "refresh within a minute sends the result", path: "/api/widgets/articles?refresh=1", inputs: aliceArticleInputs, age: 20 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newFreshnessRig(nil)
			var rememberedAt time.Time
			if tt.inputs != "" {
				rememberedAt = rig.remember(alice.ID, "articles", tt.inputs, tt.age, "old-1", "old-2", "old-3")
			}
			before := time.Now().Truncate(time.Second)

			rec, got := rig.get(t, tt.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
			}

			searched := len(rig.searches()) > 0
			if searched != tt.wantSearch || (len(rig.curator.called) > 0) != tt.wantSearch {
				t.Errorf("searches %v, curator calls %v; want searching = %v", rig.searches(), rig.curator.called, tt.wantSearch)
			}
			if got.Stale != tt.wantStale {
				t.Errorf("stale = %v, want %v", got.Stale, tt.wantStale)
			}
			if (rig.results.puts == 1) != tt.wantRemember {
				t.Errorf("remembered %d times, want a new result = %v", rig.results.puts, tt.wantRemember)
			}

			if tt.wantSearch {
				if !slices.Equal(titles(got.Items), []string{"Story new-1"}) {
					t.Errorf("items = %v, want the new pick", titles(got.Items))
				}
				if got.UpdatedAt.Before(before) {
					t.Errorf("updatedAt = %v, want now (at or after %v)", got.UpdatedAt, before)
				}
				row := rig.results.rows[resultKey(alice.ID, "articles")]
				if row.Inputs != aliceArticleInputs || !row.UpdatedAt.Equal(got.UpdatedAt) {
					t.Errorf("remembered row = inputs %q at %v; want %q at %v", row.Inputs, row.UpdatedAt, aliceArticleInputs, got.UpdatedAt)
				}
			} else {
				if !slices.Equal(titles(got.Items), []string{"old-1", "old-2", "old-3"}) {
					t.Errorf("items = %v, want the remembered ones", titles(got.Items))
				}
				if !got.UpdatedAt.Equal(rememberedAt) {
					t.Errorf("updatedAt = %v, want the remembered %v", got.UpdatedAt, rememberedAt)
				}
			}
		})
	}
}

func TestRememberedEventsFollowTheLocation(t *testing.T) {
	rig := newFreshnessRig(nil)
	rig.remember(alice.ID, "events", "space, soccer\nboulder, co", time.Hour, "Old event")

	rec, got := rig.get(t, "/api/widgets/events", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	if !slices.Equal(rig.curator.called, []string{"CurateEvents", "ExtractEvents"}) {
		t.Errorf("curator calls = %v, want events picked again for the new location", rig.curator.called)
	}
	if !slices.Equal(titles(got.Items), []string{"Star party"}) {
		t.Errorf("items = %v, want the new pick", titles(got.Items))
	}
	if row := rig.results.rows[resultKey(alice.ID, "events")]; row.Inputs != aliceEventInputs {
		t.Errorf("remembered inputs = %q, want %q", row.Inputs, aliceEventInputs)
	}

	// The next visit is served from what was just remembered.
	rig2 := &freshnessRig{curator: &fakeCurator{}, news: rig.news, events: &querySource{name: "events_search"}, results: rig.results}
	rig2.h = widgets.NewCuratedHandler(rig2.curator, sources.NewRegistry(rig.news, rig2.events),
		fakeProfiles{profile: store.MemberProfile{Interests: "Space,  Soccer", Location: "Denver, CO"}}, rig.results, discardLogger)
	if _, again := rig2.get(t, "/api/widgets/events", nil); len(rig2.curator.called) != 0 || !slices.Equal(titles(again.Items), []string{"Star party"}) {
		t.Errorf("second visit: curator calls %v, items %v; want none and the remembered events", rig2.curator.called, titles(again.Items))
	}
}

func TestFailedRefreshKeepsTheRememberedResult(t *testing.T) {
	rig := newFreshnessRig(nil)
	rig.news.errs = map[string]error{"space news": errors.New("brave is down")}
	at := rig.remember(alice.ID, "articles", aliceArticleInputs, 4*time.Hour, "old-1")

	rec, _ := rig.get(t, "/api/widgets/articles?refresh=1", nil)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body)
	}
	row := rig.results.rows[resultKey(alice.ID, "articles")]
	if rig.results.puts != 0 || !row.UpdatedAt.Equal(at) {
		t.Errorf("after a failed refresh: %d puts, row updated at %v; want 0 and %v", rig.results.puts, row.UpdatedAt, at)
	}
}

func TestNothingToSearchIsNotRemembered(t *testing.T) {
	rig := newFreshnessRig(fakeProfiles{})
	for _, path := range []string{"/api/widgets/articles", "/api/widgets/events"} {
		if rec, got := rig.get(t, path, nil); rec.Code != http.StatusOK || got.EmptyReason == "" {
			t.Errorf("%s: status %d, emptyReason %q; want 200 and a reason", path, rec.Code, got.EmptyReason)
		}
	}
	if rig.results.puts != 0 {
		t.Errorf("remembered %d results, want none", rig.results.puts)
	}
}

func TestAMemberViewUsesTheMembersResult(t *testing.T) {
	kid := store.User{ID: 8, Email: "kid@example.com"}
	rig := newFreshnessRig(profilesByUser{
		alice.ID: {Interests: "Space,  Soccer", Location: "Denver, CO"},
		kid.ID:   {Interests: "minecraft", Location: "Boulder, CO"},
	})
	rig.curator.articleQueries = []string{"space news"}

	// The kid's fresh articles are sent to the parent without searching.
	rig.remember(kid.ID, "articles", "minecraft", time.Hour, "Kid story")
	if _, got := rig.get(t, "/api/members/8/widgets/articles", &kid); len(rig.searches()) != 0 || !slices.Equal(titles(got.Items), []string{"Kid story"}) {
		t.Errorf("searches %v, items %v; want none and the kid's remembered articles", rig.searches(), titles(got.Items))
	}

	// Picking from the parent's view remembers the result as the kid's.
	if rec, _ := rig.get(t, "/api/members/8/widgets/articles?refresh=1", &kid); rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d; body: %s", rec.Code, rec.Body)
	}
	if row := rig.results.rows[resultKey(kid.ID, "articles")]; row.Inputs != "minecraft" || rig.results.puts != 1 {
		t.Errorf("kid's row inputs %q after %d puts; want %q after 1", row.Inputs, rig.results.puts, "minecraft")
	}
	if _, ok := rig.results.rows[resultKey(alice.ID, "articles")]; ok {
		t.Error("the parent got a remembered row from viewing the kid's page")
	}
}
