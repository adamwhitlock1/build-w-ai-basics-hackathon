package widgets_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// countingProfiles is a widgets.ProfileStore that counts its reads.
type countingProfiles struct {
	fakeProfiles
	reads int
}

func (c *countingProfiles) GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error) {
	c.reads++
	return c.fakeProfiles.GetMemberProfile(ctx, userID)
}

var weatherSpec = llm.WidgetSpec{Title: "This Week's Weather", Source: sources.WeatherForecast, Query: "Austin, TX", Archetype: llm.ArchetypeTable}

// describe posts prompt to h.FromPrompt as Alice.
func describe(h *widgets.Handler, prompt string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"prompt": %q}`, prompt)
	req := httptest.NewRequest(http.MethodPost, "/api/widgets/from-prompt", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), alice))
	rec := httptest.NewRecorder()
	h.FromPrompt(rec, req)
	return rec
}

// TestFromPromptSendsSavedLocation checks that the planner learns where the
// member lives, and that a missing or unreadable profile doesn't stop the
// widget being made.
func TestFromPromptSendsSavedLocation(t *testing.T) {
	tests := []struct {
		name     string
		profiles fakeProfiles
		want     string
	}{
		{"saved location", fakeProfiles{profile: store.MemberProfile{Location: "  Austin, TX "}}, "Austin, TX"},
		{"no profile", fakeProfiles{err: fmt.Errorf("get profile: %w", store.ErrNotFound)}, ""},
		{"profile read fails", fakeProfiles{err: errors.New("disk on fire")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner := &fakePlanner{spec: weatherSpec, items: shapedItems}
			src := fakeSource{name: sources.WeatherForecast, results: results}
			saved := &fakeWidgetStore{}
			h := widgets.NewHandler(planner, sources.NewRegistry(src), saved, tt.profiles, &fakeLayouts{}, discardLogger)

			rec := describe(h, "this week's weather")
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
			}
			if len(planner.locations) != 1 || planner.locations[0] != tt.want {
				t.Errorf("planner got locations %q, want [%q]", planner.locations, tt.want)
			}
			if len(saved.widgets) != 1 || saved.widgets[0].Query != "Austin, TX" {
				t.Errorf("saved %+v, want one widget whose query is the place", saved.widgets)
			}
		})
	}
}

// TestWeatherErrors checks the place-not-found and forecast-down answers,
// both when describing a widget (nothing is saved) and when refreshing one
// (the saved items and update time are kept).
func TestWeatherErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
		wantHint   string
	}{
		{"place not found", fmt.Errorf("weather_forecast: %w", sources.ErrPlaceNotFound), http.StatusUnprocessableEntity,
			"Couldn't find that place for the forecast.", "add your location to your profile"},
		{"forecast down", fmt.Errorf("weather_forecast: %w: HTTP 500", sources.ErrForecastUnavailable), http.StatusBadGateway,
			"The weather service (Open-Meteo) isn't answering right now.", "Try again"},
		{"forecast timed out", fmt.Errorf("weather_forecast: %w: %w", sources.ErrForecastUnavailable, context.DeadlineExceeded), http.StatusBadGateway,
			"The weather service (Open-Meteo) isn't answering right now.", "Try again"},
	}
	checkBody := func(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantError, wantHint string) {
		t.Helper()
		if rec.Code != wantStatus {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantStatus, rec.Body)
		}
		var body struct{ Error, Hint string }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error != wantError || !strings.Contains(body.Hint, wantHint) {
			t.Errorf("body = %+v, want error %q and a hint mentioning %q", body, wantError, wantHint)
		}
	}

	for _, tt := range tests {
		t.Run("describe/"+tt.name, func(t *testing.T) {
			src := fakeSource{name: sources.WeatherForecast, err: tt.err}
			saved, layouts := &fakeWidgetStore{}, &fakeLayouts{}
			h := widgets.NewHandler(&fakePlanner{spec: weatherSpec}, sources.NewRegistry(src), saved, fakeProfiles{}, layouts, discardLogger)

			checkBody(t, describe(h, "what's the weather like"), tt.wantStatus, tt.wantError, tt.wantHint)
			if len(saved.widgets) != 0 || len(layouts.added) != 0 {
				t.Errorf("saved %d widgets after a failed weather lookup", len(saved.widgets))
			}
		})

		t.Run("refresh/"+tt.name, func(t *testing.T) {
			old := store.CustomWidget{
				ID: 1, UserID: alice.ID, Name: "Weather", Source: sources.WeatherForecast, Query: "Austin, TX", Prompt: "this week's weather",
				RenderSpec:  json.RawMessage(`{"title":"Weather","archetype":"table","items":[{"title":"Today"}]}`),
				CreatedAt:   time.Now().Add(-3 * time.Hour).Truncate(time.Second),
				RefreshedAt: time.Now().Add(-2 * time.Hour).Truncate(time.Second),
			}
			saved := &fakeWidgetStore{widgets: []store.CustomWidget{old}}
			profiles := &countingProfiles{}
			src := fakeSource{name: sources.WeatherForecast, err: tt.err}
			h := widgets.NewHandler(&fakePlanner{}, sources.NewRegistry(src), saved, profiles, &fakeLayouts{}, discardLogger)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/widgets/custom/{id}/refresh", h.RefreshCustom)
			req := httptest.NewRequest(http.MethodPost, "/api/widgets/custom/1/refresh", nil)
			req = req.WithContext(auth.WithUser(req.Context(), alice))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			checkBody(t, rec, tt.wantStatus, tt.wantError, tt.wantHint)
			if got := saved.widgets[0]; string(got.RenderSpec) != string(old.RenderSpec) || !got.RefreshedAt.Equal(old.RefreshedAt) {
				t.Errorf("a failed refresh changed the widget: %+v", got)
			}
			// A refresh reruns the saved place; it never looks at the profile.
			if profiles.reads != 0 {
				t.Errorf("refresh read the profile %d times, want 0", profiles.reads)
			}
		})
	}
}
