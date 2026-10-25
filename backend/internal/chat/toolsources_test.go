package chat_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"porchlight/internal/chat"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// recordingSource answers every search with results, remembering what it
// was asked.
type recordingSource struct {
	name    string
	results []sources.Result
	queries []string
	auds    []sources.Audience
}

func (s *recordingSource) Name() string  { return s.name }
func (s *recordingSource) Shape() string { return "test" }
func (s *recordingSource) Search(_ context.Context, q string, aud sources.Audience) ([]sources.Result, error) {
	s.queries = append(s.queries, q)
	s.auds = append(s.auds, aud)
	return s.results, nil
}

type noFetcher struct{}

func (noFetcher) Fetch(context.Context, string) ([]byte, error) {
	panic("no calendar should be fetched")
}

func TestToolSources(t *testing.T) {
	ctx := context.Background()
	f := newFamily(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	forecast := &recordingSource{name: sources.WeatherForecast, results: []sources.Result{{Title: "Today (Sat, Oct 24)", Snippet: "Sunny. High 75°F."}}}
	web := &recordingSource{name: "web_search", results: []sources.Result{{Title: "Artemis", Site: "nasa.gov", URL: "https://nasa.gov", Snippet: "Moon."}}}
	ts := &chat.ToolSources{Profiles: f.st, Agenda: widgets.NewAgenda(noFetcher{}, f.st, logger), Results: f.st, Forecast: forecast, Web: web}

	// No location yet: no forecast, and no search for one.
	out, err := ts.Weather(ctx, f.kid)
	if err != nil || !strings.Contains(out, "haven't saved a location") || len(forecast.queries) != 0 {
		t.Errorf("weather with no location = %q, %v (%d searches)", out, err, len(forecast.queries))
	}
	profile := store.MemberProfile{UserID: f.kid.ID, Interests: "rockets", Location: "Denver, CO"}
	if err := f.st.UpsertMemberProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	out, err = ts.Weather(ctx, f.kid)
	if err != nil || !strings.Contains(out, "Denver, CO") || !strings.Contains(out, "Sunny. High 75°F.") || forecast.auds[0] != sources.Kids {
		t.Errorf("weather = %q, %v; want Denver's forecast, asked for kids", out, err)
	}

	// My day with no calendar says so (and fetches nothing).
	out, err = ts.MyDay(ctx, f.kid, time.UTC)
	if err != nil || !strings.Contains(out, "haven't connected a calendar") {
		t.Errorf("my day = %q, %v", out, err)
	}

	// Picks: interests and the remembered articles.
	if err := widgets.PrefillArticles(ctx, f.st, f.kid, profile, []llm.Item{{Title: "Rocket launch Tuesday", Label: "space.com"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	out, err = ts.MyPicks(ctx, f.kid)
	if err != nil || !strings.Contains(out, "Interests: rockets") || !strings.Contains(out, "Rocket launch Tuesday") {
		t.Errorf("picks = %q, %v", out, err)
	}

	// Search is kid-safe and carries no links.
	results, err := ts.Search(ctx, f.kid, "artemis")
	if err != nil || len(results) != 1 || results[0].Site != "nasa.gov" || web.auds[0] != sources.Kids {
		t.Errorf("search = %+v, %v; want the result, searched for kids", results, err)
	}
}
