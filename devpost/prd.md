---
doc: prd
status: approved
---
# Porchlight — Product Requirements

One line: a per-family-member homepage where every widget is AI-driven — pre-built ones are auto-curated from each member's interests, and any member can describe a new widget in plain language and have it built and rendered. Built for the learner's own household first; market is later.
Source: `scope.md > The Unique Kernel`, `scope.md > The Core Loop`, `scope.md > The POC Boundary`.

## The Core Journey

The end-to-end path someone walks from sign-up to "oh, that's cool":

1. **Sign up.** A parent creates a Porchlight account with email and password.
2. **Create the family.** A short intro screen collects: family name, one or more co-admin email(s), the parent's own interests (freeform text), and their location (freeform — city is fine). This becomes the seed for their homepage.
3. **Land on the homepage.** The parent is taken to their own homepage, which is already populated: two pre-seeded widgets (Articles, Local Events), a few empty slots, and a "+" icon to add a new widget. A sidebar lets them invite other family members and switch between views.
4. **Invite the rest of the family.** From the sidebar, the parent enters the spouse's and kids' email addresses. Each invitee receives a link to set their password and lands on their own homepage.
5. **Open the homepage.** Each family member logs in and sees *their own* homepage — their interests, their location, their pre-seeded widgets, their layout. The page is theirs.
6. **Rearrange.** Anyone taps "Arrange" and drags widgets into a new order; widgets can also be resized from one-quarter to four-quarters of the page width. The layout persists for that member.
7. **Describe a new widget.** Anyone clicks the "+" icon, types a sentence ("show me upcoming SpaceX launches"), sees a brief "thinking" state, and a new widget appears on the page, populated with live data. They can drag it into position among the others.
8. **Come back tomorrow.** Layout, custom widget definitions, interests, and location all persist. The articles and events widgets refresh with new content on each visit.

Source: `scope.md > The Core Loop`, `scope.md > The What "Working" Looks Like`.

## Screens and Layout

Porchlight is one main surface (the homepage) with a small number of supporting surfaces around it.

### Login / Sign-up
- Email and password fields, a "create account" toggle.
- A logged-out user can also click an invite link to set their password and land directly on their homepage.

### Create-Family Intro
- Shown immediately after the first sign-up.
- Form fields:
  - Family name
  - Co-admin email(s) — optional, comma-separated
  - "What would you like to see on your page?" — freeform interests
  - "Where are you?" — freeform location
- A single "Create family" button submits the form.

### The Homepage (the main surface)
- **Sidebar (left):** family name at the top, a list of family members to switch views (with an avatar/initials), an "Invite member" button.
- **Main area:** a 4-column grid of widget cards. Each widget spans 1/4 to 4/4 of the grid width (resizable by dragging the right edge). A "+" icon at the end of the grid opens the new-widget prompt.
- **Empty slots:** first-use renders a few empty cards ("This spot is empty — add a widget to fill it") that disappear once the grid is full.
- The sidebar's "switch view" is the same screen with a different member's data — same layout, different interests, different widgets, different location.

### The New-Widget Prompt
- Triggered by the "+" icon. Inline expansion at the top of the grid (not a separate page or a modal that traps focus).
- A single text input with example placeholders that cycle or appear as a hint: *"try: upcoming SpaceX launches"*, *"show me local farmers markets this weekend"*, *"give me a quote of the day"*.
- Submit on Enter. A "thinking" state replaces the input ("Building your widget..." or similar) and a spinner.
- On success: the prompt collapses and the new widget card animates into the grid in the next empty slot.
- On failure: a small inline error with a "Try again" button (resubmits the same prompt).

## Look and Feel

Direction from the design interview:
- **Character:** warm, welcoming, soft. Not corporate. The page feels like a small room with a light left on, not a dashboard.
- **Typography:** Gambarino (Fontshare) for display, headings, and any larger text treatments; Switzer (Fontshare) for body, controls, and widget content. System stack as the offline fallback.
- **Color palette:** soft warm neutrals for surfaces, with one calm accent color (likely an indigo or muted amber) for the "magic" moments — the "+" icon, the thinking spinner, the freshly-arrived widget's border.
- **Density:** generous whitespace, large widget card padding, soft shadows. Widget cards feel like cards on a kitchen table, not cells in a spreadsheet.
- **Motion:** drag-and-drop has a satisfying lift-and-settle. The new widget animates in (a soft fade + slide), not a hard pop. The thinking state is calm, not anxious.

