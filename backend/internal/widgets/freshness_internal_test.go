package widgets

import (
	"testing"
	"time"

	"porchlight/internal/store"
)

func TestFreshnessWindows(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		age    time.Duration
		window time.Duration
		stale  bool
	}{
		{"articles just inside", 3*time.Hour - time.Second, ArticlesFreshFor, false},
		{"articles exactly at the window", 3 * time.Hour, ArticlesFreshFor, false},
		{"articles past", 4 * time.Hour, ArticlesFreshFor, true},
		{"events at 5 hours", 5 * time.Hour, EventsFreshFor, false},
		{"events past", 12*time.Hour + time.Second, EventsFreshFor, true},
		{"custom inside", 6*time.Hour - time.Minute, CustomFreshFor, false},
		{"custom past", 7 * time.Hour, CustomFreshFor, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshness(now.Add(-tt.age), now, tt.window)
			if got.Stale != tt.stale {
				t.Errorf("stale = %v, want %v", got.Stale, tt.stale)
			}
			if !got.UpdatedAt.Equal(now.Add(-tt.age)) {
				t.Errorf("updatedAt = %v, want %v", got.UpdatedAt, now.Add(-tt.age))
			}
		})
	}
}

func TestRecentlyUpdated(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	if !recentlyUpdated(now.Add(-20*time.Second), now) {
		t.Error("20 seconds old: want recently updated")
	}
	if recentlyUpdated(now.Add(-time.Minute), now) {
		t.Error("a minute old: want not recently updated")
	}
}

func TestFromStoreFreshness(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	spec := []byte(`{"title":"Launches","archetype":"list","items":[]}`)

	// Never refreshed: the items date from when the widget was built.
	built := now.Add(-7 * time.Hour)
	got, err := FromStore(store.CustomWidget{ID: 1, RenderSpec: spec, CreatedAt: built}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(built) || !got.Stale {
		t.Errorf("never refreshed, built 7h ago: updatedAt %v, stale %v; want %v, true", got.UpdatedAt, got.Stale, built)
	}

	// Refreshed since: the refresh time counts.
	refreshed := now.Add(-time.Hour)
	got, err = FromStore(store.CustomWidget{ID: 1, RenderSpec: spec, CreatedAt: built, RefreshedAt: refreshed}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(refreshed) || got.Stale {
		t.Errorf("refreshed 1h ago: updatedAt %v, stale %v; want %v, false", got.UpdatedAt, got.Stale, refreshed)
	}
}

// TestWeatherFreshness checks that a forecast goes stale after an hour,
// while other described widgets keep the 6-hour window.
func TestWeatherFreshness(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	spec := []byte(`{"title":"Weather","archetype":"table","items":[]}`)
	tests := []struct {
		source string
		age    time.Duration
		stale  bool
	}{
		{"weather_forecast", 90 * time.Minute, true},
		{"web_search", 90 * time.Minute, false},
		{"weather_forecast", 30 * time.Minute, false},
	}
	for _, tt := range tests {
		cw := store.CustomWidget{ID: 1, Source: tt.source, RenderSpec: spec, CreatedAt: now.Add(-tt.age)}
		got, err := FromStore(cw, now)
		if err != nil {
			t.Fatal(err)
		}
		if got.Stale != tt.stale {
			t.Errorf("%s updated %v ago: stale = %v, want %v", tt.source, tt.age, got.Stale, tt.stale)
		}
	}
}
