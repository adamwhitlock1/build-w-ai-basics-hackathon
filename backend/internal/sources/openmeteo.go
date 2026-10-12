package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The Open-Meteo API roots. Both are free and need no key (non-commercial
// use, CC BY 4.0: the card credits Open-Meteo).
const (
	DefaultOpenMeteoGeocodingURL = "https://geocoding-api.open-meteo.com/v1"
	DefaultOpenMeteoForecastURL  = "https://api.open-meteo.com/v1"
)

// WeatherForecast is the name of the Open-Meteo forecast source.
const WeatherForecast = "weather_forecast"

var (
	// ErrPlaceNotFound is returned when the weather query names no place
	// Open-Meteo can find.
	ErrPlaceNotFound = errors.New("place not found")
	// ErrForecastUnavailable wraps every other weather failure: an HTTP
	// error, a body we can't read, or a timeout.
	ErrForecastUnavailable = errors.New("weather forecast unavailable")
)

// forecastDays is how many days of forecast a weather widget gets: today
// and the next six. With the current conditions that's 8 results, the most
// a widget shows.
const forecastDays = 7

// OpenMeteoClient calls Open-Meteo's geocoding and forecast APIs.
type OpenMeteoClient struct {
	geocodingURL string
	forecastURL  string
	http         *http.Client
}

// NewOpenMeteoClient returns a client for Open-Meteo. Empty base URLs mean
// the real APIs; tests pass an httptest server's URL instead.
func NewOpenMeteoClient(geocodingBaseURL, forecastBaseURL string) *OpenMeteoClient {
	if geocodingBaseURL == "" {
		geocodingBaseURL = DefaultOpenMeteoGeocodingURL
	}
	if forecastBaseURL == "" {
		forecastBaseURL = DefaultOpenMeteoForecastURL
	}
	return &OpenMeteoClient{
		geocodingURL: strings.TrimRight(geocodingBaseURL, "/"),
		forecastURL:  strings.TrimRight(forecastBaseURL, "/"),
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Forecast returns the "weather_forecast" source.
func (c *OpenMeteoClient) Forecast() *OpenMeteoSource {
	return &OpenMeteoSource{client: c}
}

// OpenMeteoSource is a Source whose query is a place and whose results are
// that place's current weather and daily forecast.
type OpenMeteoSource struct {
	client *OpenMeteoClient
}

// Name implements Source.
func (s *OpenMeteoSource) Name() string { return WeatherForecast }

// Shape implements Source.
func (s *OpenMeteoSource) Shape() string {
	return `current weather and a 7-day forecast for one place. The query must be only the place, written "City, ST" for the US or "City, Country" elsewhere, with the country's full name (e.g. "Denver, CO", "London, United Kingdom"). Never put days, dates or other words in the query: the forecast always covers today and the next 6 days. Best for weather, temperature, rain, what to wear. Show today's conditions as key_value, several days as table or list.`
}

// Search implements Source. It looks up the place named by query, then
// returns its current conditions followed by one result per day, each
// written out as short text facts. A forecast has nothing to filter, so aud
// is ignored.
func (s *OpenMeteoSource) Search(ctx context.Context, query string, _ Audience) ([]Result, error) {
	place, err := s.client.geocode(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", WeatherForecast, err)
	}
	units := unitsFor(place.CountryCode)
	fc, err := s.client.forecast(ctx, place, units)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", WeatherForecast, err)
	}
	return fc.results(place, units), nil
}

// place is the subset of an Open-Meteo geocoding result we use.
type place struct {
	Name        string  `json:"name"`
	Admin1      string  `json:"admin1"`  // state or region, e.g. "Colorado"
	Country     string  `json:"country"` // e.g. "United States"
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

// label names the place for people, e.g. "Denver, Colorado".
func (p place) label() string {
	region := p.Admin1
	if region == "" || region == p.Name {
		region = p.Country
	}
	if region == "" {
		return p.Name
	}
	return p.Name + ", " + region
}

// geocode returns the best match for query. Open-Meteo lists the most
// populous match first, which is what "Springfield" or "Denver" usually
// means. It knows "Denver, CO" and "London, United Kingdom" but not every
// way of writing a region ("London, UK" finds nothing), so when "City,
// Region" finds nothing it tries the city alone.
func (c *OpenMeteoClient) geocode(ctx context.Context, query string) (place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return place{}, ErrPlaceNotFound
	}
	p, found, err := c.search(ctx, query)
	if err != nil || found {
		return p, err
	}
	// strings.Cut splits at the first comma; ok is false if there isn't one.
	if city, _, ok := strings.Cut(query, ","); ok && strings.TrimSpace(city) != "" {
		p, found, err = c.search(ctx, strings.TrimSpace(city))
		if err != nil || found {
			return p, err
		}
	}
	return place{}, fmt.Errorf("%w: %q", ErrPlaceNotFound, query)
}

// search calls GET /search for name and returns its first match, if any.
func (c *OpenMeteoClient) search(ctx context.Context, name string) (p place, found bool, err error) {
	params := url.Values{"name": {name}, "count": {"5"}, "language": {"en"}, "format": {"json"}}
	var body struct {
		Results []place `json:"results"` // missing entirely when nothing matches
	}
	if err := c.get(ctx, c.geocodingURL+"/search", params, &body); err != nil {
		return place{}, false, err
	}
	if len(body.Results) == 0 {
		return place{}, false, nil
	}
	return body.Results[0], true, nil
}

// units says how temperatures and wind speeds are written for a place.
type units struct {
	imperial bool
	temp     string // "°F" or "°C"
	wind     string // "mph" or "km/h"
}

// fahrenheitCountries are the country codes (the US and its territories)
// whose forecasts use °F and mph. Everywhere else gets °C and km/h.
var fahrenheitCountries = []string{"US", "PR", "GU", "VI", "AS", "MP"}

func unitsFor(countryCode string) units {
	if slices.Contains(fahrenheitCountries, strings.ToUpper(countryCode)) {
		return units{imperial: true, temp: "°F", wind: "mph"}
	}
	return units{temp: "°C", wind: "km/h"}
}

// forecastResponse is the subset of Open-Meteo's /forecast body we use.
// Daily values are pointers because Open-Meteo sends null for values it
// doesn't have (often the chance of rain far ahead).
type forecastResponse struct {
	Current struct {
		Temperature *float64 `json:"temperature_2m"`
		FeelsLike   *float64 `json:"apparent_temperature"`
		WeatherCode *int     `json:"weather_code"`
		WindSpeed   *float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Time        []string   `json:"time"` // the place's local dates, YYYY-MM-DD, starting today
		WeatherCode []*int     `json:"weather_code"`
		High        []*float64 `json:"temperature_2m_max"`
		Low         []*float64 `json:"temperature_2m_min"`
		RainChance  []*float64 `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// forecast calls GET /forecast for p's coordinates, in p's local time zone.
func (c *OpenMeteoClient) forecast(ctx context.Context, p place, u units) (forecastResponse, error) {
	params := url.Values{
		"latitude":      {strconv.FormatFloat(p.Latitude, 'f', -1, 64)},
		"longitude":     {strconv.FormatFloat(p.Longitude, 'f', -1, 64)},
		"current":       {"temperature_2m,apparent_temperature,weather_code,wind_speed_10m"},
		"daily":         {"weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"},
		"timezone":      {"auto"},
		"forecast_days": {strconv.Itoa(forecastDays)},
	}
	if u.imperial {
		params.Set("temperature_unit", "fahrenheit")
		params.Set("wind_speed_unit", "mph")
	}
	var body forecastResponse
	if err := c.get(ctx, c.forecastURL+"/forecast", params, &body); err != nil {
		return forecastResponse{}, err
	}
	return body, nil
}

// results writes the forecast out as search results: the current
// conditions, then one per day. The day goes in Title rather than Date,
// because the shaping prompt treats a result's Date as when a page was
// published, not when something happens.
func (f forecastResponse) results(p place, u units) []Result {
	results := make([]Result, 0, 1+forecastDays)

	cur := f.Current
	var now []string
	if cur.Temperature != nil {
		s := degrees(*cur.Temperature, u)
		if cur.FeelsLike != nil {
			s += ", feels like " + degrees(*cur.FeelsLike, u)
		}
		now = append(now, s)
	}
	if cur.WeatherCode != nil {
		now = appendNonEmpty(now, wmoDescription(*cur.WeatherCode))
	}
	if cur.WindSpeed != nil {
		now = append(now, fmt.Sprintf("Wind %d %s", round(*cur.WindSpeed), u.wind))
	}
	if len(now) > 0 {
		results = append(results, Result{
			Title:   "Right now in " + p.label(),
			Snippet: sentences(now),
			Site:    "Open-Meteo",
		})
	}

	for i, day := range f.Daily.Time {
		if i == forecastDays {
			break
		}
		d, err := time.Parse(time.DateOnly, day)
		if err != nil {
			continue
		}
		title := d.Format("Mon, Jan 2")
		if i == 0 { // the daily list starts at the place's local today
			title = "Today (" + title + ")"
		}

		var parts []string
		if code := at(f.Daily.WeatherCode, i); code != nil {
			parts = appendNonEmpty(parts, wmoDescription(*code))
		}
		high, low := at(f.Daily.High, i), at(f.Daily.Low, i)
		switch {
		case high != nil && low != nil:
			parts = append(parts, fmt.Sprintf("High %s, low %s", degrees(*high, u), degrees(*low, u)))
		case high != nil:
			parts = append(parts, "High "+degrees(*high, u))
		case low != nil:
			parts = append(parts, "Low "+degrees(*low, u))
		}
		if rain := at(f.Daily.RainChance, i); rain != nil {
			parts = append(parts, fmt.Sprintf("%d%% chance of rain", round(*rain)))
		}
		if len(parts) == 0 {
			continue
		}
		results = append(results, Result{Title: title, Snippet: sentences(parts), Site: "Open-Meteo"})
	}
	return results
}

// at returns s[i], or nil when s is too short: Open-Meteo's daily arrays
// should all match "time", but a short one shouldn't panic.
func at[T any](s []*T, i int) *T {
	if i < len(s) {
		return s[i]
	}
	return nil
}

func round(v float64) int { return int(math.Round(v)) }

func degrees(v float64, u units) string { return fmt.Sprintf("%d%s", round(v), u.temp) }

func appendNonEmpty(parts []string, s string) []string {
	if s == "" {
		return parts
	}
	return append(parts, s)
}

// sentences joins parts into "A. B. C."
func sentences(parts []string) string {
	return strings.Join(parts, ". ") + "."
}

// wmoDescription turns a WMO weather code, as Open-Meteo reports it, into
// plain words. Unknown codes give "".
func wmoDescription(code int) string {
	switch code {
	case 0:
		return "Clear sky"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55:
		return "Drizzle"
	case 56, 57:
		return "Freezing drizzle"
	case 61, 63:
		return "Rain"
	case 65:
		return "Heavy rain"
	case 66, 67:
		return "Freezing rain"
	case 71, 73, 75, 77:
		return "Snow"
	case 80, 81:
		return "Rain showers"
	case 82:
		return "Heavy rain showers"
	case 85, 86:
		return "Snow showers"
	case 95:
		return "Thunderstorm"
	case 96, 99:
		return "Thunderstorm with hail"
	}
	return ""
}

// get calls endpoint with params and decodes the JSON body into out. Every
// failure wraps ErrForecastUnavailable; a timeout also still matches
// context.DeadlineExceeded, because the error wraps both (Go 1.20+ allows
// more than one %w).
func (c *OpenMeteoClient) get(ctx context.Context, endpoint string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("%w: build request: %w", ErrForecastUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: call open-meteo: %w", ErrForecastUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: open-meteo returned HTTP %d: %s", ErrForecastUnavailable, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: decode open-meteo response: %w", ErrForecastUnavailable, err)
	}
	return nil
}
