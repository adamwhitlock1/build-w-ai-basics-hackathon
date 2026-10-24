# demo-data Specification

## Purpose
Gives developers and demo-givers a complete, repeatable family to test with: every member's path through the app, calendars with events today, and pre-filled cards, all without private links or paid searches.

## Requirements

### Requirement: Rebuilding the demo database
The system SHALL provide one command that builds the demo data into a dedicated local database file, separate from the default database. The command SHALL delete that file first, so every run produces the same family, members, widgets, layouts, calendars and invites. It SHALL NOT modify any other database file. It SHALL print each account's email, its role, what it is meant to test, the shared password, and the pending invite's link. The existing test-family seed command and its accounts SHALL keep working unchanged.

#### Scenario: Fresh rebuild
- **WHEN** a developer runs the demo command
- **THEN** the demo database file is created with the demo families, and every account can log in with the shared password
- **AND** the output lists each account and the invite link

#### Scenario: Rebuilding after testing
- **WHEN** a developer has changed the demo data (edited a profile, accepted the invite, deleted a widget) and runs the demo command again
- **THEN** the demo database is back to its original state

#### Scenario: Other databases are untouched
- **WHEN** the demo command runs
- **THEN** the default database file and any other database file are unchanged

### Requirement: The Rivera demo family
The demo data SHALL contain a family named "Rivera Family" with these members. All of them SHALL use `.test` email addresses:
- **Maya**, a parent and the family's creator. She SHALL have interests, a location, a connected calendar, three described widgets (one of them a weather widget) and a saved page arrangement that differs from the default.
- **Sam**, a parent with different interests from Maya's, a location and a connected calendar. Sam SHALL have no saved arrangement, so Sam's page is the default one.
- **Jordan**, a kid, with kid-appropriate interests, a location, a connected calendar and one described widget made in kid-safe mode.
- **Lily**, a kid, with kid-appropriate interests and a location, and no calendar.
- **Grandma**, who is not yet a member: a pending, unexpired invite to join as a parent.

Profiles, widgets and layouts SHALL be valid by the same rules the app enforces when members create them, so every page opens without errors.

#### Scenario: A parent logs in
- **WHEN** someone logs in as Maya
- **THEN** Maya's page shows Maya's own arrangement, with the briefing, Today, Articles, "Happening near you" and the three described widgets
- **AND** the family panel lists Sam, Jordan and Lily as members and Grandma as invited

#### Scenario: A kid with no calendar
- **WHEN** someone logs in as Lily
- **THEN** Lily's Today card asks to add a calendar, and Lily's other cards show kid-safe content

#### Scenario: Accepting the pending invite
- **WHEN** someone opens Grandma's printed invite link and sets a password
- **THEN** Grandma joins the Rivera family as a parent and can see the other members' pages

#### Scenario: Greeting by name
- **WHEN** any Rivera member opens their page
- **THEN** the greeting uses their first name, for example "Good morning, Jordan"

### Requirement: A second family for isolation checks
The demo data SHALL contain a second, smaller family with one parent and one kid. Each SHALL have at least a profile, and the parent SHALL have one described widget. Nothing in either family SHALL be visible to or changeable by members of the other family.

#### Scenario: A parent asks for another family's member
- **WHEN** Maya requests the page, widgets or calendar of a member of the second family by id
- **THEN** the system answers as if that member doesn't exist, and changes nothing

#### Scenario: Another family's widget
- **WHEN** a member of the second family requests one of Maya's described widgets by id
- **THEN** the system answers as if the widget doesn't exist

### Requirement: Fixture calendar feeds
When the development fixtures setting is on, the server SHALL answer calendar links on the fixture host `fixtures.porchlight.test` itself, without any network connection. Links of the form `https://fixtures.porchlight.test/calendars/<name>.ics` SHALL return a calendar feed generated at request time, relative to the current date and time. A fixture name that does not exist SHALL behave like a calendar link that no longer works. Together, the demo family's feeds SHALL contain:
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

### Requirement: Fixtures are off unless turned on
The fixture feeds SHALL be served only when the development fixtures setting is explicitly turned on. With it off, which is the default, a fixture link SHALL be fetched like any other link, and SHALL fail as unreachable because the `.test` domain never resolves. The server SHALL log a startup warning while fixtures are on. Fetching for every other link SHALL behave exactly as before, including its refusal of private and local network addresses, whether fixtures are on or off.

#### Scenario: Fixtures off
- **WHEN** the setting is off and a member has a fixture link saved
- **THEN** the Today card reports the calendar as unavailable, and the server makes no fixture response

#### Scenario: Fixtures on, a real link
- **WHEN** the setting is on and a member's link is a real Google Calendar address
- **THEN** it is fetched from Google with the same safety checks as when the setting is off

#### Scenario: Fixtures on, a private address
- **WHEN** the setting is on and someone pastes a link to a private or local network address
- **THEN** it is refused exactly as when the setting is off

### Requirement: Real calendar links from the environment
The demo command SHALL read optional per-member calendar link variables from the environment and from the backend's local environment file, for example `SEED_CALENDAR_MAYA`. When one is set to a non-empty value, that member's calendar SHALL be that link instead of their fixture link. These links SHALL be written only into the demo database, and SHALL NOT be printed, logged or committed. A member with no calendar in the demo, such as Lily, SHALL stay without one.

#### Scenario: Real link override
- **WHEN** `SEED_CALENDAR_MAYA` is set to a private Google iCal address and the demo command runs
- **THEN** Maya's saved calendar is that address, the command's output says only that Maya uses a calendar from the environment, and the other members keep their fixture links

#### Scenario: No overrides
- **WHEN** no calendar variables are set
- **THEN** every member with a calendar gets their fixture link

### Requirement: Pre-filled widget results
The demo command SHALL save realistic results for every demo member's Articles, "Happening near you" and described widgets, dated at the time the command runs. Results SHALL match each member's saved interests and location, and SHALL be marked kid-safe for kids and not for parents. Opening a demo member's page while those results are fresh SHALL show them without any paid search or AI call. Once a result goes stale, or a member asks for a refresh, the app SHALL behave as it does for any member and search again. The demo command SHALL NOT pre-fill the morning briefing or the Today card: the briefing is written on the first visit each day, and Today reads the calendar feed.

#### Scenario: Browsing costs no search credit
- **WHEN** someone runs the demo command and then opens every Rivera member's page within the hour
- **THEN** the Articles, "Happening near you" and described widgets all show their pre-filled items, and no Brave search or curating AI call is made

#### Scenario: Kid content stays kid-safe
- **WHEN** someone opens Jordan's page after the demo command runs
- **THEN** Jordan's pre-filled cards are marked kid-safe and contain only kid-appropriate items

#### Scenario: Editing interests picks again
- **WHEN** Sam changes Sam's interests and the Articles card loads
- **THEN** new articles are searched for and picked, as for any member

#### Scenario: Stale results refresh as usual
- **WHEN** a pre-filled Articles result is older than its freshness window
- **THEN** it is shown marked stale, and the page asks for new results, which costs credit as usual

### Requirement: Demo data is local only
The demo data SHALL be documented as for local databases only. Its credentials are public in the repository. The demo database file SHALL be ignored by version control. The demo command SHALL refuse to write to a database file that already has users unless that file is the dedicated demo database it is about to replace.

#### Scenario: Pointing the demo at a real database
- **WHEN** a developer runs the demo command with a database path that already has users and isn't the demo database
- **THEN** it stops with an error and changes nothing

#### Scenario: Version control
- **WHEN** a developer runs the demo command and checks the repository status
- **THEN** the demo database file doesn't appear as a change
