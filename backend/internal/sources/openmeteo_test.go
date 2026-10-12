package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestWMODescription(t *testing.T) {
	tests := map[int]string{
		0:   "Clear sky",
		2:   "Partly cloudy",
		45:  "Fog",
		53:  "Drizzle",
		63:  "Rain",
		73:  "Snow",
		81:  "Rain showers",
		86:  "Snow showers",
		95:  "Thunderstorm",
		99:  "Thunderstorm with hail",
		4:   "",
		100: "",
	}
	for code, want := range tests {
		if got := wmoDescription(code); got != want {
			t.Errorf("wmoDescription(%d) = %q, want %q", code, got, want)
		}
	}
}

const denverGeocode = `{"results": [
  {"name": "Denver", "latitude": 39.73915, "longitude": -104.9847, "country_code": "US", "country": "United States", "admin1": "Colorado", "timezone": "America/Denver"},
  {"name": "Denver City", "latitude": 32.96, "longitude": -102.83, "country_code": "US", "admin1": "Texas"}
], "generationtime_ms": 0.6}`

const londonGeocode = `{"results": [
  {"name": "London", "latitude": 51.50853, "longitude": -0.12574, "country_code": "GB", "country": "United Kingdom", "admin1": "England"}
]}`

// forecastBody is a trimmed copy of a real /forecast response (2026-10-12),
// with a null chance of rain on the last day as Open-Meteo sends far ahead.
const forecastBody = `{
  "timezone": "America/Denver",
  "current": {"time": "2026-10-12T00:00", "interval": 900, "temperature_2m": 67.0, "weather_code": 3, "apparent_temperature": 60.9, "wind_speed_10m": 8.0},
  "daily": {
    "time": ["2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18"],
    "weather_code": [3, 51, 81, 0, 1, 2, 71],
    "temperature_2m_max": [81.0, 70.9, 56.5, 60.2, 64.0, 66.4, 41.6],
    "temperature_2m_min": [58.3, 49.3, 45.9, 40.1, 42.0, 44.5, 30.2],
    "precipitation_probability_max": [2, 45, 92, 0, 5, 10, null]
  }
}`

// fakeOpenMeteo serves geocoding at /geo/search and forecasts at /fc/forecast,
// recording the forecast request's query.
type fakeOpenMeteo struct {
	geocodeBody    string
	geocodeByName  map[string]string // when set, the body for each name; others find nothing
	geocodeNames   []string          // the names looked up, in order
	forecastStatus int
	forecastQuery  url.Values
	forecastCalls  int
}

func (f *fakeOpenMeteo) server(t *testing.T) *OpenMeteoSource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geo/search":
			name := r.URL.Query().Get("name")
			f.geocodeNames = append(f.geocodeNames, name)
			if f.geocodeByName == nil {
				w.Write([]byte(f.geocodeBody))
			} else if body, ok := f.geocodeByName[name]; ok {
				w.Write([]byte(body))
			} else {
				w.Write([]byte(`{"generationtime_ms": 0.4}`))
			}
		case "/fc/forecast":
			f.forecastCalls++
			f.forecastQuery = r.URL.Query()
			if f.forecastStatus != 0 {
				w.WriteHeader(f.forecastStatus)
				w.Write([]byte(`{"error": true, "reason": "boom"}`))
				return
			}
			w.Write([]byte(forecastBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return NewOpenMeteoClient(srv.URL+"/geo", srv.URL+"/fc").Forecast()
}

func TestOpenMeteoUSPlace(t *testing.T) {
	fake := &fakeOpenMeteo{geocodeBody: denverGeocode}
	results, err := fake.server(t).Search(context.Background(), "Denver, CO", Everyone)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	q := fake.forecastQuery
	for key, want := range map[string]string{
		"latitude": "39.73915", "longitude": "-104.9847", "timezone": "auto", "forecast_days": "7",
		"temperature_unit": "fahrenheit", "wind_speed_unit": "mph",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("forecast param %s = %q, want %q", key, got, want)
		}
	}

	if len(results) != 8 {
		t.Fatalf("got %d results, want 8: %+v", len(results), results)
	}
	want := []Result{
		{Title: "Right now in Denver, Colorado", Snippet: "67°F, feels like 61°F. Overcast. Wind 8 mph.", Site: "Open-Meteo"},
		{Title: "Today (Mon, Oct 12)", Snippet: "Overcast. High 81°F, low 58°F. 2% chance of rain.", Site: "Open-Meteo"},
		{Title: "Tue, Oct 13", Snippet: "Drizzle. High 71°F, low 49°F. 45% chance of rain.", Site: "Open-Meteo"},
	}
	for i, w := range want {
		got := results[i]
		if got.Title != w.Title || got.Snippet != w.Snippet || got.Site != w.Site {
			t.Errorf("result %d = %+v, want %+v", i, got, w)
		}
	}
	// A null chance of rain is left out rather than written as 0%.
	if last := results[7]; last.Title != "Sun, Oct 18" || last.Snippet != "Snow. High 42°F, low 30°F." {
		t.Errorf("last result = %+v", last)
	}
	for i, r := range results {
		if r.URL != "" || r.Date != "" {
			t.Errorf("result %d has URL %q / Date %q, want both empty", i, r.URL, r.Date)
		}
	}
}

