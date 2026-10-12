## 1. Open-Meteo source (`internal/sources`)

- [x] 1.1 Add the `WeatherForecast = "weather_forecast"` constant and the `ErrPlaceNotFound` and `ErrForecastUnavailable` sentinel errors. Add `wmoDescription(code int) string` with the WMO code table from design decision 2. Verify with a table test in `openmeteo_test.go` (a code from each group, plus an unknown code giving "") and `go test ./internal/sources/`.
- [x] 1.2 Add `OpenMeteoClient` (`NewOpenMeteoClient(geocodingBaseURL, forecastBaseURL)`, empty strings meaning the real Open-Meteo URLs) and its `Forecast()` source in `internal/sources/openmeteo.go`.
  - `Search` geocodes the query, fetches the forecast with the units for that country, and returns 8 results: the current conditions, then 7 days, formatted as in design decision 2.
  - A lookup with no matches returns `ErrPlaceNotFound`. A non-2xx response, a decode failure or a timeout wraps `ErrForecastUnavailable`.

  Verify with `go build ./internal/sources/`.
- [x] 1.3 Test the client against an `httptest.Server` serving the response shapes recorded in design.md. Cover:
  - a US place: the request asks for `temperature_unit=fahrenheit` and `wind_speed_unit=mph`, and snippets read "°F" and "mph";
  - a non-US place: neither parameter is sent, and snippets read "°C" and "km/h";
  - the first day is titled "Today (Mon, Oct 12)" and later days "Tue, Oct 13";
  - exactly 8 results, every one with an empty URL and Date;
  - no `results` key from geocoding gives `ErrPlaceNotFound` (checked with `errors.Is`);
  - a 500 from the forecast API gives `ErrForecastUnavailable`;
  - the audience doesn't change the requests.

  Verify with `go test ./internal/sources/`.

## 2. Planner knows the member's location (`internal/llm`)

- [x] 2.1 Add a `location string` parameter to `PickWidgetSpec`. When it isn't empty, `specSystemPrompt` adds the saved-location paragraph from design decision 3. Update the callers in `llm_test.go` and `curate_test.go`. Add a test that reads the fake OpenRouter's recorded system message: it contains the location and the "use this location" rule when one is given, and neither when the location is "". Verify with `go test ./internal/llm/`.

## 3. Widget handler (`internal/widgets`)

- [x] 3.1 Give `Handler` a `profiles ProfileStore` field and parameter in `NewHandler`, and pass `st` from `backend/routes.go`. Update the `Planner` interface to the new `PickWidgetSpec` signature. In `FromPrompt`, load the logged-in user's profile and pass its trimmed location to the planner:
  - `store.ErrNotFound` means no location;
  - any other error is logged at Warn and also means no location.

  `RefreshCustom` stays as it is (it never reads the profile). Update `fakePlanner` (`handler_test.go`, `routes_test.go`) to record the location it receives. Add a fake profile store. Add tests:
  - a member with "Austin, TX" sends that location to the planner;
  - no profile sends "";
  - a profile read error sends "" and the widget is still created;
  - refresh makes no profile read.

  Verify with `go test ./internal/widgets/`.
- [x] 3.2 Add `WeatherFreshFor = time.Hour` and `customFreshFor(source string)`, and use it in `FromStore`. Extend `freshness_internal_test.go`:
  - a `weather_forecast` widget updated 90 minutes ago is stale;
  - a `web_search` widget of the same age is not;
  - a `weather_forecast` widget updated 30 minutes ago is not.

  Verify with `go test ./internal/widgets/`.
- [x] 3.3 Map `ErrPlaceNotFound` (422) and `ErrForecastUnavailable` (502) in `writeSearchError` ahead of the Brave cases, with the wording from design decision 4. Add handler tests using a `fakeSource` that returns each error:
  - `FromPrompt` gives 422 and 502 with those messages and saves no widget;
  - `RefreshCustom` returns the error and leaves the saved items and update time unchanged.

  Verify with `go test ./internal/widgets/`.

## 4. Wire it up

