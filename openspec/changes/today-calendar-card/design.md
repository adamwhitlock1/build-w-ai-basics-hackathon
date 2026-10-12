## Context

See proposal.md for the motivation. The required behavior is in `specs/today-calendar/spec.md` and `specs/widget-freshness/spec.md`.

What the code does today:
- **Curated cards** (`internal/widgets/curated.go`):
  - `CuratedHandler` serves Articles and Local Events for the request's subject (`auth.SubjectFrom`), so the same handler answers `/api/widgets/...` and a parent's `/api/members/{id}/widgets/...`.
  - It remembers each result in `curated_widget_results` (`user_id`, `kind`, `inputs`, `payload`, `updated_at`), keyed by an `inputs` string. A result whose inputs no longer match is fetched again.
  - `?refresh=1` forces a new result. `freshness(updatedAt, now, window)` marks a result stale, and the page refreshes stale cards in the background.
- **Profiles** (`internal/family/handler.go`):
  - `PUT /api/me/profile` and `PUT /api/members/{id}/profile` replace `interests` and `location` in `member_profiles`.
  - `asParent` (`routes.go`) lets only parents of the same family use `/api/members/{id}/...`.
  - `GET /api/me` returns `profile: {interests, location}`.
- **Layouts** (`internal/layout/data.go`):
  - card types are `Articles`, `LocalEvents` and `Custom`;
  - `Default` puts the two curated cards first;
  - `heal` keeps each curated card at most once, and doesn't put back a curated card that is missing;
  - the frontend never lets curated cards be deleted (`permissions: { delete: false }` in `frontend/src/puck/config.tsx`), so a missing curated card only means "saved before that card existed".
- **Frontend:**
  - `CuratedWidgetCard` wraps `WidgetCard` with loading, error, "Try again" and the "add your interests" nudge (`useOpenInterestsEditor(field)`);
  - the profile fields live in `account/ProfileFields.tsx` and `account/InterestsEditor.tsx`.
- **Libraries.** Nothing in `go.mod` reads iCal. I checked the candidates:
  - `github.com/apognu/gocal` v0.9.1 is simple, but its README calls its repeat-rule handling "lightly tested", and it covers only some RRULE parts;
  - `github.com/arran4/golang-ical` v0.3.7 (Go 1.20) parses components, properties and parameters such as `TZID`;
  - `github.com/teambition/rrule-go` v1.8.2 (Go 1.16) implements the full RFC 5545 repeat rule, with `Set` handling `RDATE` and `EXDATE`.

  Both are compatible with our Go 1.22.

## Goals / Non-Goals

**Goals:**
- Correct days and times for real Google and iCloud feeds, including weekly repeats across a daylight-saving change, skipped and moved occurrences, and multi-day all-day events.
- A calendar link can never reach the browser, the logs or the AI, and the server can't be used to reach private network addresses.
- Reuse the curated-card machinery (remembered results, freshness, refresh, nudge to the profile panel) rather than build a parallel one.

**Non-Goals:**
- **More than one calendar per member**, or a merged family view (a combined family calendar stays in v2).
- **Writing to calendars**, RSVPs, reminders or notifications.
- **Encrypting the link at rest.** Anyone who can read the database can already read everything else. Revisit with multi-family hosting.
- **Windows time-zone names** (as some Outlook feeds use) beyond falling back to the member's time zone.
- **Event descriptions, attendees, video links.** They're never parsed into the response.
- **A per-member stored time zone.** The page sends the browser's (see decision 5).

## Decisions

### 1. A new `internal/calendar` package: fetch, parse, expand
The calendar code goes in one package with three parts, so the hard logic can be unit-tested against fixture files:

```go
// Fetcher gets a calendar feed over https, refusing private addresses.
type Fetcher struct{ /* *http.Client with a guarded dialer */ }
func NewFetcher() *Fetcher
func (f *Fetcher) Fetch(ctx context.Context, link string) ([]byte, error)

// Event is one occurrence on a given day: no description, no attendees.
type Event struct {
	Title, Location string
	Start, End      time.Time // in the member's time zone
	AllDay          bool
}

// Between parses feed and returns the occurrences overlapping [from, to),
// sorted all-day first, then by start. loc is the member's time zone, used
// for floating times and for all-day dates.
func Between(feed []byte, from, to time.Time, loc *time.Location) ([]Event, error)

func NormalizeLink(raw string) (string, error) // webcal:// → https://, https only
func Provider(link string) string              // "Google Calendar", "iCloud", "Outlook", or the host
```

