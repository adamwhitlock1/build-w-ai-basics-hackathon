package widgets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// paidCurator and paidSource fail the test if called: a pre-filled result
// must be served without spending search or AI credit.
type paidCurator struct{ t *testing.T }

func (p paidCurator) CurateArticles(context.Context, string, sources.Audience) ([]string, error) {
	p.t.Error("CurateArticles called for a pre-filled result")
	return nil, nil
}

func (p paidCurator) CurateEvents(context.Context, string, string, time.Time, sources.Audience) ([]string, error) {
	p.t.Error("CurateEvents called for a pre-filled result")
	return nil, nil
}

func (p paidCurator) ExtractEvents(context.Context, string, string, []sources.Result, time.Time, sources.Audience) ([]llm.Event, error) {
	p.t.Error("ExtractEvents called for a pre-filled result")
	return nil, nil
}

type paidSource struct {
	t    *testing.T
	name string
}

func (p paidSource) Name() string  { return p.name }
func (p paidSource) Shape() string { return "test results" }
func (p paidSource) Search(context.Context, string, sources.Audience) ([]sources.Result, error) {
	p.t.Errorf("%s searched for a pre-filled result", p.name)
	return nil, nil
}

func TestPrefilledResultsAreServed(t *testing.T) {
	ctx := context.Background()
	profile := store.MemberProfile{Interests: "Rockets,  Soccer", Location: "Denver, CO"}
	now := time.Now().Truncate(time.Second)

	for _, member := range []store.User{
		{ID: 1, Email: "maya@rivera.test", IsAdmin: true},
		{ID: 2, Email: "lily@rivera.test"},
	} {
		results := newMemResults()
		articles := []llm.Item{{Title: "Launch recap", URL: "https://www.nasa.gov/"}}
		events := []llm.Item{{Title: "Library story time", URL: "https://www.denverlibrary.org/"}}
		if err := widgets.PrefillArticles(ctx, results, member, profile, articles, now); err != nil {
			t.Fatal(err)
		}
		if err := widgets.PrefillEvents(ctx, results, member, profile, events, now); err != nil {
			t.Fatal(err)
		}

		h := widgets.NewCuratedHandler(paidCurator{t},
			sources.NewRegistry(paidSource{t, "news_search"}, paidSource{t, "events_search"}),
			fakeProfiles{profile: profile}, results, discardLogger)
		for path, want := range map[string]string{"/api/widgets/articles": "Launch recap", "/api/widgets/events": "Library story time"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req = req.WithContext(auth.WithUser(req.Context(), member))
			rec := httptest.NewRecorder()
			if path == "/api/widgets/articles" {
				h.Articles(rec, req)
			} else {
				h.Events(rec, req)
			}
			var got widgets.CuratedWidget
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("%s as %s: status %d, %v; body: %s", path, member.Email, rec.Code, err, rec.Body)
			}
			if len(got.Items) != 1 || got.Items[0].Title != want {
				t.Errorf("%s as %s: items = %+v, want %q", path, member.Email, got.Items, want)
			}
			if got.Stale {
				t.Errorf("%s as %s: a just-seeded result is stale", path, member.Email)
			}
			if got.KidSafe == member.IsAdmin {
				t.Errorf("%s as %s: kidSafe = %v", path, member.Email, got.KidSafe)
			}
		}
	}
}

func TestPrefilledCustomWidgetIsServed(t *testing.T) {
	jordan := store.User{ID: 3, Email: "jordan@rivera.test"}
	items := []llm.Item{{Title: "Rocket League finals", URL: "https://www.rocketleague.com/"}}
	cw, err := widgets.NewCustomWidget(jordan, "Rocket League news", "news_search", "rocket league esports", "rocket league esports news", llm.ArchetypeList, items)
	if err != nil {
		t.Fatal(err)
	}
	saved := &fakeWidgetStore{}
	if _, err := saved.CreateCustomWidget(context.Background(), cw); err != nil {
		t.Fatal(err)
	}

	h := widgets.NewHandler(&fakePlanner{}, sources.NewRegistry(paidSource{t, "news_search"}), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)
	req := httptest.NewRequest(http.MethodGet, "/api/widgets/custom/1", nil)
	req.SetPathValue("id", "1")
	req = req.WithContext(auth.WithUser(req.Context(), jordan))
	rec := httptest.NewRecorder()
	h.GetCustom(rec, req)

	var got widgets.SavedWidget
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status %d, %v; body: %s", rec.Code, err, rec.Body)
	}
	if got.Title != "Rocket League news" || len(got.Items) != 1 || got.Items[0].Title != "Rocket League finals" || !got.KidSafe {
		t.Errorf("got %+v, want the seeded kid-safe widget", got)
	}
}
