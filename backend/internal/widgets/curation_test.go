package widgets_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// These tests call the Curation directly, with no HTTP request, as the
// agent will. The fakes are the curated handler tests' (see curated_test.go).

// curationRig is a Curation with fake dependencies, and the fakes.
type curationRig struct {
	curation *widgets.Curation
	curator  *fakeCurator
	news     *querySource
	events   *querySource
	results  *memResults
}

func newCurationRig(profile store.MemberProfile, curator *fakeCurator, found map[string][]sources.Result, errs map[string]error) *curationRig {
	rig := &curationRig{
		curator: curator,
		news:    &querySource{name: "news_search", results: found, errs: errs},
		events:  &querySource{name: "events_search", results: found, errs: errs},
		results: newMemResults(),
	}
	rig.curation = widgets.NewCuration(curator, sources.NewRegistry(rig.news, rig.events), fakeProfiles{profile: profile}, rig.results, discardLogger)
	return rig
}

// searched is every query the rig's sources were asked: the news source's,
// sorted, then the events source's.
func (rig *curationRig) searched() []string {
	return slices.Concat(rig.news.queries(), rig.events.queries())
}

func TestCurationEmptyStates(t *testing.T) {
	ctx := context.Background()

	t.Run("articles without interests", func(t *testing.T) {
		rig := newCurationRig(store.MemberProfile{Interests: "  ", Location: "Denver"}, &fakeCurator{}, nil, nil)
		w, err := rig.curation.Articles(ctx, alice, alice, widgets.CurationOptions{})
		if err != nil || w.EmptyReason != widgets.EmptyNoInterests || w.Items == nil {
			t.Errorf("widget %+v, err %v; want no_interests with an empty list", w, err)
		}
		if len(rig.curator.called) != 0 || len(rig.searched()) != 0 || rig.results.puts != 0 {
			t.Errorf("curator %v, searched %v, remembered %d; want nothing", rig.curator.called, rig.searched(), rig.results.puts)
		}
	})

	t.Run("articles with nothing to search", func(t *testing.T) {
		rig := newCurationRig(store.MemberProfile{Interests: "and, and"}, &fakeCurator{articleErr: errors.New("model down")}, nil, nil)
		w, err := rig.curation.Articles(ctx, alice, alice, widgets.CurationOptions{})
		if err != nil || w.EmptyReason != widgets.EmptyNoInterests {
			t.Errorf("widget %+v, err %v; want no_interests", w, err)
		}
		if len(rig.searched()) != 0 || rig.results.puts != 0 {
			t.Errorf("searched %v, remembered %d; want neither", rig.searched(), rig.results.puts)
		}
	})

	t.Run("events without a location", func(t *testing.T) {
		rig := newCurationRig(store.MemberProfile{Interests: "chess"}, &fakeCurator{}, nil, nil)
		w, err := rig.curation.Events(ctx, alice, alice, widgets.CurationOptions{})
		if err != nil || w.EmptyReason != widgets.EmptyNoLocation || w.Items == nil {
			t.Errorf("widget %+v, err %v; want no_location with an empty list", w, err)
		}
		if len(rig.curator.called) != 0 || len(rig.searched()) != 0 {
			t.Errorf("curator %v, searched %v; want nothing", rig.curator.called, rig.searched())
		}
	})
}

