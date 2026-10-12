# weather-widgets Specification

## Purpose
Lets a family member describe a weather widget in a sentence and get a real forecast for the right place, kept current and properly credited to its data provider.

## Requirements

### Requirement: Weather requests use a forecast source
The system SHALL offer a weather forecast source among the sources a described widget can draw from. When a member describes a widget about weather (conditions, temperature, rain, what to wear), the system SHALL fill it from that source rather than from a web, news or events search. The widget's saved query SHALL be the place the forecast is for.

#### Scenario: Describing a weather widget for a named place
- **WHEN** a member describes "weather in Denver this week"
- **THEN** the widget is built from the forecast source with the query "Denver, CO" or similar
- **AND** its items come from the forecast for Denver, Colorado, not from web pages

#### Scenario: Non-weather requests are unaffected
- **WHEN** a member describes "upcoming SpaceX launches"
- **THEN** the widget is built from a web, news or events search as before

### Requirement: A request with no place uses the member's location
When a member describes a widget that needs a place but names none, the system SHALL use the location saved in that member's profile. A place named in the request SHALL take precedence over the saved location. The saved query SHALL record the place that was used, so that later refreshes keep the same place even if the member's location changes.

#### Scenario: Weather with no place named
- **WHEN** a member whose saved location is "Austin, TX" describes "this week's weather"
- **THEN** the widget shows the forecast for Austin, Texas
- **AND** its footer shows the query as "Austin, TX" or similar

#### Scenario: A named place wins
- **WHEN** a member whose saved location is "Austin, TX" describes "weather in Chicago"
- **THEN** the widget shows the forecast for Chicago

#### Scenario: Location changed after the widget was made
- **WHEN** a member's weather widget was made for "Austin, TX", and the member then changes their location to "Denver, CO" and refreshes the widget
- **THEN** the widget still shows the forecast for Austin

### Requirement: Forecast content
A weather widget's data SHALL come from the current conditions and the daily forecast for today and the following 6 days at the place's local dates. Current conditions SHALL include the temperature, the "feels like" temperature, a plain-words description of the sky (e.g. "Partly cloudy") and the wind speed. Each day SHALL include its description, high and low temperatures, and the highest chance of rain. Places in the United States SHALL use °F and mph; all other places SHALL use °C and km/h. The widget SHALL show only figures that appear in that forecast data.

#### Scenario: A week's forecast for a US place
- **WHEN** a member describes "the weather this week in Denver"
- **THEN** the widget's items show days with their highs and lows in °F and their chances of rain

#### Scenario: Metric units outside the US
- **WHEN** a member describes "weather in London today"
- **THEN** the temperatures are in °C and the wind speed in km/h

#### Scenario: Today's conditions
- **WHEN** a member describes "what's it like outside right now"
- **THEN** the widget shows the current temperature, the "feels like" temperature, the sky description and the wind for their saved location

### Requirement: Unknown places and service failures are reported clearly
If the place cannot be found, the system SHALL respond "unprocessable" with a message saying the place couldn't be found and a hint to name a town (for example "Denver, CO") or to add a location to the member's profile. If the weather service fails or times out, the system SHALL respond with a "bad gateway" error naming the weather service, not the search service. A refresh that fails either way SHALL keep the widget's current items and update time, like any failed refresh.

#### Scenario: Place not found when describing
- **WHEN** a member with no saved location describes "what's the weather like"
- **THEN** no widget is saved
- **AND** the member sees that the place couldn't be found, with a hint to name a town or add their location

#### Scenario: Weather service down when describing
- **WHEN** a member describes a weather widget and the weather service returns an error
- **THEN** no widget is saved, and the member sees that the weather service couldn't be reached

#### Scenario: Weather service down during refresh
- **WHEN** a weather widget is refreshed and the weather service fails
- **THEN** the widget keeps its current items and update time, and the card offers "Try again"

### Requirement: Weather cards credit the data provider
Every weather widget card SHALL show "Weather data by Open-Meteo.com" with a link to open-meteo.com, wherever the card appears and whoever is viewing it.

#### Scenario: Attribution on a member's own page
- **WHEN** a member views their weather widget
- **THEN** its footer shows "Weather data by Open-Meteo.com", linked to open-meteo.com, alongside how long ago it was updated

#### Scenario: Attribution on a viewed member's page
- **WHEN** a parent views a family member's page that has a weather widget
- **THEN** the attribution is shown there too

### Requirement: Weather widgets for kids
A weather widget for a kid SHALL be built in kid-safe mode like any other described widget: the AI steps get the under-18 guidance, and the widget reports `kidSafe: true`. Forecast data has no safe-search setting, so none is applied to it.

#### Scenario: Kid describes a weather widget
- **WHEN** a kid describes "weather for my soccer game Saturday"
- **THEN** the widget is built from the forecast source and reports `kidSafe: true`
- **AND** a parent viewing the kid's page sees the "Kid-safe" marker on it
