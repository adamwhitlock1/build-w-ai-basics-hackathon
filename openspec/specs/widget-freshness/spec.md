# widget-freshness Specification

## Purpose
Keep every widget on a member's page fast to open and visibly current. Results are remembered and shown at once, refreshed when they grow old or when asked, and labeled with when they were last updated, without spending search or AI credit on every visit.

## Requirements

### Requirement: Curated widgets show remembered results
The system SHALL remember each member's most recent Articles and Local Events results, along with the interests and location they were picked for. When the page asks for a curated widget, the system SHALL return the remembered results without searching or calling the AI, as long as the member's current interests and location match the ones the results were picked for.

#### Scenario: Returning visit within the freshness window
- **WHEN** a member whose Articles were picked 1 hour ago opens their page
- **THEN** the Articles card shows those same articles without a loading state
- **AND** no search or AI call is made for it

#### Scenario: First visit
- **WHEN** a member with interests opens their page and has no remembered Articles
- **THEN** the system picks articles as it does today, shows the loading state until they arrive, and remembers them

#### Scenario: Interests changed
- **WHEN** a member saves new interests and their page asks for Articles
- **THEN** the system ignores the remembered articles, picks new ones for the new interests, and remembers those instead

#### Scenario: Location changed
- **WHEN** a member saves a new location and their page asks for Local Events
- **THEN** the system ignores the remembered events, picks new ones for the new location, and remembers those instead

#### Scenario: Nothing to search for
- **WHEN** a member has no interests (Articles) or no location (Local Events)
- **THEN** the card shows the existing prompt to fill that in, and nothing is remembered for it

#### Scenario: A parent views a member's page
- **WHEN** a parent opens a family member's page and that member has fresh remembered results
- **THEN** the parent sees the member's remembered results without any new search
- **AND** results a parent's view picks are remembered for that member

### Requirement: Freshness windows
Every widget result SHALL carry the time it was last updated and whether it is stale. A result SHALL be stale once it is older than its kind's freshness window: the Today card 15 minutes, Articles 3 hours, Local Events 12 hours, weather widgets 1 hour, and other described widgets 6 hours. The morning briefing SHALL have no freshness window and SHALL always be reported as not stale, because it is rewritten when the facts it was written from change, not when it grows old.

#### Scenario: Result within its window
- **WHEN** Local Events were updated 5 hours ago
- **THEN** the system reports them as not stale, with that update time

#### Scenario: Result past its window
- **WHEN** Articles were updated 4 hours ago
- **THEN** the system returns them at once and reports them as stale, with that update time

#### Scenario: Described widget made before this change
- **WHEN** a described widget built before this change has never been refreshed
- **THEN** its update time is the time it was built

#### Scenario: Weather widget from earlier this morning
- **WHEN** a weather widget was updated 90 minutes ago
- **THEN** the system reports it as stale, and the owner's page refreshes it in the background

#### Scenario: Other described widget at the same age
- **WHEN** a described widget that isn't a weather widget was updated 90 minutes ago
- **THEN** the system reports it as not stale

#### Scenario: Today card after 20 minutes
- **WHEN** a member's Today events were fetched 20 minutes ago
- **THEN** the system returns them at once and reports them as stale, so the page fetches the calendar again in the background

#### Scenario: Briefing written hours ago
- **WHEN** a member's briefing was written 5 hours ago and its facts haven't changed
- **THEN** the system returns it with that update time and reports it as not stale

### Requirement: Stale widgets refresh in the background
When a widget on a member's own page is stale, the page SHALL keep showing its current items and request fresh ones once, without a loading state. It SHALL replace the items when the fresh ones arrive.

#### Scenario: Stale Articles on page open
- **WHEN** a member opens their page and their Articles are stale
- **THEN** the old articles show immediately, the refresh indicator spins, and new articles replace them when ready

#### Scenario: Stale described widget on page open
- **WHEN** a member opens their page and one of their described widgets is stale
- **THEN** that widget shows its current items immediately and refreshes itself in the background

#### Scenario: Stale described widget on a viewed member's page
- **WHEN** a parent views a family member's page that has a stale described widget
- **THEN** the widget shows its current items and update time, and does not refresh, since only its owner can refresh it

### Requirement: Refresh on request
Each curated widget card and each described widget on its owner's page SHALL have a refresh control. Using it SHALL fetch fresh results even when the current ones are not stale, and SHALL keep the current items on screen until the new ones arrive.

#### Scenario: Refreshing fresh Articles
- **WHEN** a member presses refresh on Articles that were updated 10 minutes ago
- **THEN** the system searches again, and the card shows the new picks with an update time of "just now"

#### Scenario: Refreshing a described widget
- **WHEN** a member presses refresh on their "upcoming SpaceX launches" widget
- **THEN** the system runs the widget's saved search again and shapes the results for the sentence it was described with
- **AND** the widget keeps its title and display style, and shows the new items

#### Scenario: Refreshing someone else's described widget
- **WHEN** a request asks to refresh a described widget that belongs to another user, or doesn't exist
- **THEN** the system responds "not found" and changes nothing

### Requirement: Failed or empty refreshes keep current items
A refresh that fails SHALL leave the widget's current items and update time unchanged, and the card SHALL say fresh results couldn't be loaded and offer to try again. A described-widget refresh whose search returns no results SHALL also keep the current items.

#### Scenario: Search service error during refresh
- **WHEN** a member refreshes a widget and the search service fails
- **THEN** the card keeps its current items and shows "Couldn't load fresh …" with a "Try again" button

#### Scenario: Search finds nothing for a described widget
- **WHEN** a described widget is refreshed and its search returns no results
- **THEN** the widget keeps its current items and update time

#### Scenario: Everything found has already happened
- **WHEN** a described widget about upcoming things is refreshed and every item found is in the past
- **THEN** the widget is updated with no items and shows its "nothing new" message

### Requirement: Repeated refreshes don't spend twice
A refresh request for a widget updated less than 1 minute ago SHALL return the current result without searching or calling the AI.

#### Scenario: Double refresh from two tabs
- **WHEN** a described widget is refreshed, and a second refresh for it arrives 20 seconds later
- **THEN** the second request returns the widget as just refreshed, and makes no search or AI call

### Requirement: Update time is shown on every card
Every widget card that has results SHALL show how long ago they were updated, in words ("just now", "5 min ago", "2h ago", "yesterday"), and keep that label current while the page stays open.

#### Scenario: Label after a refresh
- **WHEN** a widget finishes refreshing
- **THEN** its label reads "Updated just now"

#### Scenario: Label ages while the page is open
- **WHEN** a page stays open for 10 minutes after a widget refreshed
- **THEN** that widget's label reads "Updated 10 min ago" without reloading the page