Sentinel errors:
- `ErrNotCalendar`: the body isn't `BEGIN:VCALENDAR`, or doesn't parse;
- `ErrLinkGone`: HTTP 401, 403, 404 or 410;
- `ErrUnavailable`: everything else, wrapping the cause;
- `ErrBlockedAddress`: a private or local address.

Nothing returned or logged includes the link. Errors name the provider at most.

Alternatives considered:
- **gocal:** less code, but too risky for the repeat-rule correctness the spec requires.
- **Parsing ICS by hand:** line unfolding, parameter quoting and `TZID` handling are exactly what golang-ical already does.

### 2. Expanding events with golang-ical and rrule-go
For each `VEVENT` (skipping `STATUS:CANCELLED`):
- **Start time:**
  - `DTSTART` with `VALUE=DATE` is all-day. Its day is a date, placed at midnight in the member's time zone, and `DTEND` (exclusive) or one day sets how long it lasts.
  - `DTSTART;TZID=X` is read in `time.LoadLocation(X)`. An unknown `TZID` falls back to the member's location.
  - A UTC time (`…Z`) is read in UTC.
  - A floating time (no zone) is read in the member's location.
- **Duration:** `DTEND - DTSTART`, or `DURATION`, or zero.
- **Repeats** (`RRULE`): an `rrule.Set` built from `StrToRRule` with `Dtstart` set in the event's own location, so the 9 am wall-clock time holds across daylight-saving changes. Each `RDATE` and `EXDATE` is added, and `Set.Between(from - duration, to, true)` gives the starts whose occurrence overlaps the window.
- **Moved or edited occurrences** (`RECURRENCE-ID`): events sharing a `UID` with a `RECURRENCE-ID` replace the series occurrence whose original start equals that `RECURRENCE-ID`. The override's own times and fields are used, and a cancelled override removes the occurrence.
- **Output:** every occurrence that overlaps `[from, to)` becomes an `Event`, with times converted to the member's location. All-day events sort first, then by start, then by title.

Fixture feeds live in `internal/calendar/testdata/`:
- a Google-style feed with a `VTIMEZONE`, a weekly `BYDAY=TU,TH` series, an `EXDATE`, a moved occurrence, a multi-day all-day event, a cancelled event and a UTC event;
- an iCloud-style feed;
- a floating-time feed.

Tests freeze the window, and one test crosses the US daylight-saving change on 2026-11-01.

Go concept: an `rrule.Set` is an iterator over times. `Between` asks it only for the window we need, so a daily event that started in 2015 still costs just a few thousand steps.

### 3. A guarded HTTP client for links members paste
The server fetches a URL a member typed, which is the classic setup for server-side request forgery. `Fetcher` uses its own `http.Client`:
- **No proxy.** `Transport.Proxy = nil`, so environment proxies are ignored.
- **Every connection's actual IP is checked.** `net.Dialer.Control` runs after DNS resolution, on the IP really being dialed, so a host name that resolves (or re-resolves) to a private address is still refused. The check turns the address into a `netip.Addr`, unmaps IPv4-in-IPv6, and refuses it if it is loopback, private, link-local (unicast or multicast), multicast, unspecified, or in the carrier-grade NAT range `100.64.0.0/10`. A refused connection returns `ErrBlockedAddress`.
- **Redirects:** `CheckRedirect` allows at most 3, all `https` (and the dialer checks each one again).
- **Timeout:** `Client.Timeout` is 10 s.
- **Size cap:** the body is read through `io.LimitReader(body, 5 MB + 1)`, and anything over 5 MB is refused.
- **Headers:** `Accept: text/calendar` and a `User-Agent` naming Porchlight.

Tests need a loopback `httptest` TLS server, so the address check is a field (`allow func(netip.Addr) bool`) set by an unexported `newFetcher`, which the package's own tests call with a permissive check and their server's certificate. `NewFetcher` always uses the strict check.

Go concept: `Control` is a hook the dialer calls with the network and the resolved `ip:port` just before connecting. Checking there, not on the URL text, is what defeats DNS tricks.

### 4. The link lives in `member_profiles.calendar_url`, behind its own endpoints
- **Migration `004_calendar.sql`:** `ALTER TABLE member_profiles ADD COLUMN calendar_url TEXT NOT NULL DEFAULT ''`.
- **Store methods:**
  - `GetCalendarLink(ctx, userID)`;
  - `SetCalendarLink(ctx, userID, link)`, an upsert that leaves interests and location alone;
  - `DeleteCuratedResult(ctx, userID, kind)`, to discard remembered events.

  `UpsertMemberProfile` keeps writing only interests and location, so saving interests can never wipe a calendar link, and the reverse holds too.
