## Why

A family's morning starts with "what's on today?", and Porchlight can't answer it: there's nowhere to see anyone's plans. The scope cut the unified family calendar as too big. Every family calendar app, though, already offers a private iCal (ICS) link, and reading one gives each member a "Today" card without building a calendar. This is the strongest family-impact item on the roadmap, and it gives the morning briefing (roadmap item 5) the day's plans to talk about.

## What Changes

- **A new built-in "Today" card on every member's page** alongside Articles and "Happening near you". It lists the day's events from the member's calendar, in the member's time zone:
  - repeating events show on the right days, with skipped and moved occurrences honored;
  - all-day events (including ones spanning several days) are listed first;
  - events that have ended are dimmed;
  - from 6 pm the card also shows tomorrow's events, so it's useful the evening before.
- **Members connect a calendar by pasting its private iCal link** in the profile panel ("Your interests"). A parent can paste one for a family member, as they already set that member's interests. One calendar per member.
- **The link is treated as a secret:**
  - it's stored only on the server;
  - the browser only ever learns *that* a calendar is connected and which service it's from (e.g. "Google Calendar");
  - it's never shown to anyone in full, never logged, and never sent to the AI.

  Members (and parents, for their family) can replace or remove it.
- **Saving checks the link** by fetching it once. A link that isn't a calendar, or can't be reached, is refused with a plain message.
- **The server fetches calendar links safely:**
  - `https` only (`webcal://` links are accepted and treated as `https`);
  - never to private, loopback or link-local network addresses, including through redirects or DNS tricks;
  - with a size cap and a timeout.

  This matters once Porchlight is public (roadmap item 7).
- **Today's events are remembered for 15 minutes** and then refresh in the background, like the other cards. The card has a refresh button and shows "Updated … ago". Fetching a calendar costs no credit.
- **Clear empty and error states:**
  - "Add your calendar" (or "Add their calendar") when none is connected;
  - "Nothing on the calendar today";
  - a "Try again" when the fetch fails;
  - "This calendar link stopped working" when the link has been revoked.
- **The card is permanent**, like Articles and Local Events: pages arranged before this change get it at the top.

## Capabilities

### New Capabilities
- `today-calendar`: connecting a member's calendar link (who can set, replace or remove it, how it's checked, and how it's kept secret); what the Today card shows (today's and tomorrow's events, repeating and all-day events, time zone); how calendar fetches are kept safe; and the card's empty, error and refresh behavior.

### Modified Capabilities
- `widget-freshness`: the "Freshness windows" requirement gains the Today card's 15-minute window. The remembered-results, background-refresh, refresh-on-request and "Updated … ago" requirements already cover "every widget" and "each curated widget card", so they apply to the Today card without changing.

## Impact

- **Backend (Go):**
  - a new `internal/calendar` package that fetches, parses and expands ICS feeds;
  - two new dependencies: `github.com/arran4/golang-ical` (parsing) and `github.com/teambition/rrule-go` (repeat rules);
  - `time/tzdata` embedded, so time zones work on any host;
  - a safe HTTP client for fetching user-supplied links;
  - the profile store and `family` handlers gain the calendar link;
  - a new `Today` handler beside the curated ones, remembered in `curated_widget_results` as kind `today`;
  - `layout` gains the `Today` card type, and `heal` adds it to layouts that lack it.
- **API:**
  - `GET /api/widgets/today` and `GET /api/members/{id}/widgets/today` take `?tz=<IANA zone>` and `?refresh=1`;
  - `PUT` and `DELETE` on `/api/me/calendar` and `/api/members/{id}/calendar`;
  - `GET /api/me` and the profile responses gain a `calendar` summary (`connected` and `provider`, never the link);
  - layouts may contain `Today` cards.

  All additive.
- **Data:** migration `004_calendar.sql` adds `member_profiles.calendar_url` (default `''`). Nothing else changes.
- **Frontend:**
  - a `TodayWidget` and its Puck block;
  - a calendar-link field, with a "connected" state and Remove, in the profile panel;
  - new API hooks and types.
- **Later roadmap items:**
  - the morning briefing reads remembered Today results;
  - deployment seeds the demo family with a sample calendar and must keep the safe fetcher.
- **Docs:** when archived, add one line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