func TestCurationRememberedHit(t *testing.T) {
	profile := store.MemberProfile{Interests: "chess"}
	rig := newCurationRig(profile, &fakeCurator{articleQueries: []string{"chess news"}}, nil, nil)
	at := time.Now().Add(-10 * time.Minute)
	if err := widgets.PrefillArticles(context.Background(), rig.results, alice, profile, []llm.Item{{Title: "Remembered", URL: "https://example.com/r"}}, at); err != nil {
		t.Fatal(err)
	}

	w, err := rig.curation.Articles(context.Background(), alice, alice, widgets.CurationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(w.Items); !slices.Equal(got, []string{"Remembered"}) || w.Stale {
		t.Errorf("items %v, stale %v; want the remembered one, fresh", got, w.Stale)
	}
	if len(rig.curator.called) != 0 || len(rig.searched()) != 0 {
		t.Errorf("curator %v, searched %v; want nothing", rig.curator.called, rig.searched())
	}

	// A refresh works it out again: the remembered one is over a minute old.
	if _, err := rig.curation.Articles(context.Background(), alice, alice, widgets.CurationOptions{Refresh: true}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rig.curator.called, []string{"CurateArticles"}) {
		t.Errorf("curator calls after a refresh = %v, want one plan", rig.curator.called)
	}
}

func TestCurationPlanningFallback(t *testing.T) {
	found := map[string][]sources.Result{"chess": {article("1")}, "go": {article("2")}}
	rig := newCurationRig(store.MemberProfile{Interests: "chess, go"}, &fakeCurator{articleErr: errors.New("model down")}, found, nil)

	w, err := rig.curation.Articles(context.Background(), alice, alice, widgets.CurationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := rig.searched(); !slices.Equal(got, []string{"chess", "go"}) {
		t.Errorf("searched %v, want the interests as written", got)
	}
	if got := titles(w.Items); !slices.Equal(got, []string{"Story 1", "Story 2"}) {
		t.Errorf("items = %v", got)
	}
	if rig.results.puts != 1 {
		t.Errorf("remembered %d times, want 1", rig.results.puts)
	}
}

func TestCurationEventErrors(t *testing.T) {
	profile := store.MemberProfile{Interests: "chess", Location: "Denver, CO"}
	ctx := context.Background()

	t.Run("extraction failure", func(t *testing.T) {
		found := map[string][]sources.Result{"chess in Denver": {article("1")}}
		extractErr := &llm.UpstreamError{Reason: "the model took too long to answer", Err: context.DeadlineExceeded}
		rig := newCurationRig(profile, &fakeCurator{eventQueries: []string{"chess in Denver"}, extractErr: extractErr}, found, nil)

		_, err := rig.curation.Events(ctx, alice, alice, widgets.CurationOptions{})
		step, ok := errors.AsType[*widgets.StepError](err)
		if !ok || step.Step != widgets.StepEventExtraction {
			t.Fatalf("err = %v, want an event extraction StepError", err)
		}
		if !errors.Is(err, extractErr) {
			t.Errorf("err = %v, want it to wrap the upstream error", err)
		}
		if rig.results.puts != 0 {
			t.Error("a failed extraction was remembered")
		}
	})

	t.Run("planning refused by OpenRouter", func(t *testing.T) {
		refused := &llm.UpstreamError{Reason: "OpenRouter is out of credits", Status: http.StatusPaymentRequired, Err: errors.New("402")}
		rig := newCurationRig(profile, &fakeCurator{eventErr: refused}, nil, nil)

		_, err := rig.curation.Events(ctx, alice, alice, widgets.CurationOptions{})
		if step, ok := errors.AsType[*widgets.StepError](err); !ok || step.Step != widgets.StepEventPlanning {
			t.Fatalf("err = %v, want an event planning StepError", err)
		}
		if len(rig.searched()) != 0 {
			t.Errorf("searched %v after OpenRouter refused; want no searches", rig.searched())
		}
	})

	t.Run("every search fails", func(t *testing.T) {
		errs := map[string]error{"a": sources.ErrRateLimited, "b": errors.New("brave: 500")}
		rig := newCurationRig(profile, &fakeCurator{eventQueries: []string{"a", "b"}}, nil, errs)

		_, err := rig.curation.Events(ctx, alice, alice, widgets.CurationOptions{})
		if step, ok := errors.AsType[*widgets.StepError](err); !ok || step.Step != widgets.StepSearch {
			t.Fatalf("err = %v, want a search StepError", err)
		}
		if !errors.Is(err, sources.ErrRateLimited) {
			t.Errorf("err = %v, want the rate limit kept among the joined errors", err)
		}
	})
}
