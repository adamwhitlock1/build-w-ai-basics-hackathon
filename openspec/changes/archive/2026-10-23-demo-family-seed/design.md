## Context

See `proposal.md` for why, and `specs/demo-data/spec.md` for the required behavior. These facts about the current code shape the approach:

- **`cmd/seed`** creates one parent and one kid through the same store calls the app uses (`CreateFamilyWithAdmin`, `CompleteFamilySetup`, `InviteToFamily`, `AcceptInvite`). It skips itself if the parent already exists. It writes to `porchlight.db` or `-db <file>`.
- **The Today handler fetches feeds through an interface.** `widgets.CalendarFetcher` has one method: `Fetch(ctx, link) ([]byte, error)`. `main.go` passes `calendar.NewFetcher()` into `routes(...)`. Saving a link (`SetCalendar`) and reading Today both go through this one fetcher, and so does the briefing, because it reads Today's remembered result.
- **Remembered results** live in `curated_widget_results`, as `(user_id, kind, inputs, payload, updated_at)`:
  - Articles match on `inputs = normalizeInput(interests)`.
  - "Happening near you" matches on `normalizeInput(interests) + "\n" + normalizeInput(location)`.
  - A result is reused only if `widget.KidSafe == (audienceFor(member) == Kids)`.

  `normalizeInput`, `audienceFor` and the curated widgets' title, source and archetype all live unexported in `internal/widgets`.
- **Described widgets** store a `render_spec` JSON. Its type, `renderSpec`, is unexported in `internal/widgets`. Their freshness is `refreshed_at`, or `created_at` if never refreshed. Refreshing one runs its search, then one AI "shape items" call. That's true even for weather, where the search itself (Open-Meteo) is free.
- **Layouts** are Puck JSON (`layout.Data`, with blocks `Briefing`, `Today`, `Articles`, `LocalEvents` and `Custom` plus `widgetId`). A member with no saved layout gets the default page.
- **The briefing** remembers a fingerprint of its facts, including the live Open-Meteo forecast and the viewer's date and time zone. A pre-filled briefing would never match that fingerprint and would be rewritten on first visit anyway.
- **Ignored files:** `.gitignore` already covers `backend/porchlight.db*`, so `porchlight.db.demo` is ignored with no change.

## Goals / Non-Goals

**Goals:**
- One Go file declares the demo families as data, so adding a member, or later a member's chores (roadmap item 6), is a matter of editing a list.
- The seed writes remembered results through helpers in `internal/widgets`. The format of what's remembered stays owned by the package that reads it, so a later format change breaks a test, not the demo silently.
- Fixture calendars plug in at the existing `CalendarFetcher` seam. There's no change to `calendar.Fetcher`, the Today handler or the routes.

