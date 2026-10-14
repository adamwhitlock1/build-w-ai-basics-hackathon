## Why

Each card on a member's page answers one question: what's on today, what's in the news, what's happening nearby. Nothing pulls them together into the single glance a family wants first thing in the morning. A few warm sentences written by the AI ("75° and sunny, soccer at 4, and a Starlink launch Tuesday") do that. They're also the clearest AI moment for the demo opener (roadmap item 5). The Today card (item 4) now gives the briefing the day's plans, so this is the right time to build it.

## What Changes

- **A new permanent "briefing" card at the top of every member's page**, above Today:
  - The page shows a greeting for the time of day and the member's name ("Good morning, Maya").
  - Below it are 1–3 short sentences written by the AI that sum up the member's day.
  - Members can move and resize the card but not delete it, like the other built-in cards. Pages arranged before this change get it at the top.
- **The briefing is built only from facts Porchlight already has:**
  - today's weather for the member's saved location, asked from Open-Meteo directly (free and keyless, so it works without a weather card);
  - today's events from the member's remembered Today card (and tomorrow's, in the evening);
  - a few headlines from their remembered Articles;
  - a few items from their remembered "Happening near you" events.

  It never runs a paid search. Anything missing is simply left out.
- **What it costs:**
  - The first visit of the day writes the briefing, with one AI call.
  - Later visits get the remembered text, with no AI call.
  - It is rewritten only when the facts behind it change (for example, a calendar is connected, new events appear, or morning turns into evening). There are at least 10 minutes between rewrites and at most 4 AI calls per member per day.
  - A new day always starts fresh.
- **Kids get kid-safe briefings.** The under-18 guidance is added to the prompt, and it only uses results that were already picked in kid-safe mode. The response reports `kidSafe`, and parents see the "Kid-safe" marker, as on the other cards.
- **Parents viewing a family member's page see that member's briefing.** It's the same remembered text, and it counts toward the same daily cap.
- **Privacy:**
  - The AI receives event titles, times and locations, headlines, event names and a weather summary.
  - It never receives a calendar link, an email address or a member's name. The greeting with the name is added by the page.
- **Clear states:**
  - while it's being written: "Writing your briefing…";
  - when there's nothing to go on: "Add your interests, location or calendar, and your morning briefing will appear here", with a button to the profile panel;
  - when the AI fails and nothing was written yet today: "Couldn't write your briefing" with "Try again". If a briefing from earlier today exists, it stays on screen.
- **The briefing updates without a refresh button.** The page asks for it again whenever the Today, Articles or "Happening near you" results change, and the server decides whether that's worth a rewrite. It shows "Updated … ago".

## Capabilities

### New Capabilities
- `morning-briefing`:
  - the briefing card and where it appears;
  - which facts it may use and where each one comes from;
  - when it is written and rewritten, and its daily cap;
  - kid-safety;
  - what the AI may and may not receive;
  - parents viewing a member's briefing;
  - its empty, loading and error states.

### Modified Capabilities
- `today-calendar`: in "The Today card on every page", the Today card is no longer the first card on a new page. The briefing card is first, and Today comes right after it. Older layouts get the briefing added above their Today card.
- `widget-freshness`: in "Freshness windows", the briefing gets no age-based window. It is never reported stale, because it's rewritten when its facts change rather than when it grows old (see `morning-briefing`).

## Impact

- **Backend (Go):**
  - **Writing the text:** a new `llm.Client.WriteBriefing` (structured output: one `briefing` string, checked for length and blanks), with its own system prompt and the existing `forAudience` kid guidance.
  - **Serving the card:** a new `widgets.BriefingHandler` that gathers facts, decides whether to rewrite, and remembers the result.
  - **Storage:** it reuses `curated_widget_results`, with kind `briefing` for the text and kind `briefing_weather` for the day's forecast summary. There's no migration.
  - **Weather:** a small helper reads today's line from the Open-Meteo source's results.
  - **Layout:** `layout` gains the `Briefing` card type, `Default` puts it first, and `heal` adds it to layouts that lack it.
- **API (both additions):**
  - `GET /api/widgets/briefing?tz=<IANA zone>` and `GET /api/members/{id}/widgets/briefing?tz=…` (parents only, through `asParent`);
  - layouts may contain one `Briefing` card.
- **Frontend:**
  - a `BriefingWidget` and its Puck block;
  - a `useBriefing` hook whose query key includes the local date and the `updatedAt` of the Today, Articles and Events results, so it asks again when they change;
  - the greeting is built from the time of day and `nameFor(email)`;
  - `defaultLayout`, `config.tsx` and `LayoutView` gain the card.
- **Cost:** at most 4 AI calls per member per day, and no Brave searches. Open-Meteo is called at most once an hour per member and place. Deployment (roadmap item 7) adds rate limits and a daily spend cap on top.
- **Later roadmap items:** the demo family's seeded calendar and interests give the briefing something to say on first load.
- **Docs:** when archived, add one line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