## Features and Behavior

### Authentication and Account

Anyone with a Porchlight account logs in with email + password.

- A new account is created either by signing up directly (a parent creating the family) or by accepting an invite link from a parent (kids and co-admins).
- Invitees receive a link that takes them to a "set your password" page; after that, they log in normally.
- Each account belongs to exactly one family. A user cannot be a member of two families in v1.
- An admin can re-send or revoke an invite from the sidebar.

**Acceptance criteria:**
- A new sign-up lands on the create-family intro.
- An invitee clicking the invite link lands on the "set your password" page, and after that, on their own homepage.
- Logging in with valid credentials takes the user to their own homepage; invalid credentials show an inline error and the form stays put.

Source: `scope.md > The POC Boundary` (admin can create the family, invite members, and set each member's interests).

### Family Management

The family is the unit. A family has a name, 1–2 admin(s), and N members.

- The first user to sign up becomes admin and creates the family.
- The admin can invite more admins (co-parents) or regular members.
- The admin can switch to any member's view from the sidebar; non-admins can only see their own.
- Switching views is the same screen, populated with the selected member's interests, location, layout, and widgets.

**Acceptance criteria:**
- The create-family form saves and the family is queryable.
- The sidebar lists all family members; clicking another member's name swaps the homepage contents to that member's data.
- An admin can invite a new member from the sidebar; the invitee receives a working link.

### Homepage and Pre-Built Widgets

Each family member has a homepage with two pre-built widget types: **Articles** and **Local Events**. Both are populated by the LLM calling Brave Search, using that member's interests and (for events) location.

#### Articles Widget
A card listing 3–5 article summaries. Each item shows: title, one-line summary, source, and an optional thumbnail.

- On first render, the LLM is asked: *"Given the user's interests [list], produce 3–5 relevant article search queries and return the top result for each via Brave Search."* The widget shows the LLM's picks.
- On refresh (every page load, or via a "refresh" button on the card), the LLM regenerates queries and re-fetches.
- The widget is reseeded if the user changes their interests.

**Acceptance criteria:**
- The Articles widget on the homepage shows 3–5 items, each with a title and a one-line summary.
- The items are relevant to the member's interests (a member whose interests are "space exploration, soccer" sees articles about space and soccer, not random topics).
- A "refresh" control on the card re-fetches and updates the list.

#### Local Events Widget
A card listing 3–5 upcoming local events. Each item shows: event name, date, venue, and a one-line description.

- On first render, the LLM is asked: *"Given the user's interests [list] and location [string], produce 3–5 local-events search queries and return the top result for each via Brave Search."*
- Same refresh model as Articles.

**Acceptance criteria:**
- The Local Events widget shows 3–5 events, each with a name, date, and venue.
- The events are local (anchored to the member's location) and relevant to their interests.
- A "refresh" control on the card re-fetches and updates the list.

### Custom Widgets ("Describe-a-Widget")

The kernel. Anyone can add a new widget by typing a sentence.

- Click the "+" icon at the end of the grid.
- A single text input appears with example placeholders that hint at the kinds of prompts the LLM can handle.
- The user types their sentence and submits.
- The LLM is asked: *"Given the user's prompt [string] and the available data sources [list], pick the best data source and produce a search query. The data sources are: news search, events search, generic web search. Return a JSON spec with source, query, and display shape."*
- The system calls the chosen source, fetches the data, and renders it as a widget card.
- The widget appears in the next empty slot of the grid.
- If the LLM can't pick a source, or the API fails, the prompt shows an inline "Try again" button.

**Acceptance criteria (the kernel):**
- Clicking "+" opens the prompt.
- Typing "show me upcoming SpaceX launches" and submitting produces a widget titled roughly "SpaceX Launches" containing a list of upcoming launches.
- During the LLM/API call, the prompt shows a "thinking" state (no button, no submission).
- On success, the new widget is in the grid, draggable, and persists across page reloads.
- On failure, an inline "Try again" button appears; clicking it resubmits the same prompt.

Source: `scope.md > The Unique Kernel`, `scope.md > What "Working" Looks Like` (the third beat — the "oh, that's cool" moment).

### Drag-and-Drop Layout

Widget cards can be reordered and resized.

- **Arrange mode:** the page normally behaves like a page — links and refresh buttons inside widgets just work. An "Arrange" toggle switches into arranging, where widgets can be dragged and resized; "Done" switches back. (Chosen during the build over always-draggable widgets, which would make every card's links and buttons fight the drag layer.)
- **Reorder:** in arrange mode, drag a widget by its body; other widgets reflow around it. Drop in the new position.
- **Resize:** drag the right edge of a widget from its current width (1/4, 2/4, 3/4, or 4/4) to a new width. Snap to 1/4 increments.
- **Persistence:** the layout is saved automatically on drop, no explicit save button.
- **Per-member:** each member's layout is stored independently. Switching views shows each member's own layout.

**Acceptance criteria:**
- Tapping "Arrange" enables dragging and resizing; tapping "Done" returns to the normal page, where links and buttons in widgets work.
- Dragging a widget into a new position moves it; releasing the mouse drops it there.
- Resizing a widget to a different width snaps to 1/4 increments and persists.
- Logging out and back in preserves the layout.
- Switching to a different family member's view shows that member's layout, not the admin's.

### Data Sources

The system supports a small, named menu of data sources. v1 ships with one backing service: **Brave Search**. The LLM is told the names and shapes; the system has a registry mapping names to handlers.

v1 menu:
- `news_search` — Brave Search, news-shaped results
- `events_search` — Brave Search, events-shaped results
- `web_search` — Brave Search, generic results

**Acceptance criteria:**
- The LLM's output for a custom widget specifies one of `news_search`, `events_search`, or `web_search`.
- The system successfully calls Brave Search for the chosen source and renders the result.

## States and Boundaries

- **First use (new family member):** the homepage shows two pre-seeded widgets (Articles, Local Events) plus 1–2 empty placeholder cards. The "+" icon is always available.
- **Logged out:** any page except `/login` redirects to `/login`. Invite links work for logged-out users.
- **LLM or API failure on custom widget:** the prompt shows an inline "Try again" button; clicking it resubmits the same prompt. No widget is added.
- **No matching content for interests:** if the LLM can't find relevant articles or events, the widget shows a calm message ("Nothing new today — try refreshing later") rather than empty space.
- **Inviting an existing email:** the sidebar surfaces a small error ("Already a member") and does not send a duplicate invite.

**What persists across sessions:**
- Each member's interests, location, layout, custom-widget definitions, and the most recent refresh of pre-built widgets.

**What does not persist:**
- Pre-built widget results are not cached server-side beyond a single page load. The LLM is asked again on each visit (or refresh).

**Permissions and boundaries:**
- Admins can edit any member's interests and view any member's homepage.
- Non-admins can only edit their own interests and view their own homepage.
- All members can rearrange their own widgets, resize their own widgets, and create their own custom widgets.

## Product Decisions

- **Single text field for custom widgets, not a wizard.** The user said this fits Porchlight's "small magic trick" feel; a wizard adds friction and dilutes the kernel.
- **Inline "try again" on errors, not a full error page.** The demo is short and the user wants errors to be unobtrusive.
- **Email + password for everyone, including kids.** Kids have their own accounts; this is the simplest model that supports the "switch view" sidebar cleanly.
- **Brave Search only, abstracted behind a registry.** v1 ships one data source. v2 can add PredictHQ or Google Events with a config change, not a refactor.
- **A few seeded + empty slots on first use.** Hybrid first-use: not a blank canvas (overwhelming), not a fully populated page (less inviting to customize).
- **Cater the demo to prompts the LLM handles well.** E.g., "show me upcoming SpaceX launches" — known to work. We don't need to handle adversarial prompts in v1.
- **System font stack as the offline fallback for Gambarino + Switzer.** Keeps the PoC runnable without network access; production v2 can always require the Fontshare fonts.

## What We're Building

The full proof of concept must do all of the following to be complete:

1. Email + password authentication (sign up, log in, log out, invite-link-based account creation).
2. Create-family intro that collects family name, co-admin emails, the parent's interests, and the parent's location.
3. Sidebar with family member list and "invite member" action.
4. Per-member homepage with a 4-column grid that supports reorder and resize (1/4 to 4/4) with persistence.
5. Articles widget, LLM-curated via Brave Search from the member's interests.
6. Local Events widget, LLM-curated via Brave Search from the member's interests and location.
7. "+" icon that opens a single text field prompt with example placeholders.
8. Custom-widget creation flow: prompt → thinking state → widget appears in the grid → draggable.
9. Inline "try again" error state for custom-widget failures.
10. Go API backend, simple frontend, runs locally. No production deployment.

## Deferred From the POC

These are features Porchlight implies but does not build in v1. Each is named with its reason.

- **Homework photo upload + admin review/check-off.** Separate product surface (photo storage, review queue, notifications). Source: `scope.md > Explicitly Cut`.
- **Gamification (points for chores, homework, admin-assigned tasks).** Requires a points ledger, history, and reward logic. Source: `scope.md > Explicitly Cut`.
- **Unified family calendar view.** Large UI surface; not part of the personalized-homepage kernel. Source: `scope.md > Explicitly Cut`.
- **Per-member schedule widget.** Calendar problem in disguise; not load-bearing for the kernel. Source: `scope.md > Explicitly Cut`.
- **Daily fun / meme widget.** Cut during scope to make room for the natural-language widget feature. Source: `scope.md > Explicitly Cut`.
- **Truly open agentic UI generation (OpenUI-style general "describe any UI").** Research-grade, weeks of work. v1 has a constrained natural-language widget with three data sources. Source: `scope.md > Explicitly Cut`.
- **Multi-tenant / multi-family market features.** Explicitly v2 per the learner's stated plan (v1 = own household; v2 = market). Source: `scope.md > Explicitly Cut`.
- **External auth providers (OAuth, SSO).** v1 uses simple email + password. Source: `scope.md > Explicitly Cut`.
- **Production deployment.** PoC runs locally. Source: `scope.md > Explicitly Cut`.
- **Mobile app.** Desktop browser only for v1.

## Possible Later Enhancements

A sentence each. No acceptance criteria.

- **Real LLM cache for pre-built widgets.** Cache the LLM's pick of articles/events for an hour so refresh isn't a fresh LLM call every time.
- **PredictHQ integration for richer local events.** Drop-in for `events_search`; a member in a supported area gets richer event metadata. Source: `scope.md > Data Sources Explored`.
- **Google Events via hasdata and Real-Time Events Search via openwebninja.** Same drop-in model. Source: `scope.md > Data Sources Explored`.
- **"Share this widget with the family."** A parent can publish a custom widget to all family members' homepages, not just their own.
- **Member-to-member messages on widgets.** A kid can drop a note on a widget ("saw this and thought of you, dad").
- **Widget refresh schedules.** Per-widget cadence (daily, hourly, on-demand) instead of every page load.
- **Weather widget.** A real weather source, not a Brave Search query.
- **Onboarding polish.** A "tour" the first time a family member lands on their homepage.

## Non-Goals

Things this project will NOT do, each with a brief reason.

- **Real-time collaboration.** Two family members editing the same widget at the same time is out of scope. Each member has their own homepage.
- **Notifications, email digests, push.** Out of scope for v1; the app is a place you visit, not something that pings you.
- **Family chat or messaging.** Not the kernel; would inflate the build.
- **Privacy controls / per-widget visibility.** All widgets on a member's homepage are visible to admins. Per-widget privacy is v2.
- **Adversarial-prompt handling.** Custom widgets are a single LLM call; we don't defend against prompt injection. Acceptable for a single-family, local-network PoC.
- **Offline mode.** The PoC requires network access (LLM API + Brave Search). No offline cache in v1.

## Open Questions

- **None blocking.** The PRD is internally consistent. Any remaining unknowns (e.g., specific LLM model, exact resize handle behavior) are 4-spec or 5-build decisions, not product decisions.

## Post-PoC Additions

Everything above describes the proof of concept as planned and built with the Devpost Learn skill pack, and is kept unchanged. The commit tagged `poc-complete` is that snapshot. Features added after it are planned and specified with OpenSpec: proposals in progress live in `openspec/changes/`, finished ones in `openspec/changes/archive/`, and the specs they produce in `openspec/specs/`.

- **Fresh, fast widgets** (2026-10-11): widgets remember their results, so pages open instantly. Stale ones refresh in the background (Articles 3h, Local Events 12h, described widgets 6h), described widgets gain a refresh button, and every card shows "Updated … ago". Spec: `openspec/specs/widget-freshness/spec.md`; change: `openspec/changes/archive/2026-10-11-fresh-fast-widgets/`.
- **Kid-safe picks** (2026-10-11): content for kids (members who aren't parents) uses Brave strict safe-search and an under-18 instruction on every AI step, decided by whose page it is, so parents viewing a kid's page see the same. Remembered results and refreshes keep it, and parents see a "Kid-safe" marker on those cards. Spec: `openspec/specs/kid-safe-content/spec.md`; change: `openspec/changes/archive/2026-10-11-kid-safe-picks/`.
- **Weather source** (2026-10-12): a describe-a-widget source backed by Open-Meteo (free, no key) gives real forecasts: current conditions and 7 days for a place, in °F and mph for US places and °C and km/h elsewhere. A request with no place uses the member's saved location, weather widgets go stale after 1 hour, and every weather card credits Open-Meteo. Spec: `openspec/specs/weather-widgets/spec.md`; change: `openspec/changes/archive/2026-10-12-weather-source/`.
- **Today calendar card** (2026-10-12): every page gets a Today card at the top, filled from a private iCal link the member (or a parent, for them) pastes in the profile panel. It shows today's events (tomorrow's too from 6 pm), with repeating, moved and deleted occurrences handled. The link stays secret on the server and is fetched safely over https only. Spec: `openspec/specs/today-calendar/spec.md`; change: `openspec/changes/archive/2026-10-12-today-calendar-card/`.
- **Morning briefing** (2026-10-13): a permanent card above Today greets the member by name for the time of day and gives 1–3 sentences the AI writes from facts Porchlight already has: the Open-Meteo forecast (tomorrow's from 6 pm), today's calendar events, and a few remembered headlines and nearby events. It never searches. It's written on the first visit of the day and rewritten only when those facts change, at most 4 times a day. Kids get the under-18 guidance, and the AI never receives a name, email, location or calendar link. Spec: `openspec/specs/morning-briefing/spec.md`; change: `openspec/changes/archive/2026-10-13-morning-briefing/`.
- **Demo family seed** (2026-10-23): `make demo` rebuilds a throwaway database (`backend/porchlight.db.demo`, never `porchlight.db`) with the Rivera family. Maya and Sam are parents, Jordan is a kid of about 15, Lily is a kid of about 9 with no calendar, and Grandma is a pending parent invite. A tiny Chen family is included for isolation checks. `make demo-serve` runs the API on it with development-only fixture calendars that always have events today. Widget results are pre-filled, so browsing the demo spends no search credit. Spec: `openspec/specs/demo-data/spec.md`; change: `openspec/changes/archive/2026-10-23-demo-family-seed/`.
- **Go upgrade** (2026-10-24): the backend moves from go1.22.0 (past end of support) to go1.27.3, with `go 1.27.0` in `go.mod` and `GOTOOLCHAIN=local`, so no toolchain is ever downloaded. Dependencies are refreshed one at a time (SQLite driver v1.61.0 with libc pinned, scs v2.9.0, x/crypto v0.58.0, golang-ical v0.3.8), `go fix` modernizes the code, and `make check` now runs govulncheck (0 reachable vulnerabilities). `CGO_ENABLED=0` stays, by choice, for static builds that cross-compile to Linux; go1.27 fixed the macOS crash that first forced it. No app behavior changed. Change: `openspec/changes/archive/2026-10-24-go-upgrade/` (tooling only, no spec).
- **v2 foundation** (2026-10-24): groundwork for the family agent, with no visible change:
  - The widget work moves out of the HTTP handlers into services (`WidgetBuilder`, `Curation`, `Agenda`, `Briefings`) that take a context and a member, and the handlers only translate. The existing tests pass with no assertion changed.
  - `routes()` takes a `deps` struct.
  - The middleware lets streamed replies flush.
  - The write origin check uses Go's `http.CrossOriginProtection`.
  - Every AI answer's token usage is recorded per member and feature in `llm_usage`, by a background writer, with no message content.
  - GitHub Actions CI runs the backend checks, the race detector, and the frontend lint and build.

  Spec: `openspec/specs/llm-usage/spec.md`; change: `openspec/changes/archive/2026-10-24-v2-foundation/`.
- **Genkit integration** (2026-10-24): Genkit for Go v1.14.0 joins the backend in one package, `internal/aikit`, which builds it with OpenRouter (and, with a key, MiniMax for the owner's own use, never kids). The morning briefing now goes through Genkit, with the same prompts, rules, retries and error messages as before, which a contract test checks; `PORCHLIGHT_BRIEFER=legacy` switches back. Genkit calls record usage in `llm_usage` like every other AI call. `aitest` gives later changes a scripted model for tests, `make ai-smoke` checks the real providers on request, and `make dev-ui` opens Genkit's Developer UI on a laptop only. Change: `openspec/changes/archive/2026-10-24-genkit-integration/` (no spec; the briefing's behavior is unchanged).
- **Kid tutor agent** (2026-10-24): kids get Chat, the family agent built on Genkit. Homework help guides with a hint ladder and never hands over answers; Explore is for curiosity and creative projects. Its voice follows a birth year parents set (younger up to 12, teen from 13), replies stream in, and every screen says "Your parents can review these chats". A fast safety check reads every kid message first. It flags but never blocks; a possible crisis gets a caring reply ending with 988 / 741741 / 911 text added by the server, and a provider's refusal fails safe. Parents get Reviews (crisis first, mark reviewed, full transcripts, delete) and a crisis alert on every page. Kid traffic runs only on a separate OpenRouter key with zero-data-retention routing (Claude Haiku 5.5 by default, since the OpenAI models' ZDR endpoints block crisis messages), and is off without it. `make ai-eval-kids` checks the behavior on real models. Specs: `openspec/specs/kid-chat/`, `kid-chat-safety/`, `parent-review/`, and `demo-data/` updated; change: `openspec/changes/archive/2026-10-24-kid-tutor-agent/`.
- **Agent tools** (2026-10-24): the kid agent can look things up for the kid who's chatting, and only for them: the weather where they live, their own calendar (today and tomorrow, in their time zone), their page's picks, and a kid-safe web search. Search is in Explore only, never Homework help, with at most 2 per message, short snippets and no links. Tools are built per reply as Genkit dynamic tools, so no model argument can point them at another member, and whatever they return is treated as information, not instructions. Each tool use shows as a note ("Checked the weather", "Searched: …") to the kid and in parents' transcripts, and is kept with the conversation. `make ai-eval-kids` now has 17 cases, including an instruction planted in the picks. Spec: `openspec/specs/kid-chat-tools/`; change: `openspec/changes/archive/2026-10-24-agent-tools/`.
- **Parent assistant** (2026-10-24): parents get a private, read-only Assistant. Only the parent who had a conversation can see it, and kids can't use it. "Ask anything" runs on the main key, with web research citing links, the family's day (anyone's calendar), weather and picks. "About the kids" adds the kids' chats (review queue, conversations, transcripts) and runs entirely on the kids' zero-retention key; the kids'-chats tool is only built for that kind. Its conversations are kept in their own tables. Replies stream with tool notes, kid and parent tools share one builder, kid chat now points parents to the Assistant, and `make ai-eval-parent` checks it on real models. Spec: `openspec/specs/parent-assistant/`, with `kid-chat` updated; change: `openspec/changes/archive/2026-10-24-parent-agent/`.
- **Kinlight identity** (2026-10-24): Porchlight is renamed Kinlight, everywhere, as a clean break before any deployment: the UI, the AI prompts, the Go module, `KINLIGHT_*` settings, the session cookie and `kinlight.db`. The server refuses to start while an old setting remains, and names its replacement. The app takes on the identity in `brand/`: the lantern mark in the tab, home-screen icon and header; Fraunces and Figtree bundled with the app instead of loaded from Fontshare; and two themes, Daybreak and Porch light, built from the brief's tokens, with haint blue for buttons and links and amber kept for light. Each member picks Automatic, Daybreak or Porch light; the choice is saved with their account and shown before the first paint. A contrast check in CI holds every pairing to WCAG AA in both themes. Both agents call family members by name rather than guessing pronouns. Specs: `openspec/specs/app-identity/`, with `parent-assistant` and `demo-data` updated; change: `kinlight-identity` in `openspec/changes/archive/`.
