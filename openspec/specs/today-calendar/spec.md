# today-calendar Specification

## Purpose
Shows each family member what's on their calendar today, read from the private calendar link they (or a parent) connect, while keeping that link secret and fetching it safely.

## Requirements

### Requirement: Connecting a calendar link
A member SHALL be able to connect one calendar by pasting its private iCal link, and to replace or remove it. A parent SHALL be able to do the same for any member of their family. A kid SHALL NOT be able to change anyone else's calendar link. The system SHALL accept `https://` links, and `webcal://` links, which it SHALL treat as `https://`. Before saving a link, the system SHALL fetch it once and SHALL refuse it if the fetch fails or the response is not a calendar. A refused link SHALL leave any previously saved link in place.

#### Scenario: Member connects their calendar
- **WHEN** a member pastes a working Google Calendar secret iCal address and saves
- **THEN** the link is saved, and their profile shows "Google Calendar connected"
- **AND** their Today card shows today's events from that calendar

#### Scenario: Parent connects a kid's calendar
- **WHEN** a parent opens their kid's page, pastes a calendar link for the kid and saves
- **THEN** the link is saved for the kid, and the kid's Today card shows events from it

#### Scenario: A webcal link
- **WHEN** a member pastes a `webcal://` link
- **THEN** the system fetches it over `https` and saves it if it is a calendar

#### Scenario: A link that isn't a calendar
- **WHEN** a member pastes the address of an ordinary web page
- **THEN** nothing is saved, and the member sees that the link didn't give a calendar, with a hint about where to find the private iCal address

#### Scenario: Removing a calendar
- **WHEN** a member removes their calendar
- **THEN** the link is deleted, their remembered events are discarded, and the Today card asks them to add a calendar

#### Scenario: A kid tries to change a sibling's calendar
- **WHEN** a kid sends a request to set or remove another member's calendar link
- **THEN** the system refuses it and changes nothing

