## Context

See proposal.md for the motivation and `specs/weather-widgets/spec.md` and `specs/widget-freshness/spec.md` for the required behavior.

What the code does today:
- **Sources.** Every data source implements `sources.Source` (`Name`, `Search(ctx, query, aud)`, `Shape`) in `internal/sources/sources.go`. `main.go` registers the three Brave sources in a `Registry`. `Registry.Describe()` pastes each source's `Shape()` hint into the planner prompt, and `specSchema` limits `source` to the registered names. A new source therefore only needs a new type and one line in `main.go`.
- **Describe-a-widget pipeline** (`internal/widgets/handler.go`). `FromPrompt` works in four steps:
  1. `PickWidgetSpec` picks the source, query and archetype.
  2. `src.Search` runs the query.
  3. `ShapeItems` turns the results into at most 8 items, and `groundItems` blanks any URL the model made up.
  4. The widget is saved.

  `RefreshCustom` reruns steps 2–3 with the saved query.
- **The planner knows nothing about the member** except the audience. `FromPrompt` only has the logged-in `store.User`. The member's location lives in `member_profiles` and is read through `ProfileStore.GetMemberProfile`, which returns `store.ErrNotFound` when there's no profile yet. Only `CuratedHandler` uses it today.
- **Errors.** `writeSearchError` (`handler.go`) words every search failure as a Brave Search failure.
- **Freshness.** `FromStore` uses `CustomFreshFor` (6h) for every described widget (`internal/widgets/freshness.go`). The frontend refreshes stale widgets in the background on their owner's page.
- **Card footer.** `WidgetCard` renders "From {sourceLabel} for “{query}”" from a `SOURCE_LABELS` map keyed by `SourceName` (`frontend/src/widgets/WidgetCard.tsx`).
- **Open-Meteo** (checked live on 2026-10-12):
  - **Geocoding:** `GET https://geocoding-api.open-meteo.com/v1/search?name=…&count=…&language=en&format=json` returns `results[]` with `name`, `admin1`, `country_code`, `latitude`, `longitude` and `timezone`. "Denver, CO", "Portland, OR" and "Austin, TX, USA" all resolve, but "Portland Oregon" (no comma) returns nothing. An unknown name returns no `results` key at all, not an error.
  - **Forecast:** `GET https://api.open-meteo.com/v1/forecast?latitude=…&longitude=…&current=temperature_2m,apparent_temperature,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=auto&forecast_days=7`, plus `temperature_unit=fahrenheit&wind_speed_unit=mph` for °F and mph. With `timezone=auto`, `daily.time` holds the place's local dates and starts at its local today.

## Goals / Non-Goals

**Goals:**
- Weather goes through the existing describe → search → shape → save pipeline as just another `Source`. It doesn't get a parallel code path.
- Forecast numbers reach the card unchanged. The model arranges them; it never computes or converts them.
- The member's saved location reaches the planner in one explicit argument.

**Non-Goals:**
- **A built-in weather card** (like Articles or "Happening near you"). Weather is a described widget the member asks for. A curated card can follow later.
- **Hourly forecasts, weather alerts, radar or icons.** The existing archetypes render text only.
- **A per-member °C/°F setting.** Units follow the place (see decision 5).
- **Remembering geocoding results.** A refresh looks the place up again, which is one extra free call (see Risks).
- **Weather for a parent viewing someone else's page.** Only the owner describes and refreshes widgets, as today.

## Decisions

### 1. A new `weather_forecast` source in `internal/sources/openmeteo.go`
`OpenMeteoClient` holds an `*http.Client` (15s timeout, like Brave) and two base URLs, so tests can point it at an `httptest.Server`:

```go
func NewOpenMeteoClient(geocodingBaseURL, forecastBaseURL string) *OpenMeteoClient
func (c *OpenMeteoClient) Forecast() *OpenMeteoSource // Name() == "weather_forecast"
```

`Search(ctx, query, aud)` works in two steps:
1. **Geocode** the query, asking for up to 5 matches (`count=5`), and take the first. Open-Meteo ranks the biggest place first, which is right for "Springfield" or "Denver". If "City, Region" finds nothing, try the city alone, the text before the first comma. Open-Meteo knows "London, United Kingdom" and "London, GB" but not "London, UK", which the live check showed the model writes. Still no match returns `ErrPlaceNotFound`.
2. **Fetch** the forecast for that place's coordinates.

It ignores `aud`, because forecasts have nothing to filter (see decision 7).

Its `Shape()` hint tells the planner when to pick it and what the query must look like:

> current weather and a 7-day forecast for one place. The query must be only the place, written "City, ST" for the US or "City, Country" elsewhere, with the country's full name (e.g. "Denver, CO", "London, United Kingdom"). Never put days, dates or other words in the query: the forecast always covers today and the next 6 days. Best for weather, temperature, rain, what to wear. Show today's conditions as key_value, several days as table or list.

