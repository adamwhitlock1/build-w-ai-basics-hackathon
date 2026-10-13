## 1. Calendar package (`internal/calendar`)

- [x] 1.1 Add `github.com/arran4/golang-ical` v0.3.7 and `github.com/teambition/rrule-go` v1.8.2 to `backend/go.mod` (`CGO_ENABLED=0 GOTOOLCHAIN=local go get …`). Verify that `go build ./...` passes and that `go.mod` still says `go 1.22.0`.
- [x] 1.2 Add `NormalizeLink` (`webcal://` → `https://`; `https` only; trimmed; at most 2,000 characters) and `Provider` (Google Calendar, iCloud, Outlook, else the host), plus the sentinel errors `ErrNotCalendar`, `ErrLinkGone`, `ErrUnavailable` and `ErrBlockedAddress`. Verify with table tests and `go test ./internal/calendar/`.
- [x] 1.3 Add `Fetcher` with the guarded dialer, redirect limit, 10 s timeout and 5 MB cap from design decision 3. Its address check is a field that `export_test.go` can widen for loopback test servers. Tests must cover:
  - a table of blocked and allowed `netip` addresses (IPv4, IPv6, IPv4-mapped, CGNAT `100.64.0.0/10`, `169.254.169.254`);
  - an `httptest.NewTLSServer` feed that is fetched with the check widened, and refused with `ErrBlockedAddress` without it;
  - a redirect to `http://` that is refused;
  - more than 3 redirects, refused;
  - a body over 5 MB, refused;
  - 404 and 410 giving `ErrLinkGone`, and 500 giving `ErrUnavailable`;
  - no error message ever containing the link.

  Verify with `go test ./internal/calendar/`.
- [x] 1.4 Add the fixture feeds to `internal/calendar/testdata/` (as listed in design decision 2): Google-style, iCloud-style and floating-time. Write them by hand from real exported structure, with no personal data. Verify that each starts with `BEGIN:VCALENDAR` and parses with golang-ical in a smoke test.
- [x] 1.5 Implement `Between(feed, from, to, loc)`: single, all-day, multi-day all-day, `TZID`, UTC and floating events; `RRULE` with `RDATE` and `EXDATE`; `RECURRENCE-ID` overrides, including cancelled ones; `STATUS:CANCELLED` skipped; an unknown `TZID` falling back to `loc`; a malformed event skipped, not fatal; sorting all-day first, then by start, then by title. Test each spec scenario against the fixtures with a fixed window:
  - Tuesday/Thursday practice shows on Thursday;
  - the excluded Thursday doesn't;
  - the moved occurrence shows at 7 pm only;
  - spring break shows on Wednesday;
  - a 9 am New York event shows at 7 am Denver;
  - a weekly 9 am event stays at 9 am on 2026-11-02, after daylight saving ends.

  Verify with `go test ./internal/calendar/`.

## 2. Storage and calendar endpoints

- [x] 2.1 Add migration `backend/migrations/004_calendar.sql` (`member_profiles.calendar_url TEXT NOT NULL DEFAULT ''`). Add the store methods `GetCalendarLink`, `SetCalendarLink` (an upsert that leaves interests and location alone) and `DeleteCuratedResult`. Tests in `internal/store` must check:
  - setting a link keeps interests and location;
  - `UpsertMemberProfile` keeps the link;
  - deleting a result removes only that kind.

  Verify with `go test ./internal/store/`.
- [x] 2.2 Add `widgets.TodayHandler` with `SetCalendar` and `RemoveCalendar` (design decision 4), behind small interfaces for the fetcher and store so tests use fakes. Add the routes in `routes.go`: `PUT`/`DELETE /api/me/calendar` (private) and `PUT`/`DELETE /api/members/{id}/calendar` (`asParent`). Tests must cover:
  - a valid link is saved, the remembered `today` result is discarded, and the response is `{calendar: {connected: true, provider: "Google Calendar"}}` with no link in the body;
  - `webcal://` is normalized;
  - a not-calendar link gives 422 and keeps the old link;
  - a blocked address gives 422;
  - a parent can set a kid's link;
  - a kid gets 403 on another member's route (in `members_test.go`);
  - DELETE clears the link and the result;
  - a test log handler shows the link is never logged.

  Verify with `go test ./...`.