- **Endpoints**, on a new `widgets.TodayHandler`, which has the fetcher and the results store:
  - `PUT /api/me/calendar` and `PUT /api/members/{id}/calendar` take `{"url": "..."}`;
  - `DELETE /api/me/calendar` and `DELETE /api/members/{id}/calendar` remove the link;
  - the member routes go behind `asParent`, like the profile route, so a kid can't reach them;
  - all four act on `auth.SubjectFrom`.
- **PUT** normalizes the link, then fetches and parses it once (with a 15 s budget):
  - `ErrNotCalendar` → 422 "That link didn't give us a calendar.", with a hint naming Google's "Secret address in iCal format" and iCloud's public calendar link;
  - `ErrBlockedAddress` → 422 "That link points somewhere Porchlight can't reach.";
  - `ErrLinkGone` or `ErrUnavailable` → 422 "Couldn't open that calendar link." with "Check it and try again";
  - success → the link is saved, the remembered `today` result is deleted, and the response is `{calendar: {connected: true, provider}}`.
- **DELETE** clears the link and the remembered result, and returns `{calendar: {connected: false, provider: ""}}`.
- **`GET /api/me`**: `MeProfile` gains `Calendar CalendarSummary` (`connected`, `provider`). `family.writeMe` fills it in from `store.GetCalendarLink` and `calendar.Provider`.
- **A parent viewing a member's page** learns whether that member's calendar is connected from the homepage response: `layout.Profile` (the `profile` of `GET /api/me/homepage` and `GET /api/members/{id}/homepage`) gains the same `calendar` summary, which the profile panel already reads. The plan had been to infer it from the Today response's `emptyReason`, but the homepage profile was simpler and already in the panel's hands, so that changed during implementation.
- **`PUT /api/me/profile`** also answers with the calendar summary, read from the store, so saving interests never makes the page think the calendar was disconnected.
- **Logging:** `logRequests` logs paths only, and the link travels in the body. Handlers log `provider` and never the link.

### 5. `GET /api/widgets/today` (and the member version)
- **Query:** `?tz=America/Denver&refresh=1`.
  - `tz` is checked with `time.LoadLocation`; a missing or unknown zone falls back to UTC (and is logged).
  - `main.go` imports `_ "time/tzdata"` so zone data is built into the binary, whatever the host has.
- **Response:**

```go
type TodayWidget struct {
	Freshness                         // updatedAt, stale
	EmptyReason EmptyReason `json:"emptyReason"` // "", "no_calendar", "calendar_gone"
	Provider    string      `json:"provider"`    // e.g. "Google Calendar", "" when none
	Date        string      `json:"date"`        // today in tz, YYYY-MM-DD
	TimeZone    string      `json:"timeZone"`
	Today       []TodayEvent `json:"today"`      // never null
	Tomorrow    []TodayEvent `json:"tomorrow"`   // never null
}
type TodayEvent struct {
	Title    string    `json:"title"`
	Location string    `json:"location"`
	Start    time.Time `json:"start"` // RFC 3339 with the tz offset
	End      time.Time `json:"end"`
	AllDay   bool      `json:"allDay"`
}
```

- **The server always returns today and tomorrow.** The browser shows "Tomorrow" from 18:00 local time and dims events whose `end` has passed, using the existing `useNow` tick. The server never needs to know the hour, so one remembered result serves the whole day. Each list is capped at 12 events, plus a `moreToday`/`moreTomorrow` count so the card can say "+3 more".
- **Remembering:** the result goes in `curated_widget_results` with `kind = "today"`, and inputs written as below.

```text
sha256(link)[:16] + "\n" + tz + "\n" + date
```

  - The hash means the inputs never hold the link itself.
  - A new link, time zone or day misses the remembered result, which satisfies "a new day never shows yesterday's events".
  - The window is `TodayFreshFor = 15 * time.Minute`, and `?refresh=1` with the existing 1-minute guard behaves as for the other curated cards.
  - The remembered payload is the `TodayWidget` minus `Freshness`.
- **Errors:**
  - no link → 200 with `emptyReason: "no_calendar"`, and nothing remembered;
  - `ErrLinkGone` → 200 with `emptyReason: "calendar_gone"` and empty lists, with the remembered result deleted;
  - `ErrUnavailable`, `ErrNotCalendar` or a timeout → 502 "Couldn't load your calendar right now." with "Try again in a moment.". The page keeps the events it already shows, as with the other curated cards.
  - The fetch and parse get a 20 s budget.