The last two sentences of the query rule came from the live check. Before them, the model wrote "London, UK" and "Austin, TX weather Saturday October 17 2026". The second only resolved thanks to the city-only fallback, and its footer read badly.

Alternatives considered:
- **Search weather through Brave** (what happens today): returns pages about weather rather than a forecast, and costs credit.
- **A weather API that needs a key** (OpenWeatherMap, WeatherAPI): one more secret to manage for deployment, for no gain at this scale.

### 2. Forecast → `[]sources.Result`, one result per row the card can show
The source returns the forecast already written out as short text facts, so `ShapeItems` only has to choose and arrange them:
- **First result, the current conditions:**
  - `Title`: "Right now in Denver, Colorado"
  - `Snippet`: "67°F, feels like 61°F. Overcast. Wind 8 mph."
- **Then one result per day for 7 days:**
  - `Title`: "Today (Mon, Oct 12)", then "Tue, Oct 13" and so on
  - `Snippet`: "Light drizzle. High 71°F, low 49°F. 45% chance of rain."
- **Every result** has `Site: "Open-Meteo"`. `URL`, `Date` and `Thumbnail` are empty.

That's 8 results, exactly `llm.MaxItems`.

Day names come from parsing `daily.time` (local `YYYY-MM-DD`) with `time.Parse(time.DateOnly, …)`. The first entry is labeled "Today (…)", because the forecast API starts the daily list at the place's local today.

A `wmoDescription(code int) string` table turns WMO weather codes into plain words: 0 "Clear sky", 1–3 "Mainly clear" / "Partly cloudy" / "Overcast", 45/48 "Fog", 51–57 drizzle, 61–67 rain, 71–77 snow, 80–82 rain showers, 85–86 snow showers, 95–99 "Thunderstorm". An unknown code gives "".

Temperatures are rounded to whole degrees.

Why `Date` stays empty: `shapeUserMessage` presents `Result.Date` to the model as `published`, and the shaping prompt tells it never to use that as when a thing happens. The day therefore goes in `Title`, where the model reads it as content.

Alternatives considered:
- **Build the items in Go and skip `ShapeItems` for weather.** That would be deterministic, but a request like "will I need an umbrella Saturday" couldn't be answered by picking out Saturday, and weather would need its own branch in `FromPrompt` and `RefreshCustom`. Shaping keeps one pipeline, and its failure fallback (`itemsFromResults`) already turns these results into a readable list. The "only facts stated in the results" rule plus pre-written text facts keeps the numbers grounded (see Risks).
- **Return the raw JSON as one result:** the model would have to read arrays and do date arithmetic, which is exactly where it makes mistakes.

### 3. The member's location goes to the planner as an argument
`Planner.PickWidgetSpec` gains a `location string` argument:

```go
PickWidgetSpec(ctx context.Context, prompt string, catalog llm.Catalog, aud sources.Audience, location string) (llm.WidgetSpec, error)
```

When the location is not empty, `specSystemPrompt` adds:

> The member's saved location is "Austin, TX". When the request needs a place and names none ("weather", "near me"), use this location in the query. A place named in the request wins.

`widgets.Handler` gets the existing `ProfileStore` interface (already declared in `curated.go`) as a new field, and `NewHandler` gains a `profiles ProfileStore` parameter. `routes.go` passes `st`, as it does for `NewCuratedHandler`. `FromPrompt` loads the logged-in user's profile:
- `store.ErrNotFound` counts as no location;
- any other error is logged at Warn and counts as no location. A described widget that doesn't need a place shouldn't fail because the profile read did.

`RefreshCustom` does not read the profile. It reruns the saved query, so a weather widget keeps its place (as the spec requires).

Go concept: the consumer declares the small interface it needs (`ProfileStore`), and `*store.Store` satisfies it implicitly, so the handler doesn't depend on the whole store. Tests pass a fake.

Alternatives considered:
- **Put the location in the prompt text before the LLM call** ("this week's weather (I live in Austin, TX)"): it changes the saved `prompt`, which refresh uses to shape results.
- **Have the weather source fall back to the member's location when the query is empty:** sources would then need to know about members, and the saved query would no longer record the place.

### 4. New sentinel errors, mapped in `writeSearchError`
Add to `internal/sources`:

```go
var (
	ErrPlaceNotFound       = errors.New("place not found")
	ErrForecastUnavailable = errors.New("weather forecast unavailable")
)
```

The Open-Meteo client returns `ErrPlaceNotFound` when geocoding finds no results. It wraps every other non-2xx response or decode failure as `fmt.Errorf("…: %w", ErrForecastUnavailable)`.

`writeSearchError` checks these errors before its Brave cases:

| Error | Status | Message | Hint |
| --- | --- | --- | --- |
| `ErrPlaceNotFound` | 422 | "Couldn't find that place for the forecast." | "Name a town, like “weather in Denver, CO”, or add your location to your profile." |
| `ErrForecastUnavailable` | 502 | "The weather service (Open-Meteo) isn't answering right now." | "Try again in a moment." |
| `context.DeadlineExceeded` from a weather call | 502 | as above | as above |