func TestOpenMeteoMetricPlace(t *testing.T) {
	fake := &fakeOpenMeteo{geocodeBody: londonGeocode}
	results, err := fake.server(t).Search(context.Background(), "London, UK", Everyone)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if fake.forecastQuery.Has("temperature_unit") || fake.forecastQuery.Has("wind_speed_unit") {
		t.Errorf("metric place sent unit params: %v", fake.forecastQuery)
	}
	if got := results[0]; got.Title != "Right now in London, England" || !strings.Contains(got.Snippet, "°C") || !strings.Contains(got.Snippet, "km/h") {
		t.Errorf("current conditions = %+v, want °C and km/h for London", got)
	}
}

func TestOpenMeteoIgnoresAudience(t *testing.T) {
	everyone := &fakeOpenMeteo{geocodeBody: denverGeocode}
	kids := &fakeOpenMeteo{geocodeBody: denverGeocode}
	if _, err := everyone.server(t).Search(context.Background(), "Denver, CO", Everyone); err != nil {
		t.Fatal(err)
	}
	if _, err := kids.server(t).Search(context.Background(), "Denver, CO", Kids); err != nil {
		t.Fatal(err)
	}
	if everyone.forecastQuery.Encode() != kids.forecastQuery.Encode() {
		t.Errorf("kids request %v differs from everyone's %v", kids.forecastQuery, everyone.forecastQuery)
	}
}

func TestOpenMeteoPlaceNotFound(t *testing.T) {
	for _, query := range []string{"Nowhereville", "  "} {
		fake := &fakeOpenMeteo{geocodeBody: `{"generationtime_ms": 0.4}`}
		_, err := fake.server(t).Search(context.Background(), query, Everyone)
		if !errors.Is(err, ErrPlaceNotFound) {
			t.Errorf("Search(%q) error = %v, want ErrPlaceNotFound", query, err)
		}
		if fake.forecastCalls != 0 {
			t.Errorf("Search(%q) fetched a forecast for a place it couldn't find", query)
		}
	}
}

func TestOpenMeteoForecastError(t *testing.T) {
	fake := &fakeOpenMeteo{geocodeBody: denverGeocode, forecastStatus: http.StatusInternalServerError}
	_, err := fake.server(t).Search(context.Background(), "Denver, CO", Everyone)
	if !errors.Is(err, ErrForecastUnavailable) {
		t.Errorf("error = %v, want ErrForecastUnavailable", err)
	}
	if errors.Is(err, ErrPlaceNotFound) {
		t.Errorf("a forecast outage reads as place not found: %v", err)
	}
}

func TestOpenMeteoTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the request fails at once, as a timed-out one would
	fake := &fakeOpenMeteo{geocodeBody: denverGeocode}
	_, err := fake.server(t).Search(ctx, "Denver, CO", Everyone)
	if !errors.Is(err, ErrForecastUnavailable) || !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want it to match both ErrForecastUnavailable and context.Canceled", err)
	}
}

// TestOpenMeteoFallsBackToTheCity checks that a region Open-Meteo doesn't
// know ("London, UK") still finds the city, and that a city that exists
// nowhere is still not found.
func TestOpenMeteoFallsBackToTheCity(t *testing.T) {
	fake := &fakeOpenMeteo{geocodeByName: map[string]string{"London": londonGeocode}}
	results, err := fake.server(t).Search(context.Background(), "London, UK", Everyone)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := []string{"London, UK", "London"}; !slices.Equal(fake.geocodeNames, want) {
		t.Errorf("looked up %q, want %q", fake.geocodeNames, want)
	}
	if results[0].Title != "Right now in London, England" {
		t.Errorf("first result = %+v, want London's current conditions", results[0])
	}

	fake = &fakeOpenMeteo{geocodeByName: map[string]string{}}
	if _, err := fake.server(t).Search(context.Background(), "Nowhereville, ZZ", Everyone); !errors.Is(err, ErrPlaceNotFound) {
		t.Errorf("error = %v, want ErrPlaceNotFound", err)
	}
	if want := []string{"Nowhereville, ZZ", "Nowhereville"}; !slices.Equal(fake.geocodeNames, want) {
		t.Errorf("looked up %q, want %q", fake.geocodeNames, want)
	}
}
