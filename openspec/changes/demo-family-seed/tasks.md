## 1. Fixture calendars (`internal/calendar`)

- [x] 1.1 Add `fixtures.go` with `FixtureHost`, and an unexported `fixtureFeed(name string, now time.Time) ([]byte, bool)` that builds the `maya`, `sam`, `jordan` and `alex` feeds in code, with floating times, CRLF line endings and stable UIDs (design decision 4). Table-test it by parsing each feed with the package's own event-listing code at a fixed `now` (for example 14:00 on a Wednesday) and checking that:
  - Maya's feed has the ended event, the later-today event and the all-day event today, and two events tomorrow evening;
  - Sam's standup is at 11:00 today and 09:00 tomorrow;
  - Jordan's band practice is absent today and at 16:00 tomorrow;
  - an unknown name returns `false`.

  Verify with `go test ./internal/calendar/`.
- [x] 1.2 Add a test of `fixtureFeed` at 00:20 and at 23:40. Each feed must still parse, and must have no event outside today or tomorrow. Verify with `go test ./internal/calendar/`.
- [x] 1.3 Add `FixtureFetcher` and `NewFixtureFetcher(next, now)`. Its `Fetch` normalizes the link, serves `https://fixtures.porchlight.test/calendars/<name>.ics` (and its `webcal://` form) from `fixtureFeed`, returns `ErrLinkGone` for an unknown name or another path on that host, and passes every other link to `next` unchanged. Test with a recording fake `next` that:
  - a fixture link never reaches `next`;
  - `https://calendar.google.com/...` and `https://127.0.0.1/x.ics` reach `next` exactly as given;
  - an unknown fixture gives `ErrLinkGone`;
  - no returned error contains the link.

  Verify with `go test ./internal/calendar/`.

## 2. Server setting (`main.go`)

- [x] 2.1 Read `PORCHLIGHT_DEV_FIXTURES` in `loadConfig` (only the value `1` turns it on). In `run`, wrap `calendar.NewFetcher()` with `calendar.NewFixtureFetcher` when it's on, and log the startup warning. Verify with `go vet ./... && go build ./...`, and by starting the server with and without the setting and checking that the warning appears only with it.
- [x] 2.2 Add an integration test in `routes_test.go` that wires `routes(...)` with a `FixtureFetcher` around a fake that fails if called. As a seeded member, `PUT /api/me/calendar` with the Maya fixture link must succeed and report `connected: true` with provider `fixtures.porchlight.test`, and `GET /api/widgets/today` must list Maya's fixture events. Verify with `go test ./...`.

## 3. Pre-fill helpers (`internal/widgets`)

- [x] 3.1 Pull the inline title, source and archetype of the Articles and Events widgets into two small unexported constructors, used by both handlers (design decision 3). This must not change behavior. Verify with `go test ./internal/widgets/`: all existing curated tests stay green.
- [x] 3.2 Add `prefill.go` with `PrefillArticles`, `PrefillEvents` and `NewCustomWidget`. They use `normalizeInput`, `audienceFor` and `renderSpec`, so the matching rules exist in one place. Doc comments say they're for seeding local data.
- [x] 3.3 Test that each pre-filled result is actually served without spending credit:
  - Pre-fill a parent and a kid with `PrefillArticles` and `PrefillEvents`. Their `Articles` and `Events` handlers must return those items, not stale, with `kidSafe` matching the member, using a curator and sources that call `t.Fatal` if used.
  - Save a widget from `NewCustomWidget` through the fake widget store. `GetCustom` must return its items.

  Verify with `go test ./internal/widgets/`.

## 4. Demo seed (`cmd/seed`)

- [x] 4.1 Restructure `cmd/seed/main.go` so the existing test family still seeds by default, exactly as today, and add a `-demo` flag whose default `-db` is `porchlight.db.demo`. Verify that `make seed` on a fresh file still prints and creates the two existing accounts.
- [x] 4.2 Add the reset guard (design decision 1):
  - In demo mode, delete the target file and its `-wal`/`-shm` files only when its name ends in `.demo`.
  - Otherwise, refuse if it already has users.

  Verify by hand:
  - `go run ./cmd/seed -demo -db porchlight.db` against a seeded `porchlight.db` exits non-zero and leaves the file's size and modified time unchanged;
  - two runs of `make demo` both succeed.
- [x] 4.3 Add `cmd/seed/demo.go` with the Rivera and Chen families as data (design decision 2):
  - each member's interests, location and calendar fixture;
  - described widgets: Maya gets three, including a `weather_forecast` one; Jordan gets one kid-appropriate one; Alex gets one;
  - canned Articles and Events items (kid-appropriate for Jordan, Lily and Riley; URLs only to real, stable home pages);
  - Maya's custom layout, built from `layout.Data` with every built-in card and Maya's three widgets in a non-default order and spans;
  - Grandma's pending parent invite, expiring in 30 days.
- [x] 4.4 Implement the demo seed loop: create the families through the store calls the app uses, then set profiles, calendar links, widgets (via `widgets.NewCustomWidget` and `CreateCustomWidget`), pre-filled results and layouts.
  - Calendar links: load `backend/.env` and apply any non-empty `SEED_CALENDAR_<NAME>` override (normalized, never printed).
  - Output: print the accounts table, the password and the invite URL (using `PORCHLIGHT_WEB_PORT`, default 5174).

  Verify by running `make demo` with no overrides. The output must list Maya, Sam, Jordan, Lily, Alex and Riley as accounts and Grandma as invited, and `sqlite3 porchlight.db.demo 'select count(*) from users'` must print 6.
