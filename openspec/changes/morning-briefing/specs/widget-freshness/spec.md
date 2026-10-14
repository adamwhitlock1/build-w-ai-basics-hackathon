## MODIFIED Requirements

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
