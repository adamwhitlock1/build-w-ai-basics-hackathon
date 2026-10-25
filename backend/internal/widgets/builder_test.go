package widgets_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// These tests call the WidgetBuilder directly, with no HTTP request, as the
// agent will. The fakes are the handler tests' (see handler_test.go).

func TestBuilderBuildSavesAndAppends(t *testing.T) {
	planner := &fakePlanner{spec: launchSpec, items: shapedItems}
	saved := &fakeWidgetStore{}
	layouts := &fakeLayouts{}
	b := widgets.NewWidgetBuilder(planner, sources.NewRegistry(fakeSource{name: "events_search", results: results}), saved, fakeProfiles{}, layouts, discardLogger)

	w, err := b.Build(context.Background(), alice, "show me upcoming SpaceX launches")
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != 1 || w.Title != "SpaceX Launches" || !slices.Equal(titles(w.Items), []string{"Starlink 10-12"}) {
		t.Errorf("widget = %+v, want the shaped launches with id 1", w)
	}
	if !planner.hadDeadline {
		t.Error("planning ran without a deadline; Build should apply RequestBudget")
	}
	if len(saved.widgets) != 1 || saved.widgets[0].UserID != alice.ID || saved.widgets[0].Prompt != "show me upcoming SpaceX launches" {
		t.Errorf("saved %+v, want one widget for alice with its prompt", saved.widgets)
	}
	if !slices.Equal(layouts.added, []int64{1}) || layouts.user != alice.ID {
		t.Errorf("layout added %v for user %d, want [1] for alice", layouts.added, layouts.user)
	}
}

func TestBuilderBuildErrors(t *testing.T) {
	src := fakeSource{name: "events_search", results: results}

	t.Run("unknown source", func(t *testing.T) {
		spec := launchSpec
		spec.Source = "tv_listings"
		saved := &fakeWidgetStore{}
		b := widgets.NewWidgetBuilder(&fakePlanner{spec: spec}, sources.NewRegistry(src), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)
		_, err := b.Build(context.Background(), alice, "what's on tv")
		if !errors.Is(err, widgets.ErrUnknownSource) {
			t.Errorf("err = %v, want ErrUnknownSource", err)
		}
		if len(saved.widgets) != 0 {
			t.Error("a widget with an unknown source was saved")
		}
	})

	t.Run("planning fails", func(t *testing.T) {
		b := widgets.NewWidgetBuilder(&fakePlanner{specErr: llm.ErrInvalidSpec}, sources.NewRegistry(src), &fakeWidgetStore{}, fakeProfiles{}, &fakeLayouts{}, discardLogger)
		_, err := b.Build(context.Background(), alice, "gibberish")
		if step, ok := errors.AsType[*widgets.StepError](err); !ok || step.Step != widgets.StepPlanning || !errors.Is(err, llm.ErrInvalidSpec) {
			t.Errorf("err = %v, want a planning StepError wrapping ErrInvalidSpec", err)
		}
	})

	t.Run("save fails", func(t *testing.T) {
		saved := &fakeWidgetStore{err: errors.New("disk full")}
		b := widgets.NewWidgetBuilder(&fakePlanner{spec: launchSpec, items: shapedItems}, sources.NewRegistry(src), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)
		_, err := b.Build(context.Background(), alice, "show me upcoming SpaceX launches")
		if !errors.Is(err, widgets.ErrNotSaved) {
			t.Errorf("err = %v, want ErrNotSaved", err)
		}
	})
}

func TestBuilderShapingFallsBack(t *testing.T) {
	planner := &fakePlanner{spec: launchSpec, shapeErr: llm.ErrNoItems}
	b := widgets.NewWidgetBuilder(planner, sources.NewRegistry(fakeSource{name: "events_search", results: results}), &fakeWidgetStore{}, fakeProfiles{}, &fakeLayouts{}, discardLogger)

	w, err := b.Build(context.Background(), alice, "show me upcoming SpaceX launches")
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(w.Items); !slices.Equal(got, []string{"Launch schedule"}) || w.Items[0].Thumbnail == "" {
		t.Errorf("items = %+v, want the raw search result", w.Items)
	}
}

func TestBuilderRefresh(t *testing.T) {
	found := []sources.Result{{Title: "Raw result", URL: "https://example.com/raw"}}

	t.Run("within the guard", func(t *testing.T) {
		widget := oldLaunches("p")
		widget.RefreshedAt = time.Now().Add(-20 * time.Second).Truncate(time.Second)
		src := &countingSource{fakeSource: fakeSource{name: "events_search", results: found}}
		b := widgets.NewWidgetBuilder(&fakePlanner{items: shapedItems}, sources.NewRegistry(src), &fakeWidgetStore{widgets: []store.CustomWidget{widget}}, fakeProfiles{}, &fakeLayouts{}, discardLogger)

		w, err := b.Refresh(context.Background(), alice, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := titles(w.Items); !slices.Equal(got, []string{"Old launch"}) {
			t.Errorf("items = %v, want the saved ones", got)
		}
		if n := src.searches.Load(); n != 0 {
			t.Errorf("searched %d times within the guard, want 0", n)
		}
	})

	t.Run("refreshes and saves", func(t *testing.T) {
		saved := &fakeWidgetStore{widgets: []store.CustomWidget{oldLaunches("show me launches")}}
		planner := &fakePlanner{items: shapedItems}
		b := widgets.NewWidgetBuilder(planner, sources.NewRegistry(fakeSource{name: "events_search", results: found}), saved, fakeProfiles{}, &fakeLayouts{}, discardLogger)

		w, err := b.Refresh(context.Background(), alice, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := titles(w.Items); !slices.Equal(got, []string{"Starlink 10-12"}) || w.Stale {
			t.Errorf("items %v, stale %v; want the new ones, fresh", got, w.Stale)
		}
		if planner.shapedFor != "show me launches" || saved.widgets[0].RefreshedAt.IsZero() {
			t.Errorf("shaped for %q, refreshed at %v; want the saved prompt and a new time", planner.shapedFor, saved.widgets[0].RefreshedAt)
		}
	})

	t.Run("someone else's widget", func(t *testing.T) {
		widget := oldLaunches("p")
		widget.UserID = 99
		b := widgets.NewWidgetBuilder(&fakePlanner{}, sources.NewRegistry(fakeSource{name: "events_search"}), &fakeWidgetStore{widgets: []store.CustomWidget{widget}}, fakeProfiles{}, &fakeLayouts{}, discardLogger)
		if _, err := b.Refresh(context.Background(), alice, 1); !errors.Is(err, widgets.ErrNoSuchWidget) {
			t.Errorf("err = %v, want ErrNoSuchWidget", err)
		}
	})

	t.Run("source gone", func(t *testing.T) {
		widget := oldLaunches("p")
		widget.Source = "gone_search"
		b := widgets.NewWidgetBuilder(&fakePlanner{}, sources.NewRegistry(fakeSource{name: "events_search"}), &fakeWidgetStore{widgets: []store.CustomWidget{widget}}, fakeProfiles{}, &fakeLayouts{}, discardLogger)
		if _, err := b.Refresh(context.Background(), alice, 1); !errors.Is(err, widgets.ErrSourceGone) {
			t.Errorf("err = %v, want ErrSourceGone", err)
		}
	})
}