- [x] 4.1 In `backend/main.go`, create `sources.NewOpenMeteoClient("", "")` and add `openMeteo.Forecast()` to `NewRegistry` after the Brave sources. No new env var or key is needed. Verify with `go vet ./...` and `go test ./...` from `backend/`, and by checking that the server starts with `make run` (or the usual run command).

## 5. Frontend

- [x] 5.1 Add `'weather_forecast'` to `SourceName` in `frontend/src/api/client.ts`, and `weather_forecast: 'a forecast'` to `SOURCE_LABELS`. Verify that `npm run build` passes.
- [x] 5.2 In `WidgetCard`, render the weather footer for `source === 'weather_forecast'`: "Forecast for “{query}” · Weather data by Open-Meteo.com", with the provider linked to `https://open-meteo.com/` (`target="_blank" rel="noopener noreferrer"`), keeping "Updated … ago". Add `'this week’s weather'` to `EXAMPLES` in `DescribeWidgetPrompt.tsx`. Verify that `npm run build` and `npm run lint` pass.

## 6. Verify in the running app

- [x] 6.1 Run the app with real keys. As a parent whose saved location is a US city:
  - describe "this week's weather": the card shows the saved city in its footer, with °F highs and lows and the Open-Meteo link;
  - describe "weather in London today": °C and km/h;
  - describe "farmers markets near me": the query names the saved location;
  - check one card's numbers against a direct `curl` of the forecast API for the same place;
  - with `sqlite3`, set the widget's `custom_widgets.refreshed_at` back 2 hours (or wait) and reload: it refreshes in the background.

  Record what you saw in this task's notes.

  **Notes (2026-10-12):** run on a throwaway database (`backend/porchlight.db.weather-check`, port 8091), with a parent saved in "Austin, TX" and a kid.
  - "This week's weather" gave the query `Austin, TX`, a table archetype and 7 days in °F. Every high, low and chance of rain matched a direct Open-Meteo call; two values are exactly x.5, and Go rounds them up.
  - "Weather in London today" first failed with place-not-found: the model wrote "London, UK", which Open-Meteo doesn't know. Fixed by:
    - a city-only geocoding fallback, tested in `TestOpenMeteoFallsBackToTheCity`;
    - asking for full country names in `Shape()`.

    After the fix: `London, United Kingdom`, in °C and km/h.
  - "Farmers markets near me" gave `events_search` with the query "farmers markets near Austin TX 2026".
  - After ageing two widgets by 2h in SQLite, the weather widget read `stale: true` and the events widget `stale: false`. Opening the page refreshed the weather card in the background ("Updated just now").
  - The footer credit was truncated on a half-width card, so it moved to its own line (design decision 8).
- [x] 6.2 As the kid, describe "weather for my soccer game Saturday". Check that it's built from `weather_forecast` and reports `kidSafe: true`, and that the parent sees the "Kid-safe" marker on it. Then, as a member with no saved location, describe "what's the weather like" and check that the place-not-found message and hint appear.

  **Notes (2026-10-12):**
  - The kid with no location got 422 "Couldn't find that place for the forecast." with the hint. The planner had written "today weather forecast".
  - With the kid's location set to Austin:
    - "weather for my soccer game Saturday", "do I need an umbrella tomorrow" and "weather in Chicago this weekend" were built from `weather_forecast` with `kidSafe: true`, and the queries were `Austin, TX`, `Austin, TX` and `Chicago, IL`;
    - the umbrella card picked out Tuesday's 12%.
  - On the kid's page, the parent saw the "Kid-safe" marker and the Open-Meteo credit on every weather card.
  - Before the `Shape()` fix, one query was "Austin, TX weather Saturday October 17 2026". That widget stays in the throwaway database only.

## 7. Docs and wrap-up

- [x] 7.1 Add a short "Data sources" note to `README.md` crediting Open-Meteo (CC BY 4.0, free tier for non-commercial use). Verify by reading the rendered README.
- [x] 7.2 After archiving, add one line about the weather source to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`. Verify with `git diff devpost/` that only those two lines were added.