### Requirement: The calendar link stays secret
The system SHALL never send a saved calendar link back to any browser, including to the member it belongs to. API responses SHALL say only whether a calendar is connected and which service it comes from (for example "Google Calendar", "iCloud", "Outlook", or the link's host name). The link SHALL NOT appear in logs, error messages, or anything sent to the AI service.

#### Scenario: Member reloads their profile
- **WHEN** a member with a connected calendar opens the profile panel
- **THEN** it shows that a Google Calendar is connected, with options to replace or remove it, and does not show the link

#### Scenario: Parent views a kid's profile
- **WHEN** a parent opens the profile panel on their kid's page
- **THEN** it shows whether the kid has a calendar connected and from which service, never the link

### Requirement: Calendar links are fetched safely
The system SHALL fetch a calendar link only over `https`, and SHALL refuse to connect to private, loopback, link-local, multicast or unspecified network addresses, whether the link names one directly, its host name resolves to one, or a redirect leads to one. It SHALL follow at most 3 redirects, SHALL give up after 10 seconds, and SHALL stop reading after 5 MB.

#### Scenario: A link to an internal address
- **WHEN** a member pastes `https://127.0.0.1/calendar.ics`, or a link whose host name resolves to a private address such as `10.0.0.5`
- **THEN** the system makes no connection to that address and refuses the link

#### Scenario: A redirect to an internal address
- **WHEN** a calendar link redirects to `http://169.254.169.254/`
- **THEN** the system does not follow the redirect, and treats the fetch as failed

#### Scenario: An oversized feed
- **WHEN** a calendar link returns more than 5 MB
- **THEN** the system stops reading and treats the fetch as failed

### Requirement: The Today card shows the day's events
Every member's page SHALL have a Today card. It SHALL list the events from the member's calendar that take place on the current day in the member's time zone, which is the time zone of the browser showing the page. It SHALL show:
- all-day events first, then timed events in order of start time;
- each event's title and, for a timed event, its start and end time;
- where the event is, when the calendar gives a location.

An event that started on an earlier day and is still going on SHALL be included, and so SHALL each day of a multi-day all-day event. Events that have already ended SHALL be shown dimmed. Cancelled events SHALL NOT be shown. From 6 pm local time, the card SHALL also show the next day's events under a "Tomorrow" heading. Event descriptions, attendees and other details SHALL NOT be shown or sent to the browser. When a day has more than 12 events, the card SHALL show the first 12 and say how many more there are.

#### Scenario: A normal day
- **WHEN** a member's calendar has a 3:30–4:30 pm dentist appointment and a 6 pm soccer practice today
- **THEN** the Today card lists both, in that order, with their times and locations

#### Scenario: All-day and multi-day events
- **WHEN** a member's calendar has a "Spring break" all-day event running Monday to Friday, and today is Wednesday
- **THEN** the Today card shows "Spring break" as an all-day event above today's timed events

#### Scenario: Evening view
- **WHEN** a member opens their page at 7 pm
- **THEN** the card shows today's events, with the ones that are over dimmed, and tomorrow's events under "Tomorrow"

#### Scenario: Morning view
- **WHEN** a member opens their page at 7 am
- **THEN** the card shows only today's events

#### Scenario: Time zone
- **WHEN** a calendar event is set for 9 am in New York and the member's browser is in Denver
- **THEN** the card shows it at 7 am

#### Scenario: Nothing today
- **WHEN** a member's calendar has no events today
- **THEN** the card says there's nothing on the calendar today

### Requirement: Repeating events
The Today card SHALL show each occurrence of a repeating event on the days its repeat rule produces, in the event's own time zone (so a weekly 9 am event stays at 9 am across daylight-saving changes). An occurrence removed from the series SHALL NOT be shown. An occurrence that was moved or edited on its own SHALL be shown at its new time, with its new details, instead of the original.

#### Scenario: Weekly practice
- **WHEN** a member's calendar has soccer practice every Tuesday and Thursday at 6 pm, and today is Thursday
- **THEN** the card shows soccer practice at 6 pm

#### Scenario: A skipped occurrence
- **WHEN** this Thursday's practice was deleted from the series
- **THEN** the card does not show practice today

#### Scenario: A moved occurrence
- **WHEN** this Thursday's practice was moved to 7 pm
- **THEN** the card shows practice at 7 pm only

### Requirement: Today card states and refresh
A member with no calendar connected SHALL see a prompt to add one that opens the profile panel at the calendar field ("Add their calendar" when a parent is viewing a family member). The card SHALL behave like the other curated cards: a refresh button, "Updated … ago", remembered events shown at once, and a background refresh when they are stale. A failed fetch SHALL keep the events already shown and offer "Try again". When the calendar service answers that the link no longer exists or is no longer allowed, the card SHALL say the calendar link stopped working and offer to replace it. A new day SHALL never show the previous day's remembered events as today's.

#### Scenario: No calendar yet
- **WHEN** a member with no calendar connected opens their page
- **THEN** the Today card says "Add your calendar to see what's on today" with a button that opens the profile panel at the calendar field

#### Scenario: Calendar service down
- **WHEN** the Today card refreshes and the calendar service doesn't answer
- **THEN** the card keeps the events it was showing, and offers "Try again"

#### Scenario: Revoked link
- **WHEN** a member reset their secret address in Google Calendar, so the saved link now returns "not found"
- **THEN** the card says their calendar link stopped working, with a button to replace it

#### Scenario: Next morning
- **WHEN** a member's events were remembered yesterday evening and they open their page the next morning
- **THEN** the card fetches today's events instead of showing yesterday's

### Requirement: The Today card on every page
The Today card SHALL be a permanent card, like Articles and Local Events: every page has exactly one, members can move and resize it, and it can't be removed. A page that has never been arranged SHALL show it first, at full width. A page arranged before this change SHALL get it added at the top. Kid-safe filtering SHALL NOT apply to a member's own calendar, and the Today card SHALL NOT show the "Kid-safe" marker.

#### Scenario: New member's page
- **WHEN** a member who has never arranged their page opens it
- **THEN** the Today card is the first card, across the full width of the page

#### Scenario: Page arranged before the card existed
- **WHEN** a member whose saved layout has no Today card opens their page
- **THEN** the Today card appears at the top, and the rest of their layout is unchanged
