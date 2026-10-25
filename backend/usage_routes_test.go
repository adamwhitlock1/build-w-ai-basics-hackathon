package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/sqlite3store"

	"kinlight/internal/auth"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/usage"
)

// fakeTokens is what each fake AI call reports using.
var fakeTokens = llm.Usage{Model: "fake/model", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}

// report tells rec about one AI call, the way llm.Client does after every
// answer: with the request's context, which carries the attribution.
func report(ctx context.Context, rec llm.UsageRecorder, step string) {
	u := fakeTokens
	u.Step = step
	rec.RecordUsage(ctx, u)
}

// reportingPlanner and reportingCurator are the usual fakes, reporting
// usage for every call.
type reportingPlanner struct {
	fakePlanner
	rec llm.UsageRecorder
}

func (p reportingPlanner) PickWidgetSpec(ctx context.Context, prompt string, c llm.Catalog, aud sources.Audience, loc string) (llm.WidgetSpec, error) {
	report(ctx, p.rec, "widget_spec")
	return p.fakePlanner.PickWidgetSpec(ctx, prompt, c, aud, loc)
}

func (p reportingPlanner) ShapeItems(ctx context.Context, prompt string, spec llm.WidgetSpec, results []sources.Result, aud sources.Audience) ([]llm.Item, error) {
	report(ctx, p.rec, "widget_items")
	return p.fakePlanner.ShapeItems(ctx, prompt, spec, results, aud)
}

type reportingCurator struct {
	fakeCurator
	rec llm.UsageRecorder
}

func (c reportingCurator) CurateArticles(ctx context.Context, interests string, aud sources.Audience) ([]string, error) {
	report(ctx, c.rec, "article_queries")
	return c.fakeCurator.CurateArticles(ctx, interests, aud)
}

// failingUsageStore can't save anything.
type failingUsageStore struct{}

func (failingUsageStore) InsertLLMUsage(context.Context, store.LLMUsage) error {
	return errors.New("disk full")
}

// startUsageServer is a test server whose fake AI reports usage to a
// usage.Recorder writing to usageStore (the server's own store if nil).
func startUsageServer(t *testing.T, usageStore usage.Store) (*httptest.Server, *store.Store, *usage.Recorder) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if usageStore == nil {
		usageStore = st
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rec := usage.New(usageStore, logger, 64)
	go rec.Run(context.Background())

	sessions := auth.NewSessionManager(sqlite3store.NewWithCleanupInterval(st.DB(), 0), logger)
	srv := httptest.NewServer(routes(deps{
		Logger:    logger,
		Store:     st,
		Sessions:  sessions,
		Planner:   reportingPlanner{rec: rec},
		Curator:   reportingCurator{rec: rec},
		Briefer:   &countingBriefer{},
		Sources:   sources.NewRegistry(fakeNews{}, fakeSource{}, fakeWeather{}),
		Calendars: fakeCalendars{},
	}))
	t.Cleanup(func() { st.Close() })
	t.Cleanup(func() { rec.Close(5 * time.Second) })
	t.Cleanup(srv.Close)
	return srv, st, rec
}

// usageRow is the attribution and labels of one llm_usage row.
type usageRow struct {
	familyID, memberID, viewerID int64
	feature, step                string
}

func usageRows(t *testing.T, st *store.Store) []usageRow {
	t.Helper()
	rows, err := st.DB().Query(`SELECT family_id, member_id, viewer_id, feature, step FROM llm_usage ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []usageRow
	for rows.Next() {
		var r usageRow
		if err := rows.Scan(&r.familyID, &r.memberID, &r.viewerID, &r.feature, &r.step); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestUsageRecordedForDescribedWidget(t *testing.T) {
	srv, st, rec := startUsageServer(t, nil)
	alice := newClient(t, srv)
	me := alice.signup("alice@example.com")
	alice.setUpFamily()

	const secret = "launches near my secret clubhouse"
	alice.do("POST", "/api/widgets/from-prompt", map[string]string{"prompt": secret}, http.StatusCreated, nil)
	rec.Flush()

	a, fam := me.User.ID, me.Family.ID
	want := []usageRow{
		{fam, a, a, "describe_widget", "widget_spec"},
		{fam, a, a, "describe_widget", "widget_items"},
	}
	if got := usageRows(t, st); !slices.Equal(got, want) {
		t.Errorf("usage rows =\n%+v\nwant\n%+v", got, want)
	}

	// Nothing alice wrote is in the usage table, in any column.
	var dump string
	if err := st.DB().QueryRow(`SELECT group_concat(created_at || family_id || member_id || viewer_id || feature || step || model
		|| prompt_tokens || completion_tokens || total_tokens, '|') FROM llm_usage`).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, "clubhouse") {
		t.Errorf("llm_usage holds the prompt text: %q", dump)
	}
}

func TestUsageAttributedToKidWhenParentLooks(t *testing.T) {
	srv, st, rec := startUsageServer(t, nil)
	alice, kid := newClient(t, srv), newClient(t, srv)
	aliceMe := alice.signup("alice@example.com")
	alice.setUpFamily()
	kidMe := kid.accept(tokenOf(alice.invite(aliceMe.Family.ID, "kid@example.com", "kid", http.StatusCreated)))

	alice.do("PUT", memberPath(kidMe.User.ID, "/profile"), map[string]string{"interests": "chess", "location": "Denver"}, http.StatusOK, nil)
	alice.do("GET", memberPath(kidMe.User.ID, "/widgets/articles"), nil, http.StatusOK, nil)
	rec.Flush()

	want := []usageRow{{aliceMe.Family.ID, kidMe.User.ID, aliceMe.User.ID, "articles", "article_queries"}}
	if got := usageRows(t, st); !slices.Equal(got, want) {
		t.Errorf("usage rows =\n%+v\nwant (the kid's card, caused by alice)\n%+v", got, want)
	}
}

func TestUsageStoreFailureDoesNotFailTheRequest(t *testing.T) {
	srv, _, rec := startUsageServer(t, failingUsageStore{})
	alice := newClient(t, srv)
	alice.signup("alice@example.com")
	alice.setUpFamily()
	alice.do("POST", "/api/widgets/from-prompt", map[string]string{"prompt": "show me upcoming SpaceX launches"}, http.StatusCreated, nil)
	rec.Flush() // the failed writes are logged, nothing more
}