- [x] 2.3 Add `calendar: {connected, provider}` to `MeProfile` and fill it in `family.writeMe`. Update the frontend-facing tests in `routes_test.go`/`members_test.go` to expect it. Verify with `go test ./...` and that `GET /api/me` never includes the link.

## 3. The Today endpoint

- [x] 3.1 Implement `TodayHandler.Today` for `GET /api/widgets/today` and `GET /api/members/{id}/widgets/today` (`asParent`), as in design decision 5:
  - `tz` checking with a UTC fallback;
  - today and tomorrow windows;
  - 12-event caps with `more…` counts;
  - remembered results under kind `today` with hashed inputs;
  - `TodayFreshFor = 15 * time.Minute`;
  - `?refresh=1` with the 1-minute guard;
  - `no_calendar` and `calendar_gone` empty reasons;
  - 502 on other failures;
  - 20 s budget.

  Import `_ "time/tzdata"` in `main.go`, and wire `calendar.NewFetcher()` and the handler in `main.go`/`routes.go`. Tests with a fake fetcher returning fixture feeds must cover:
  - events for "today" and "tomorrow" in the given `tz`;
  - a second request within 15 minutes not fetching, and one after 15 minutes coming back stale;
  - a new `date` or a new link causing a fetch;
  - `?refresh=1`;
  - no calendar giving `no_calendar` and nothing remembered;
  - `ErrLinkGone` giving `calendar_gone` and the remembered result deleted;
  - `ErrUnavailable` giving 502;
  - a parent viewing a kid getting the kid's events;
  - the remembered inputs never containing the link.

  Verify with `go test ./...`.
- [x] 3.2 Extend `freshness_internal_test.go`: Today fetched 20 minutes ago is stale, and 10 minutes ago is not. Verify with `go test ./internal/widgets/`.

## 4. Layout

- [x] 4.1 Add `TypeToday` ("Today", id "today"). `Default` becomes Today (span 4), then Articles, then Local Events. `heal` puts a full-width Today card at the front when it's missing, and `parseLayout` accepts it at most once. Extend `data_test.go`:
  - the default layout order;
  - an old saved layout gaining Today at the top with the rest unchanged;
  - two Today cards collapsing to one;
  - a saved layout that moved or resized Today keeping that.

  Verify with `go test ./internal/layout/` and `go test ./...` (homepage tests).

## 5. Frontend

- [x] 5.1 Add the `TodayWidget`, `TodayEvent` and `CalendarSummary` types, plus `Profile.calendar`, to `api/client.ts`. Add `useToday`, `useRefreshToday`, `useSetCalendar` and `useRemoveCalendar` to `api/hooks.ts`, sending the browser `tz` and keying on the local date. Verify that `npm run build` passes.
- [x] 5.2 Give `WidgetCard` a `children` body override and a footer note. Add `TodayWidget.tsx`:
  - rows with a time range or "All day", title and location;
  - ended events dimmed;
  - a "Tomorrow" group from 18:00;
  - "+N more";
  - empty, `no_calendar` and `calendar_gone` states, with buttons that open the profile panel at `calendar`;
  - loading, "Try again" and refresh, as on `CuratedWidgetCard`;
  - "Their day" wording on a member's page.

  Add the `Today` Puck component (`permissions: { delete: false }`), its block, and the `LayoutView` mapping. Verify that `npm run build` and `npm run lint` pass.