To tell a weather timeout from a Brave timeout, the Open-Meteo client wraps timeouts with `ErrForecastUnavailable` as well, so the `ErrForecastUnavailable` case is checked first and matches them. The curated widgets only use Brave sources, so the two new cases never trigger for them.

Go concept: `errors.Is` follows `%w` wrapping, so the handler can match the sentinel through any context the client adds. `errors.Join`/multiple `%w` (Go 1.20+) lets a timeout match both `ErrForecastUnavailable` and `context.DeadlineExceeded`.

### 5. Units follow the geocoded country
If the geocoding result's `country_code` is one of `US`, `PR`, `GU`, `VI`, `AS` or `MP` (the US and its territories), the forecast request asks for `temperature_unit=fahrenheit&wind_speed_unit=mph`. Otherwise it uses Open-Meteo's defaults, °C and km/h. The unit strings (`°F`/`°C`, `mph`/`km/h`) are chosen in Go, not read from the response's `*_units`, because Open-Meteo writes mph as "mp/h".

Alternatives considered:
- **A profile setting:** it needs a migration and UI for a demo that's US-based.
- **Always °F:** wrong for any non-US place a judge might try.

### 6. Freshness chosen by source
Add `WeatherFreshFor = time.Hour` beside the other windows, and a helper:

```go
// customFreshFor is how long a described widget's items stay fresh: a
// forecast changes faster than most searches.
func customFreshFor(source string) time.Duration
```

`FromStore` uses it instead of `CustomFreshFor`. No frontend change is needed, because the server already sends `stale` and the page already refreshes stale widgets in the background on their owner's page. `refreshGuard` (1 minute) still applies.

The source name `"weather_forecast"` becomes an exported constant, `sources.WeatherForecast`. `widgets` compares against that constant rather than a string literal.

### 7. Kids: no filter on the data, guidance still on the AI
`OpenMeteoSource.Search` ignores `aud`, because there's nothing to filter in a forecast. The handler still passes `aud` to `PickWidgetSpec` and `ShapeItems` and saves `KidSafe: aud == sources.Kids`, so a kid's weather widget reports `kidSafe: true` and parents see the marker, as the existing kid-safe-content spec requires. Nothing in `widgets` treats weather specially here.

### 8. Frontend: attribution replaces the "From …" line
- **Types.** `SourceName` in `api/client.ts` gains `'weather_forecast'`, and `SOURCE_LABELS` gains `weather_forecast: 'a forecast'`, so the `Record` type stays exhaustive.
- **Footer.** When `widget.source === 'weather_forecast'`, `WidgetCard`'s footer shows two lines:
  - "Forecast for “{query}”", which truncates on a narrow card like any other footer;
  - "Weather data by Open-Meteo.com", which is never truncated.

  The provider name links to `https://open-meteo.com/` and opens in a new tab with `rel="noopener noreferrer"`. The footer keeps the "Updated … ago" label. Putting everything on one truncated line hid the credit on a half-width card, which the license doesn't allow.
- **Example.** `'this week’s weather'` joins `EXAMPLES` in `DescribeWidgetPrompt.tsx`.

## Risks / Trade-offs

- **[The model rewrites a number]** → Each figure arrives as finished text ("High 71°F"), so the model copies rather than computes. The shaping prompt already forbids inventing numbers. The live check compares the card against the raw API response for one place.
- **[The planner writes a place Open-Meteo can't geocode]** (e.g. "Portland Oregon" without a comma) → The `Shape()` hint asks for "City, ST" / "City, Country" with the full country name, and the source retries with the city alone. If the lookup still fails, the place-not-found error tells the member how to rephrase.
- **[A vague request with no saved location geocodes to something odd]** (e.g. the planner writes "local" and a place called Local exists) → The planner prompt only offers a location when one is saved. The card's footer shows the place used ("Forecast for “…”"), so a wrong place is visible, and the member can delete the widget and describe it again.
- **[The biggest place isn't the intended one]** (Springfield, IL vs MO) → Including the state in the query fixes it, and the footer shows which place was used. Choosing among matches is left out.
- **[Two Open-Meteo calls per build or refresh]** → Both are free and fast. With a 1-hour freshness window and one family, use stays far under 10,000 calls a day. Deployment (roadmap item 4) adds rate limits anyway.
- **[Open-Meteo's terms are non-commercial]** → The hackathon demo is non-commercial. README and card attribution satisfy CC BY 4.0. If Porchlight were ever commercial, it would need Open-Meteo's paid plan, which is noted in the README.
- **[`PickWidgetSpec`'s signature change breaks test fakes]** → `fakePlanner` in `handler_test.go` and any other implementers gain the argument. The compiler finds every one.

## Migration Plan

No data migration. Existing described widgets keep the 6-hour window, and their source isn't `weather_forecast`. Deploy as usual. Rolling back removes the source: any weather widget saved meanwhile would then answer a refresh with the existing "This widget's data source isn't available any more" message, and its saved items still display.
