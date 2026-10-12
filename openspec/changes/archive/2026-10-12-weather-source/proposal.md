## Why

Weather is the first thing a family checks in the morning, and it was in the scope's data-source list, but it was never built. Today, "what's the weather this week" goes to a web search. That returns pages *about* weather rather than a forecast, so the card is vague and often out of date. A real forecast source makes the morning-glance story concrete in the demo. It's also cheap: Open-Meteo is free, needs no API key and costs no Brave credit.

## What Changes

- **New `weather_forecast` source.** It sits in the describe-a-widget source list next to news, web and events search, and is backed by Open-Meteo:
  - the geocoding API turns a place name into coordinates;
  - the forecast API returns the current conditions and the next 7 days.
- **The planner picks it** for weather requests. The query it writes is just a place, e.g. `Denver, CO`.
- **Requests with no place use the member's saved location.** "This week's weather" then means the weather where the member lives. This applies to any described widget that needs a place, e.g. "farmers markets near me", not only weather.
- **Units follow the place.** US places get °F and mph; everywhere else gets °C and km/h.
- **Weather widgets go stale after 1 hour** instead of the 6 hours other described widgets get. The existing background refresh then keeps a morning forecast current.
- **Weather cards credit Open-Meteo.** The footer reads "Weather data by Open-Meteo.com", with a link, as its CC BY 4.0 license requires.
- **Weather errors name the weather service.** A place that can't be found gets a "couldn't find that place" message telling the member to name a town or add their location, and an Open-Meteo outage says so. Both used to be reported as Brave Search errors.
- **The describe-a-widget prompt gets a weather example** in its rotating suggestions.

## Capabilities

### New Capabilities
- `weather-widgets`: describing a widget that shows a weather forecast. Covers:
  - how the place is chosen (named in the request, or the member's saved location);
  - what the forecast contains, and its units;
  - errors for unknown places and an unavailable weather service;
  - attribution on the card;
  - how weather widgets behave for kids.

### Modified Capabilities
- `widget-freshness`: the "Freshness windows" requirement gains a 1-hour window for weather widgets. Other described widgets stay at 6 hours.

## Impact

- **Backend (Go):**
  - `internal/sources`:
    - a new `OpenMeteoClient` and source, `openmeteo.go`, with tests against an `httptest` server;
    - new sentinel errors `ErrPlaceNotFound` and `ErrForecastUnavailable`.
  - `internal/llm`: `PickWidgetSpec` takes the member's saved location, and its system prompt explains how to use it.
  - `internal/widgets`:
    - `Handler` loads the member's profile (via the existing `ProfileStore` interface) to get their location;
    - freshness is chosen by source;
    - `writeSearchError` maps the two new errors.
  - `main.go` / `routes.go`: register the new source and pass the profile store to `widgets.NewHandler`.
- **API:**
  - `source` can now be `"weather_forecast"`.
  - FromPrompt and refresh can return a new 422 when a place can't be found.
  - Both changes are additive.
- **Data:** no migration. A weather widget saves the place as its `query`, like any described widget.
- **Frontend:**
  - `SourceName` in `api/client.ts` gains `weather_forecast`;
  - `WidgetCard` shows the Open-Meteo attribution footer for it;
  - a weather example joins `DescribeWidgetPrompt`'s examples.
- **External service:**
  - Open-Meteo's free API: no key, under 10,000 calls a day, non-commercial use only, CC BY 4.0. A hackathon demo fits those terms.
  - Roadmap item 4 (public deployment) must keep the attribution and stay non-commercial.
- **Docs:**
  - credit Open-Meteo in `README.md`;
  - when the change is archived, add one line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