**Non-Goals:**
- No offline or fake-AI mode in this change (`PORCHLIGHT_FAKE_AI`). It's noted as a stretch goal and would get its own change.
- No pre-filled briefing or Today result (see the spec's pre-fill requirement, and Decision 5).
- No deployed demo, rate limiting or chores. Those are roadmap items 6 and 7, which will reuse this seed.
- No frontend changes.

## Decisions

### 1. One seed command, with a demo mode
`cmd/seed` gains `-demo`. Without it, the command runs today's behavior, with the existing accounts. With it:

- the default `-db` becomes `porchlight.db.demo`;
- it seeds the families declared in a new `cmd/seed/demo.go`.

The Makefile adds:
```make
demo:        go run ./cmd/seed -demo
demo-serve:  PORCHLIGHT_DEV_FIXTURES=1 DB_PATH=porchlight.db.demo go run .
```

**Reset safety:**
- In demo mode, the seed deletes the target file (and its `-wal` and `-shm` side files) only if the file name ends in `.demo`.
- For any other path, it opens the file, and refuses with an error if the `users` table has any rows.
- This makes "run it against my real DB by mistake" fail loudly.

*Alternative:* a separate `cmd/demo` command. Rejected: the two would duplicate the token and password helpers and the "go through the real store calls" approach. One command with a mode keeps `make seed` unchanged and gives item 7 one place to extend.

### 2. Families as data
The families are declared as Go values: a `demoFamily` holds a name and `[]demoMember`. Each member has:

- an email and whether they're a parent;
- interests and a location;
- a calendar fixture name, or none;
- described widgets: each has a title, source, query, prompt, archetype and items;
- an optional layout;
- canned Articles and Events items.

A family can also hold pending invites. The seed loop creates each member the way the app does:

- the creator, through `CreateFamilyWithAdmin` and `CompleteFamilySetup`;
- everyone else, through `InviteToFamily` and `AcceptInvite`, so `is_admin` comes from the invite, exactly as in the app;
- then profiles, calendar links, widgets, results and layouts.

**Grandma's invite** is created and not accepted. It's a parent invite (`is_admin`), because in Porchlight anyone who isn't a parent is treated as a kid for content, which would be wrong for Grandma. Its expiry is set 30 days out, so the printed link keeps working across a month of testing.

**The second family** is the Chens (`alex@chen.test`, a parent with one described widget, and `riley@chen.test`, a kid). It's declared the same way.

*Alternative:* a JSON or YAML fixture file. Rejected: Go values get compile-time checking against the store and widget types, the user is learning Go, and there's no need to edit the data without rebuilding.

### 3. Pre-fill helpers in `internal/widgets`
These are new exported functions in a small `prefill.go`:

- `PrefillArticles(ctx, results ResultStore, member store.User, profile store.MemberProfile, items []llm.Item, at time.Time) error`
- `PrefillEvents(...)`, with the same shape.
- `NewCustomWidget(member store.User, title, source, query, prompt string, archetype llm.Archetype, items []llm.Item) (store.CustomWidget, error)`. It returns a row, with its `render_spec` encoded, ready for `store.CreateCustomWidget`.

Each one builds its value with the same unexported pieces the handlers use:

- `normalizeInput` for the inputs;
- `audienceFor` for `KidSafe`;
- the curated widgets' title, source and archetype.

That last part means pulling the title, source and archetype that `Articles` and `Events` currently write inline into two small constructors, which the handlers and the helpers both call.

A test seeds a member with each helper, then calls the real `Articles`, `Events` and `GetCustom` handlers with a curator and sources that fail the test if called. This proves a pre-filled result is actually served without spending credit, which is the property the spec cares about.

*Alternative:* the seed writes the JSON itself. Rejected: it would copy the matching rules (normalization, kid-safe flag) into a second place, and a drift would make the app quietly re-search, which costs credit and is exactly what this avoids.

### 4. Fixture calendars as a fetcher wrapper
In `internal/calendar`, add `fixtures.go`:

- `const FixtureHost = "fixtures.porchlight.test"`.
- `NewFixtureFetcher(next interface{ Fetch(...) }, now func() time.Time) *FixtureFetcher`. Its `Fetch` does this:
  - It runs `NormalizeLink` first, so `webcal://` and odd input behave as usual.
  - If the host is exactly `FixtureHost` and the path is `/calendars/<name>.ics`, it returns the generated feed, or `ErrLinkGone` for an unknown name. There's no network.
  - Anything else goes to `next.Fetch` unchanged, so the real fetcher's private-address checks still apply.
- `main.go` wraps `calendar.NewFetcher()` with this only when `PORCHLIGHT_DEV_FIXTURES=1`, and then logs `slog.Warn("dev fixture calendars are on; never enable this in a deployment")`.
- With the setting off, nothing changes. A saved fixture link reaches the real fetcher, DNS fails for `.test`, and Today shows "calendar unavailable".

**Feed generation:** `fixtureFeed(name, now)` writes an iCalendar document in code, with CRLF line endings and `PRODID`/`VERSION`, from a per-name list of events built relative to `now`:

- **Floating times** (no `TZID`, no `Z`). The parser already treats these as the same clock time in the viewer's zone (see `testdata/floating.ics`), so a feed made for "today" reads correctly whichever zone the browser sends. "Today" is the server's local date. In local development the server and browser are on the same machine.
- **Already ended:** from `q-90m` to `q-30m`, where `q` is `now` rounded down to the quarter hour, so times read like real plans ("3:30", not "3:23").
- **Later today:** starts at `q+90m`.

  Both are kept within today. Between about 23:00 and 01:00, one of them may be skipped (see Risks).
- **All-day:** `DTSTART;VALUE=DATE` today.
- **Tomorrow evening:** 18:30 and 19:30 tomorrow, so the after-6-pm "Tomorrow" view has content.
- **Deleted occurrence:** Jordan's "Band practice" is a daily `RRULE` at 16:00, starting 7 days ago, with `EXDATE` set to today's occurrence. It's missing today and present tomorrow.
- **Moved occurrence:** Sam's "Team standup" is a daily `RRULE` at 09:00, with a second `VEVENT` that has `RECURRENCE-ID` set to today 09:00 and `DTSTART` today 11:00. It shows at 11:00 today and 09:00 tomorrow.

**Feeds:**

| Feed | Contents |
|---|---|
| `maya` | ended, later today, all-day ("Recycling day"), tomorrow evening |
| `sam` | the moved standup, a later-today event |
| `jordan` | the deleted band practice, an all-day event tomorrow, tomorrow evening |
| `alex` | one simple event, for the second family |

Each feed has a stable `UID` per event, so repeated fetches describe the same events.

*Alternatives:*
- Static `.ics` files: rejected, because they'd go stale the next day.
- A local HTTP server on 127.0.0.1: rejected, because the real fetcher correctly refuses loopback addresses, and loosening that for development is the kind of switch that leaks into production.
- Intercepting in the Today handler: rejected, because that's more code paths, and `SetCalendar` validation would also need it.

### 5. What is and isn't pre-filled
**Pre-filled**, all with `updated_at` / `created_at` equal to seed time:
- Articles and Events for every member with interests (and a location, for Events);
- every described widget's items.

The canned items are hand-written and plausible. Their URLs point to real, stable home pages (for example a team's or museum's site), never to made-up article paths that would 404. Kids' items are written to be kid-appropriate.

**Not pre-filled:**
- **Today:** it's free, and must reflect the generated feed.
- **The briefing:** see Context. Its fingerprint includes the live forecast and the viewer's date, so it costs one small AI call per member per day regardless.

This matches the spec's "no paid search or curating AI call while fresh". After the windows pass, normal behavior resumes: Articles at 3 h, Events at 12 h, described widgets at 6 h, weather at 1 h. The README says to re-run `make demo` to get back to zero-cost browsing.

### 6. Calendar overrides from the environment
In demo mode, the seed loads `backend/.env` with `godotenv` (already a dependency). The shell's variables win, as in `main.go`. For each member with a fixture calendar, it reads `SEED_CALENDAR_<UPPERCASE FIRST NAME>`. A non-empty value replaces the fixture link:

- The value goes through `calendar.NormalizeLink`, so a pasted `webcal://` still works.
- It is saved with `SetCalendarLink`.
- The output says only `Maya: calendar from SEED_CALENDAR_MAYA`, never the value.

The seed doesn't fetch the link. A broken one shows up on the Today card as it would for any member. Members without a calendar in the demo ignore any override.

### 7. Output
The seed prints:

- a table of account, role, what it tests and calendar (`fixture maya`, `from SEED_CALENDAR_MAYA`, or `none`);
- the shared password;
- the invite URL `http://localhost:<port>/invite/<token>`. The port comes from `PORCHLIGHT_WEB_PORT`, the same variable `frontend/vite.config.ts` reads, and defaults to its 5174.

## Risks / Trade-offs

- **[Risk] Someone turns on `PORCHLIGHT_DEV_FIXTURES` in a deployment.** → Every member could then point their calendar at canned data, but nothing leaks and no network access is gained: the wrapper only adds answers for one fake host. The startup warning makes it visible, and roadmap item 7's deploy config simply won't set it. The worst case is fake events, not a security hole.
- **[Risk] The fixture window near midnight.** Between about 23:00 and 01:00, the "already ended" or "later today" event may not fit inside today and is skipped. → This is documented in the README. Testing at those hours is rare, and the rest of the feed still works.
- **[Risk] Server and browser in different time zones**, for example a remote dev box. → Floating times keep clock times right, but "today" is the server's date. This is documented, and it only matters around midnight.
- **[Trade-off] Pre-filled items are fake.** Links go to real home pages, but the "articles" aren't real articles. → That's acceptable for testing. For the public demo (item 7), a real refresh just before recording or judging replaces them.
- **[Trade-off] Moving the curated widgets' title, source and archetype into constructors** touches the Articles and Events handlers slightly. → Existing handler tests cover them. It's a pure extraction with no behavior change.
- **[Risk] The refuse-if-users guard is bypassed by naming a real DB `something.demo`.** → That's deliberate naming by the developer. The README says demo files are disposable.

## Migration Plan

- There's no schema change.
- `make seed` is unchanged.
- Rollback is deleting the new files and Makefile targets.
- `porchlight.db.demo` is already git-ignored.