- [x] 4.5 Add a test in `cmd/seed` that runs the demo seed into a temporary file and checks:
  - six users in two families, with the right parent and kid roles;
  - Lily has no calendar link, and everyone else has a fixture link;
  - one unaccepted parent invite;
  - Maya's layout parses and lists Maya's three widget ids;
  - every member with interests has Articles and Events results;
  - with `SEED_CALENDAR_MAYA` set via `t.Setenv`, Maya's link is that value, the other members keep their fixture links, and the value never appears in the output.

  Verify with `go test ./cmd/seed/`.

## 5. Makefile and docs

- [x] 5.1 Add `demo` and `demo-serve` targets to `backend/Makefile`, with a comment in the same style as `seed`. Verify that `make demo && make demo-serve` starts the API on the demo database with the fixtures warning shown.
- [x] 5.2 Add commented-out `PORCHLIGHT_DEV_FIXTURES=1` and `SEED_CALENDAR_MAYA=` / `SAM` / `JORDAN` / `ALEX` lines to `backend/.env.example`, saying they are for local development only. Verify by reading the file.
- [x] 5.3 Add a "Demo family" section to `README.md`, next to "Local test accounts". It covers:
  - the accounts table and what each one tests;
  - `make demo` / `make demo-serve` and the frontend command;
  - the fixture calendars and what they contain;
  - the `.env` overrides;
  - when browsing starts costing credit again, and "re-run `make demo`";
  - the midnight and time-zone caveats;
  - "local databases only".

  Verify by following the section from a clean checkout.

## 6. End-to-end check

- [x] 6.1 With `make demo`, `make demo-serve` and the frontend running, log in as each account. Confirm with the API log that no Brave search or curating AI call runs on first page loads, and check:
  - Maya: the custom arrangement; Today shows the ended event dimmed, the all-day event first and later events; the briefing's sources include "calendar".
  - Sam: the default page; the standup at 11:00, not 09:00.
  - Jordan: kid-safe cards; no band practice today.
  - Lily: "Add your calendar". Paste `https://fixtures.porchlight.test/calendars/maya.ics` and see events appear.
  - Grandma: open the printed invite link, join, and see the other members' pages.
  - Alex (Chen family): requesting `/api/members/<Maya's id>/homepage` returns 404.

  After 6 pm (or by temporarily faking the clock), Today shows a "Tomorrow" section. Record the results in this task.

  **Notes (2026-10-14, about 13:50–14:00 Denver):** done with `make demo` and `PORT=8094 make demo-serve` through the API with curl, then in the browser (`PORCHLIGHT_API=http://127.0.0.1:8094 PORCHLIGHT_WEB_PORT=5182 npx vite`).
  - **Maya:**
    - The custom arrangement: Briefing 4/4; Today and Denver Weather side by side; Articles and Launches; Events and Dinner Ideas.
    - Today listed "Recycling day" (all day) first, then "Coffee with Priya" (ended, dimmed), then "Dentist appointment".
    - Tomorrow had Book club 18:30 and PTA 19:30.
    - The briefing mentioned the dentist time and recycling day, and its `uses` included `calendar`.
    - All three widgets were served fresh, not kid-safe.
  - **Sam:** the default page. Standup at 11:00 today and 09:00 tomorrow.
  - **Jordan:** Articles, Events and the robotics widget were all `kidSafe: true`. No band practice today; band practice at 16:00 tomorrow.
  - **Lily:** "Add their calendar to see what's on their day." (seen from Maya's sidebar), and a kid-safe briefing. Pasting the Maya fixture link was accepted ("fixtures.porchlight.test" connected), and Today then listed Maya's events.
  - **Grandma:** the invite was accepted (201). Grandma joined as a parent, the pending list emptied, and Grandma could open Jordan's page (200).
  - **Alex (Chen family):** Maya's member homepage, Today and Articles, Maya's widget and Maya's calendar `PUT` all returned 404.
  - **No paid calls:** the API log had no `articles curated`, `events curated`, `widget planned` or `widget refreshed`. The only AI call was `briefing written`, once per member viewed. No fixture link appears in the log.
  - **Evening view:** the 6 pm "Tomorrow" heading itself isn't checked in a browser (it was early afternoon). The API returns tomorrow's events, and when to show them is existing, unchanged frontend behavior.
  - **Changed during the check:**
    - Maya's weather card was cramped at 1/4 width, so the layout now gives Today and weather 2/4 each.
    - Clock-relative fixture events now start on quarter hours ("3:45", not "3:23"), with a test, and the design note is updated.
- [x] 6.2 Restart the server without `PORCHLIGHT_DEV_FIXTURES` and confirm that Maya's Today card reports the calendar as unavailable and the log contains no fixture link. Then run `go vet ./... && go test ./...` in `backend/`.

  **Notes:** after a fresh `make demo`, with fixtures off, Maya's Today returned 502 "Couldn't load the calendar right now." The log said only `provider=fixtures.porchlight.test err="calendar unavailable: dial tcp: lookup fixtures.porchlight.test: i/o timeout"`: the host, never the path. There was no fixtures warning at startup. `make check` passed (vet, all tests, build).
- [x] 6.3 Use the demo to finish morning-briefing task 6.1 (calendar events in a live briefing) in `openspec/changes/archive/2026-10-13-morning-briefing/tasks.md`, noting that the check used fixture calendars.
