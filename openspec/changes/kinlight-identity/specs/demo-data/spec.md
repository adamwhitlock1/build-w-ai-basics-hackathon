## MODIFIED Requirements

### Requirement: Fixture calendar feeds
When the development fixtures setting is on, the server SHALL answer calendar links on the fixture host `fixtures.kinlight.test` itself, without any network connection. Links of the form `https://fixtures.kinlight.test/calendars/<name>.ics` SHALL return a calendar feed generated at request time, relative to the current date and time. A fixture name that does not exist SHALL behave like a calendar link that no longer works. Together, the demo family's feeds SHALL contain:
- an event today that has already ended;
- at least one timed event later today;
- an all-day event today;
- at least one event tomorrow evening;
- a repeating event with one occurrence deleted;
- a repeating event with one occurrence moved to another time.

Event times in the feeds SHALL be local clock times, so they show at the same times whichever time zone the viewer is in. Fixture links SHALL be accepted, saved and summarized like any other calendar link, and SHALL stay secret like any other calendar link.

#### Scenario: Today card at any hour
- **WHEN** fixtures are on and Maya's page is opened at any time of day
- **THEN** the Today card lists today's fixture events in time order, with the all-day event first, and the already-ended event shown dimmed

#### Scenario: Deleted and moved occurrences
- **WHEN** fixtures are on and a member whose feed has a repeating event opens their Today card
- **THEN** the deleted occurrence doesn't appear on its day, and the moved occurrence appears only at its new time

#### Scenario: Evening shows tomorrow
- **WHEN** fixtures are on and a member with a fixture calendar opens their page after 6 pm
- **THEN** the Today card shows tomorrow's fixture events under "Tomorrow"

#### Scenario: Briefing uses fixture events
- **WHEN** fixtures are on and Maya's briefing is written
- **THEN** it can mention Maya's fixture events, and its list of sources includes "calendar"

#### Scenario: Connecting a fixture link by hand
- **WHEN** fixtures are on and Lily (or a parent, for Lily) pastes a fixture link and saves
- **THEN** it is accepted like any working calendar link, and Lily's Today card shows that feed's events

#### Scenario: Unknown fixture name
- **WHEN** fixtures are on and a member's link names a fixture that doesn't exist
- **THEN** the Today card reports that the calendar link no longer works