- [x] 5.3 Add `'calendar'` to `ProfileField`, and a `CalendarLinkField` in the profile panel:
  - a URL input with "Connect" (its own request, showing the server's 422 message inline);
  - help on finding Google's "Secret address in iCal format" and iCloud's public calendar link;
  - a "{Provider} connected" state with Replace and Remove;
  - "their" wording on a member's page, where the connected state comes from that page's profile;
  - `autoComplete="off"` and `spellCheck={false}`.

  Verify that `npm run build` and `npm run lint` pass.

## 6. Verify in the running app

- [x] 6.1 On a throwaway database with a parent and a kid:
  - connect Google's public US holidays feed (`https://calendar.google.com/calendar/ical/en.usa%23holiday%40group.v.calendar.google.com/public/basic.ics`) and check the card and "Google Calendar connected" in the profile panel;
  - ask the user to create a throwaway Google calendar with a weekly repeating event, one deleted occurrence and one moved occurrence, then paste its secret address, and check the card on the affected days (using `tz` and seeded remembered results where needed);
  - check the page at 7 pm shows "Tomorrow";
  - check a pasted web page and `https://127.0.0.1/` are refused with the right messages;
  - check that removing the calendar shows the "Add your calendar" state;
  - as the parent, connect the kid's calendar from the kid's page.

  Grep the backend log for the calendar link (it must not appear). Record what you saw in this task's notes.

  **Notes (2026-10-12):** done on a throwaway database (`backend/porchlight.db.calendar-check`, port 8091) with a parent and a kid. Repeating events were then also checked live, as below.
  - **Connecting a real Google feed:** Google's public US holidays feed connected as the parent. The card showed "Columbus Day · All day" (2026-10-12), "From your Google Calendar" and "Updated just now". The profile panel showed "Google Calendar connected" with Replace and Remove, and `/api/me` held only `{connected, provider}`.
  - **Refused links:** a web page (422 "didn't give us a calendar"), `https://127.0.0.1/…`, `https://localhost:8091/…`, `webcal://169.254.169.254/…` (422 "points somewhere Porchlight can't reach") and `http://…` (422 "doesn't look like a calendar link"). The saved link stayed after each.
  - **Permissions:** the kid got 403 setting the parent's calendar.
  - **Parent connects a kid's calendar:** done in the UI from the kid's page with a `webcal://` link. "Add their calendar" opened the panel with the cursor in the link box, and "Their day" filled in with no Kid-safe marker.
  - **Removing a calendar:** "Add your calendar" came back.
  - **Evening view:** with the page's clock set to 19:05, the card added "TOMORROW · Nothing on the calendar tomorrow."
  - **Logs:** the backend log holds no part of any calendar link, only service or host names of refused links.
  - **A real Google calendar with repeats:** the user made a throwaway calendar with "Soccer Practice" every Tue and Thu at 18:00–19:30, deleted the Oct 15 occurrence, and moved the Oct 22 one to 19:00. They connected it themselves as the seeded parent (`make seed`).
    - A temporary checker (`backend/cmd/calcheck`, not committed) fetched it with the real `Fetcher` and ran `calendar.Between` for each day in America/Denver: Oct 13, 20, 27 and 29 showed 18:00–19:30; Oct 15 showed no practice; Oct 22 showed 19:00–20:30 only.
    - The calendar's other repeating event (Tue/Thu 09:00) and its one-off events showed on their own days.
    - The live `GET /api/widgets/today?refresh=1` matched: "Make something cool" 15:00 today, and tomorrow (Oct 13) "Review Devpost Hackathons" 09:00 and "Soccer Practice" 18:00.
  - **Fixed during the check:** the focus for "Add … calendar" landed on Cancel because `section button` matched the panel itself. It now targets `[data-calendar-field]`.

## 7. Docs and wrap-up

- [x] 7.1 Add a short "Calendars" note to `README.md`: where to find the private iCal link in Google and iCloud, that it's kept secret on the server, and that Google can take a few hours to publish changes. Verify by reading the rendered README.
- [x] 7.2 After archiving, add one line about the Today card to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`. Verify with `git diff devpost/` that only those two lines were added.
