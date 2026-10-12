package widgets_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

var (
	parent = store.User{ID: 7, Email: "parent@example.com", IsAdmin: true}
	kid    = store.User{ID: 8, Email: "kid@example.com"}
)

// allAre reports whether auds is non-empty and every entry is want.
func allAre(auds []sources.Audience, want sources.Audience) bool {
	if len(auds) == 0 {
		return false
	}
	for _, a := range auds {
		if a != want {
			return false
		}
	}
	return true
}

// The curated widgets are kid-safe when the member whose card it is is a
// kid, whoever is looking, and every model call and search says so.
func TestCuratedWidgetsAreKidSafeForKids(t *testing.T) {
	who := []struct {
		name    string
		viewer  store.User
		subject store.User // the zero User means "the viewer"
		want    sources.Audience
	}{
		{"kid on their own page", kid, store.User{}, sources.Kids},
		{"parent on their own page", parent, store.User{}, sources.Everyone},
		{"parent on their kid's page", parent, kid, sources.Kids},
	}
	cards := []struct {
		name    string
		curator func() *fakeCurator
		serve   func(*widgets.CuratedHandler, http.ResponseWriter, *http.Request)
	}{
		{"articles", func() *fakeCurator {
			return &fakeCurator{articleQueries: []string{"minecraft news"}}
		}, (*widgets.CuratedHandler).Articles},
		{"articles fallback", func() *fakeCurator {
			// The model fails, so the interests are searched as written.
			return &fakeCurator{articleErr: errors.New("model down")}
		}, (*widgets.CuratedHandler).Articles},
		{"events", func() *fakeCurator {
			return &fakeCurator{
				eventQueries: []string{"Boulder events"},
				events:       []llm.Event{{Name: "Build-off", Date: "Sat", SortDate: "2099-01-01"}},
			}
		}, (*widgets.CuratedHandler).Events},
	}
	profile := store.MemberProfile{Interests: "minecraft", Location: "Boulder, CO"}
	found := map[string][]sources.Result{
		"minecraft news": {article("1")},
		"minecraft":      {article("2")},
		"Boulder events": {article("3")},
	}

	for _, c := range cards {
		for _, w := range who {
			t.Run(c.name+"/"+w.name, func(t *testing.T) {
				curator := c.curator()
				news := &querySource{name: "news_search", results: found}
				events := &querySource{name: "events_search", results: found}
				h := widgets.NewCuratedHandler(curator, sources.NewRegistry(news, events),
					fakeProfiles{profile: profile}, newMemResults(), discardLogger)

				ctx := auth.WithUser(context.Background(), w.viewer)
				if w.subject.ID != 0 {
					ctx = auth.WithSubject(ctx, w.subject) // what RequireMember does
				}
				req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
				rec := httptest.NewRecorder()
				c.serve(h, rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
				}
				if !allAre(curator.auds, w.want) {
					t.Errorf("curator audiences = %v, want all %v", curator.auds, w.want)
				}
				if searched := append(news.auds, events.auds...); !allAre(searched, w.want) {
					t.Errorf("search audiences = %v, want all %v", searched, w.want)
				}
				var got widgets.CuratedWidget
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.KidSafe != (w.want == sources.Kids) {
					t.Errorf("kidSafe = %v, want %v", got.KidSafe, w.want == sources.Kids)
				}
			})
		}
	}
}

// A widget a kid describes is planned, searched and shaped for a kid, and
// remembers that it was, both when made and when loaded again.
func TestFromPromptIsKidSafeForKids(t *testing.T) {
	tests := []struct {
		name  string
		user  store.User
		shape error // ShapeItems fails, so the raw results are used
		want  sources.Audience
	}{
		{"kid", kid, nil, sources.Kids},
		{"kid, shaping fails", kid, errors.New("model down"), sources.Kids},
		{"parent", parent, nil, sources.Everyone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner := &fakePlanner{spec: launchSpec, items: shapedItems, shapeErr: tt.shape}
			src := &querySource{name: "events_search", results: map[string][]sources.Result{launchSpec.Query: results}}
			saved := &fakeWidgetStore{}
			h := widgets.NewHandler(planner, sources.NewRegistry(src), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)

			req := httptest.NewRequest(http.MethodPost, "/api/widgets/from-prompt", strings.NewReader(`{"prompt":"upcoming SpaceX launches"}`))
			req = req.WithContext(auth.WithUser(req.Context(), tt.user))
			rec := httptest.NewRecorder()
			h.FromPrompt(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
			}
			if len(planner.auds) != 2 || !allAre(planner.auds, tt.want) {
				t.Errorf("planner audiences = %v, want 2 calls with %v", planner.auds, tt.want)
			}
			if !allAre(src.auds, tt.want) {
				t.Errorf("search audiences = %v, want all %v", src.auds, tt.want)
			}

			wantKidSafe := tt.want == sources.Kids
			var got widgets.SavedWidget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.KidSafe != wantKidSafe {
				t.Errorf("response kidSafe = %v, want %v", got.KidSafe, wantKidSafe)
			}
			back, err := widgets.FromStore(saved.widgets[0], time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if back.KidSafe != wantKidSafe {
				t.Errorf("kidSafe after loading = %v, want %v (render_spec %s)", back.KidSafe, wantKidSafe, saved.widgets[0].RenderSpec)
			}
		})
	}
}

