## Why

Testing Porchlight end to end currently means hand-building data. `make seed` makes one parent and one kid with no widgets, no layout and no calendar. A calendar can only be tested with a private Google link pasted by hand, and only on days when that calendar happens to have events. Every page visit also runs paid Brave and OpenRouter calls. The morning-briefing check (task 6.1 of that change) is still open for exactly this reason: calendar events in a briefing can only be checked on certain evenings.

A rich, repeatable demo family fixes this. It can be rebuilt with one command and always has events today, so every card and every member's path can be checked at any hour. It costs no search credit to browse. Roadmap item 7 (public demo deployment) needs the same thing: "a demo family with a sample calendar", so this isn't throwaway work.

## What Changes

- **`make demo` builds a demo family in its own throwaway database** (`backend/porchlight.db.demo`). It deletes that file first, so every run starts from the same place. It never touches `porchlight.db`. `make seed` and its existing test accounts stay as they are.
- **The Rivera family**, all with password `password1`:
  - **Maya** (`maya@rivera.test`) is a parent who created the family. Maya has interests, a location, a calendar, three described widgets (one of them weather) and a custom page arrangement.
  - **Sam** (`sam@rivera.test`) is a parent with different interests, a location, a calendar and the default page.
  - **Jordan** (`jordan@rivera.test`) is a kid of about 15. Jordan has kid-appropriate interests, a location, a calendar and one described widget.
  - **Lily** (`lily@rivera.test`) is a kid of about 9, with interests and a location but **no calendar**. This tests the "Add your calendar" path.
  - **Grandma** (`grandma@rivera.test`) is a **pending parent invite**. This tests the invites list and accepting an invite; the seed prints the invite link.
- **A second, tiny family** (the Chens: one parent and one kid) lets you check that one family can never see another's members, pages or widgets.
- **`.test` email addresses** can never reach a real inbox, and the page greets each member by name ("Good morning, Maya").
- **Fixture calendars with no secrets.** The `PORCHLIGHT_DEV_FIXTURES=1` setting is for development only. With it on, the server itself answers calendar links on `https://fixtures.porchlight.test/calendars/<name>.ics`. Each feed is built fresh, relative to today, and always contains:
  - an event today that has already ended;
  - events later today;
  - an all-day event;
  - events tomorrow evening;
  - a repeating event with one occurrence deleted;
  - a repeating event with one occurrence moved.

  Without the setting, those links fail like any unreachable calendar, because `.test` never resolves. The real calendar fetcher and its private-address protections are unchanged for every other link.
- **Optional real calendars.** If `SEED_CALENDAR_MAYA=…`-style variables are set in `backend/.env`, `make demo` uses those links for those members instead of the fixtures. They go only into the local demo database, never into the repository.
- **Browsing the demo costs no search credit.** The seed pre-fills every member's Articles, "Happening near you" and described widgets with realistic results, kid-safe for the kids and marked as just updated. Opening any page as any member costs no Brave or OpenRouter credit until a result goes stale or someone presses refresh, as in the real app. The morning briefing still writes itself on the first visit each day (one small AI call per member). Weather and calendars are free.
- **`make demo-serve`** runs the API on the demo database with fixture calendars on.
- **The README** gets a "Demo family" section: the accounts, what each one tests, the commands, the `.env` overrides, and the rule that it's for local databases only.

**Stretch goal, not in this change:** an offline mode (`PORCHLIGHT_FAKE_AI=1`) that swaps in canned AI and search answers, for zero-cost runs and automated browser tests. It gets its own change if it's wanted before the deadline.

## Capabilities

### New Capabilities
- `demo-data`: local test data for development and demos. It covers:
  - the demo family and what each member has;
  - the reset-and-rebuild command and that it is local-only;
  - the second family used to check isolation;
  - the fixture calendar feeds and the dev setting that turns them on;
  - optional real calendar links from the environment;
  - pre-filled widget results, and when they cost credit again.

### Modified Capabilities

None. The fixture feeds exist only with the development setting on, and with it off every existing requirement holds unchanged. That includes `today-calendar`'s safe fetching and link secrecy.

## Impact

- **Backend:**
  - `cmd/seed` becomes data-driven, with a member list in Go, and gains a demo mode. Item 6 (chore checklist) can add each member's chores there later.
  - `internal/calendar` gets a fixture feed generator and a fetcher wrapper used only when the dev setting is on.
  - `internal/widgets` gets small exported pre-fill helpers, so the format of remembered results stays owned by that package.
  - `main.go` reads `PORCHLIGHT_DEV_FIXTURES`, and the `Makefile` gets `demo` and `demo-serve`.
- **No schema migration, no new API routes, no frontend changes, and no new dependencies.**
- **Docs:**
  - `README.md` gets the new section.
  - `backend/.env.example` gets commented-out `PORCHLIGHT_DEV_FIXTURES` and `SEED_CALENDAR_*` lines.
  - On archive, one line goes in "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
- **Unblocks** the open morning-briefing task 6.1 (calendar events in a live briefing), which can now be checked at any hour.