- **Kid-safety:** the response has no `kidSafe` field, and no audience is involved: it's the family's own calendar.

Alternatives considered:
- **Store a time zone in the profile:** a migration and UI for a value every browser already knows.
- **Return only today and let the server pick "tomorrow after 6 pm":** the remembered result would then depend on the hour.

### 6. Layout: a third permanent card, first and full width
- `TypeToday = "Today"`, with default id `"today"`.
- `Default` becomes `[Today (span 4), Articles (2), LocalEvents (2), …widgets]`.
- `heal` adds a full-width Today card at the front when the layout has none. It's the only curated card it adds back, because Today is the only one that can be missing from a layout saved before it existed.
- `parseLayout` accepts `Today` at most once, like the other curated cards.
- Frontend: `config.tsx` gains a `Today` component (`permissions: { delete: false }`) rendering `TodayBlock`, and `LayoutView` maps it.

### 7. Frontend
- **Data:**
  - `api/client.ts` gains the `TodayWidget`, `TodayEvent` and `CalendarSummary` types, and `Profile` gains `calendar`;
  - `api/hooks.ts` gains `useToday()` and `useRefreshToday()`, modeled on `useCuratedWidget`. They send `tz` from `Intl.DateTimeFormat().resolvedOptions().timeZone`, and the local date is part of the query key, so a page left open past midnight fetches the new day;
  - `useSetCalendar()` and `useRemoveCalendar()` invalidate `me` and `today`.
- **`TodayWidget.tsx`:**
  - it reuses `WidgetCard`'s shell, through a new `children` prop that replaces the archetype renderer and a footer that shows "From your {provider}" and "Updated … ago";
  - each row shows the time range (or "All day"), the title and the location;
  - ended events are dimmed (`text-ink-muted`, `aria-label` "ended");
  - the "Tomorrow" group shows from 18:00;
  - empty states: `no_calendar` gives "Add your calendar to see what's on today" (or "their") with a button to the profile panel at the `calendar` field, and `calendar_gone` gives "Your calendar link stopped working" with "Replace link";
  - the title is "Today" for your own page and "Their day" for a member's.
- **Profile panel:**
  - `ProfileField` gains `'calendar'`;
  - a `CalendarLinkField` under location shows either a URL input with a "Connect" button (separate from Save, since it checks the link) and short help on where to find the private address, or "{Provider} connected" with Replace and Remove;
  - on someone else's page, it shows "their calendar" wording, and the connected state comes from that page's profile;
  - the input uses `autoComplete="off"` and `spellCheck={false}`.

## Risks / Trade-offs

- **[Repeat-rule edge cases differ between providers]** → Fixture tests from real Google and iCloud exports. The live check uses a real calendar. Anything unhandled degrades to "event missing on that day", never a crash: a malformed event is skipped and logged without its title.
- **[A large feed is slow to fetch on every refresh]** → It's fetched at most every 15 minutes per member while a page is open, with a 5 MB cap and a 20 s budget. Most family feeds are well under 1 MB.
- **[Google's "secret address" updates with a delay]** (Google can take hours to publish changes to ICS feeds) → This is outside our control. The card's "Updated … ago" shows when we last fetched, not when Google last published, and the help text says changes can take a while to show.
- **[A blocked-address check that's too loose or too strict]** → Table tests over `netip` addresses (IPv4, IPv6, mapped, CGNAT) and an end-to-end test where a public-looking host name resolves to loopback via a test resolver. Since the check runs at dial time, redirects are covered too.
- **[Two new third-party dependencies]** → Both are small and pure Go, and their versions are pinned in `go.sum`. If golang-ical's parser fails on a feed, the error is `ErrNotCalendar`, so the card says so instead of crashing.
- **[The browser's time zone differs from where the member lives]** (a parent travelling) → It's the viewer's local day, which is what a glance should show. Noted for the briefing.
- **[The live check needs a calendar we can share]** → Use Google's public US holidays feed for all-day events, plus a throwaway Google calendar with a weekly repeat, an exception and a moved occurrence, its secret address entered by the user.

## Migration Plan

- Migration `004_calendar.sql` adds the column with a default of `''`. Old rows mean "no calendar".
- Layouts saved before this change gain the Today card when they are next loaded. No data rewrite is needed.
- Rolling back leaves an unused column and `today` rows in `curated_widget_results`, which are harmless.
- Deployment (roadmap item 7) seeds the demo members' calendar links through the same endpoints.