// Widgets saved before kid-safe picks existed have no kidSafe in their
// render spec, and read as not kid-safe.
func TestFromStoreWithoutKidSafe(t *testing.T) {
	cw := store.CustomWidget{ID: 3, UserID: kid.ID, Source: "news_search", Query: "q",
		RenderSpec: []byte(`{"title":"Old Widget","archetype":"list","items":[]}`)}
	got, err := widgets.FromStore(cw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.KidSafe {
		t.Error("kidSafe = true, want false for a widget saved before the field existed")
	}
}

// A remembered curated result is only sent again to someone it was picked
// for: a kid's result picked before kid-safe picks existed is picked again,
// kid-safe, and so is a parent's result that was picked kid-safe.
func TestRememberedResultsMatchTheAudience(t *testing.T) {
	tests := []struct {
		name           string
		member         store.User
		rememberedSafe bool
		wantPicked     bool
	}{
		{"kid, remembered before kid-safe picks", kid, false, true},
		{"kid, remembered kid-safe", kid, true, false},
		{"parent, remembered for a parent", parent, false, false},
		{"parent, remembered kid-safe", parent, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newFreshnessRig(profilesByUser{tt.member.ID: {Interests: "space"}})
			payload, _ := json.Marshal(widgets.Widget{Title: "Remembered", Source: "news_search", Archetype: llm.ArchetypeCards,
				Items: []llm.Item{{Title: "Old story"}}, KidSafe: tt.rememberedSafe})
			rig.results.rows[resultKey(tt.member.ID, "articles")] = store.CuratedResult{UserID: tt.member.ID, Kind: "articles",
				Inputs: "space", Payload: payload, UpdatedAt: time.Now().Add(-time.Minute * 10)}

			req := httptest.NewRequest(http.MethodGet, "/api/widgets/articles", nil)
			req = req.WithContext(auth.WithUser(req.Context(), tt.member))
			rec := httptest.NewRecorder()
			rig.h.Articles(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
			}
			var got widgets.CuratedWidget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}

			wantSafe := tt.member.ID == kid.ID
			if picked := len(rig.news.auds) > 0; picked != tt.wantPicked {
				t.Fatalf("searched = %v, want %v", picked, tt.wantPicked)
			}
			if got.KidSafe != wantSafe {
				t.Errorf("kidSafe = %v, want %v", got.KidSafe, wantSafe)
			}
			if !tt.wantPicked {
				return
			}
			if want := audienceFor(tt.member); !allAre(rig.news.auds, want) || !allAre(rig.curator.auds, want) {
				t.Errorf("picked with search audiences %v and curator audiences %v, want all %v", rig.news.auds, rig.curator.auds, want)
			}
			var row widgets.Widget
			if err := json.Unmarshal(rig.results.rows[resultKey(tt.member.ID, "articles")].Payload, &row); err != nil || row.KidSafe != wantSafe {
				t.Errorf("remembered kidSafe = %v (err %v), want %v", row.KidSafe, err, wantSafe)
			}
		})
	}
}

// Refreshing a described widget searches and shapes for its owner: a kid's
// widget, even one saved before kid-safe picks, comes back kid-safe.
func TestRefreshCustomIsKidSafeForKids(t *testing.T) {
	tests := []struct {
		name  string
		owner store.User
		want  sources.Audience
	}{
		{"kid's widget from before kid-safe picks", kid, sources.Kids},
		{"parent's widget", parent, sources.Everyone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := oldLaunches("upcoming SpaceX launches") // no kidSafe in its render spec
			old.UserID = tt.owner.ID
			saved := &fakeWidgetStore{widgets: []store.CustomWidget{old}}
			planner := &fakePlanner{items: shapedItems}
			src := &querySource{name: "events_search", results: map[string][]sources.Result{old.Query: results}}
			h := widgets.NewHandler(planner, sources.NewRegistry(src), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/widgets/custom/{id}/refresh", h.RefreshCustom)
			req := httptest.NewRequest(http.MethodPost, "/api/widgets/custom/1/refresh", nil)
			req = req.WithContext(auth.WithUser(req.Context(), tt.owner))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
			}
			if !allAre(src.auds, tt.want) || !allAre(planner.auds, tt.want) {
				t.Errorf("search audiences %v, planner audiences %v; want all %v", src.auds, planner.auds, tt.want)
			}
			wantSafe := tt.want == sources.Kids
			var got widgets.SavedWidget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			back, err := widgets.FromStore(saved.widgets[0], time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if got.KidSafe != wantSafe || back.KidSafe != wantSafe {
				t.Errorf("kidSafe in response %v, after loading %v; want %v", got.KidSafe, back.KidSafe, wantSafe)
			}
		})
	}
}

// audienceFor mirrors the handlers' rule, for the tests' expectations.
func audienceFor(u store.User) sources.Audience {
	if u.IsAdmin {
		return sources.Everyone
	}
	return sources.Kids
}
