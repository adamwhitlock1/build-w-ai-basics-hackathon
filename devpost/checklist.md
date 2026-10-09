---
doc: checklist
status: approved
---
# Build Checklist

Build mode: [not chosen yet — learn or fast, chosen after the build order is approved]

## Slices

- [ ] **1. Type a sentence, get a live widget**
  Becomes usable: Open the app in a browser, type "show me upcoming SpaceX launches" into the new-widget prompt, watch a calm "thinking" state, and a titled widget card appears filled with live results. No login and nothing saved yet.
  Why now: This is the kernel, and it carries the two biggest unknowns: whether the chosen OpenRouter model reliably returns the structured widget spec, and whether Brave Search returns usable data. If either is wrong, we learn it before anything is built around it. Bootstrapping both halves of the app (Go module, Vite app) happens here so every later slice has a home.
  PRD ref: `prd.md > Custom Widgets ("Describe-a-Widget")`, `prd.md > The New-Widget Prompt`, `prd.md > Data Sources`
  Spec ref: `spec.md > Components` (`internal/sources`, `internal/llm`, `internal/widgets`, the "describe-a-widget" prompt modal), `spec.md > External Services and Dependencies`, `spec.md > Important Failure Modes`, `spec.md > Where It Runs and How Someone Tries It`, `spec.md > Look and Feel`
  Build: Backend: pinned Go module (`go 1.22.0`), `main.go` on stdlib `net/http`, `.env` loading, fail-fast on missing keys and an unknown `OPENROUTER_MODEL`, a run script/Makefile that sets `CGO_ENABLED=0 GOTOOLCHAIN=local`; `internal/sources` (Source interface, Registry, Brave web + news); `internal/llm` (go-openai pointed at OpenRouter, strict JSON-schema output picking `{title, source, query, archetype}`, validated in Go, one retry); `POST /api/widgets/from-prompt` that maps Brave results into one fixed item shape and returns `{title, archetype, items}` (422 for an unknown source). Frontend: Vite + React 18 + TypeScript, Tailwind v4 with the spec palette, Gambarino/Switzer fonts, dev server on `:5174` proxying `/api`, TanStack Query; a temporary page holding the inline prompt (cycling example hints, Enter submits, thinking state, inline "Try again") and a widget card that renders all five archetypes plus a fallback. `.env.example` and a README with run steps.
  Verify (mechanical): `go vet` and `go build` pass; starting without keys exits with a clear message; `curl` the SpaceX prompt to `/api/widgets/from-prompt` and confirm a known source, a title, and non-empty items; `npm run build` passes; drive the page in the browser, submit the prompt, and confirm the thinking state then a populated card; force an LLM failure and confirm "Try again" resubmits.
  Learner check: Start both halves, open http://localhost:5174, type "show me upcoming SpaceX launches" and then a prompt of your own. Does the widget feel like the "small magic trick" you pictured, and is the content any good?
  Commit: `Add describe-a-widget: a sentence becomes a live widget`

- [ ] **2. Your own account, your family, and widgets that stay**
  Becomes usable: Sign up with email and password, fill in the create-family intro (family name, co-admin emails, your interests, your location), and land on "Your page". Widgets you describe are saved to you and are still there after logging out and back in. Logged-out visitors are sent to the login screen; a wrong password shows an inline error.
  Why now: Everything after this is per-member, so the database, sessions, and "who am I" have to exist before the canvas, the curated widgets, or invites have anywhere to live. It comes second because the kernel's output now needs a home.
  PRD ref: `prd.md > The Core Journey` (steps 1–3), `prd.md > Login / Sign-up`, `prd.md > Create-Family Intro`, `prd.md > Authentication and Account`, `prd.md > States and Boundaries`
  Spec ref: `spec.md > Data Model`, `spec.md > Components` (`internal/store`, `internal/auth`, `internal/family`, Routes), `spec.md > The Core Journey Through the System` (steps 1–3)
  Build: `migrations/001_init.sql` (spec schema plus the sessions table), `internal/store` on modernc SQLite, scs sessions with the SQLite store, bcrypt; `POST /api/signup` (family and admin user created together), login, logout, `GET /api/me`, `POST /api/families` completing the intro (co-admin emails stored as pending admin invites); every private handler checks the session itself; `from-prompt` now saves a `custom_widgets` row for the user; `GET /api/me/homepage` returns profile and saved widgets. Frontend: TanStack Router file routes (`/login`, `/signup`, `/create-family`, protected `/`), one fetch wrapper that sends any 401 to `/login`, and the homepage listing saved widgets.
  Verify (mechanical): `go vet`/`go build` pass; a `curl` script runs signup → create family → describe a widget → logout → login → `GET /api/me/homepage` and finds the widget; a wrong password returns 401 with no session; a logged-out request to a private endpoint returns 401; restart the server and the data is still there; `npm run build` passes; walk the flow in the browser.
  Learner check: Sign up, create your family with your real interests and location, describe a widget, log out, log back in, and confirm the widget is still on your page. Try a wrong password too.
  Commit: `Add accounts, family setup, and saved custom widgets`

- [ ] **3. Articles and Local Events pick themselves from your interests**
  Becomes usable: Your page now opens with an Articles card (3–5 items: title, one-line summary, source, optional thumbnail) and a Local Events card (3–5 upcoming events: name, date, venue, one-line description) chosen for your interests and location. Each card has a refresh control, shows "Nothing new today — try refreshing later" when there's nothing, and reseeds when you edit your interests or location.
  Why now: This finishes the "every widget is AI-driven" kernel by reusing slice 1's LLM and search plumbing. Local events are the remaining data-quality risk (Brave has no events feed, so events come from web results plus LLM extraction), and we want to know they are demo-worthy before arranging the page around them.
  PRD ref: `prd.md > Articles Widget`, `prd.md > Local Events Widget`, `prd.md > Data Sources`, `prd.md > States and Boundaries`
  Spec ref: `spec.md > Components` (`internal/llm` CurateArticles/CurateEvents, `internal/widgets`, Custom blocks, `internal/layout` `PUT /api/me/profile`), `spec.md > Important Failure Modes`
  Build: `CurateArticles` (LLM writes 3–5 queries from interests → Brave news → best result each, deduplicated) and `CurateEvents` (location- and month-anchored web search → LLM extracts a strict event shape → past dates dropped in Go); `GET /api/widgets/articles` and `/events`; one backoff retry on a 429; an interests/location editor backed by `PUT /api/me/profile`. Frontend: Articles and LocalEvents cards with refresh, fetched once per visit (no refetch on window focus, to protect the search credit), invalidated when interests change.
  Verify (mechanical): `go vet`/`go build` pass; a small `go test` covers past-event filtering and the empty-results path; as a logged-in user, `curl` both endpoints and confirm 3–5 items with every required field; change interests via `PUT /api/me/profile` and confirm the article queries change; `npm run build` passes; load the page in the browser and confirm both cards render and refresh.
  Learner check: Open your page. Are the articles really about your interests? Are the events near you and still upcoming? Hit refresh, then change your interests and watch both cards reseed.
  Commit: `Add AI-curated Articles and Local Events widgets`

- [ ] **4. Arrange your page: drag, resize, and it sticks**
  Becomes usable: Your page is a 4-column grid of widget cards, with "This spot is empty" cards and a "+" at the end. Switch into arranging (see the edit-mode decision) and drag cards into a new order, or drag a card's right edge to resize it from 1/4 to 4/4 (it snaps to quarters). Changes save automatically and survive reloads and re-logins. A new widget from "+" opens inline at the top of the grid, fades into the next empty slot, and can be dragged like the rest.
  Why now: The canvas comes after the curated widgets so it arranges the real set of cards (Articles, Local Events, Custom), and the default layout is seeded once, here. Puck has no built-in resize, so the right-edge resize is custom code. A browser prototype during planning showed it works, but it still needs proving in the real app.
  PRD ref: `prd.md > Drag-and-Drop Layout`, `prd.md > The Homepage (the main surface)`, `prd.md > The New-Widget Prompt`, `prd.md > The Core Journey` (steps 6–7)
  Spec ref: `spec.md > Components` (Puck configuration, Custom blocks, the "describe-a-widget" prompt modal, `internal/layout`), `spec.md > Data Model` (`homepage_layouts`), `spec.md > Look and Feel`
  Build: Install `@puckeditor/core` pinned; Puck config with a root grid slot and `Articles`, `LocalEvents`, `Custom` blocks that each carry a 1–4 width; a default layout (Articles and Local Events at half width plus empty slots) seeded for anyone without one; `GET`/`PUT /api/me/homepage` storing the Puck data as JSON, saved with a short debounce; a right-edge resize handle snapping to quarters, with width buttons as a fallback; the "+" flow inserts the new widget into the layout; fade-and-slide arrival with the amber "fresh" border.
  Verify (mechanical): `go vet`/`go build` pass; a `curl` round-trip of `PUT` then `GET /api/me/homepage` returns the same layout, and malformed JSON is rejected; `npm run build` passes; drive the browser: a warm-up drag, then move a card, resize one to 3/4, reload, and confirm order and width persisted; log out and in and confirm again; add a widget with "+" and confirm it appears, can be dragged, and persists.
  Learner check: Rearrange your page the way you'd want it on a Tuesday morning: drag a few cards, resize one, add a new widget, then reload. Does dragging feel satisfying, and did everything stay put?
  Commit: `Add drag-and-drop, resizable homepage canvas`

- [ ] **5. Invite your family; everyone gets their own page**
  Becomes usable: The sidebar shows the family name, members with initials, pending invites (including the co-admins from setup), and "Invite member". Inviting an email (as parent or kid) gives you a link to copy. You can revoke a pending invite or copy its link again, and inviting an existing member shows "Already a member". Opening the link while logged out leads to "set your password", then to that person's own fresh page, whose layout is independent of yours.
  Why now: Multi-member is in the POC boundary and the demo's setup beat, but it builds on accounts, layouts, and curated widgets that now exist. Doing it after the canvas means each new member's page is complete the moment they arrive.
  PRD ref: `prd.md > Family Management`, `prd.md > Authentication and Account`, `prd.md > The Core Journey` (steps 4–5), `prd.md > States and Boundaries`
  Spec ref: `spec.md > Components` (`internal/family`, Routes `/invite/:token`), `spec.md > The Core Journey Through the System` (steps 4–5), `spec.md > Data Model` (`invites`)
  Build: `POST /api/families/{id}/invites` (admin only, parent-or-kid role, duplicate-member check), revoke, `GET /api/me/family` listing members and pending invites, `POST /api/invites/{token}/accept` (creates the user, profile, and default layout, then signs them in; rejects expired or used tokens). Frontend: sidebar with member list and invite form with copyable links, and `/invite/$token` set-password page.
  Verify (mechanical): `go vet`/`go build` pass; a `curl` script: admin invites → accept with a password while logged out → new session lands on its own default homepage; a second invite to that email → "Already a member"; a kid creating an invite → 403; reusing an accepted token → rejected; revoke works; `npm run build` passes; open an invite link in a fresh browser session and complete it.
  Learner check: Invite a test email as your son, open the link in a private window, set a password, and land on his page. Rearrange it there, then confirm your own page didn't change.
  Commit: `Add family invites and per-member homepages`

- [ ] **6. Parents can switch to any member's page and set their interests**
  Becomes usable: As a parent, click a family member in the sidebar to see their page: their layout, their custom widgets, articles from their interests, events at their location. Set that member's interests and location from there, and their cards reseed. Kids only ever see their own page. Logging out lives in the sidebar too.
  Why now: Last because it reads everything the earlier slices built. It completes the demo's setup beat: a parent setting interests for one of the kids.
  PRD ref: `prd.md > Family Management`, `prd.md > The Homepage (the main surface)` (switch view), `prd.md > States and Boundaries` (permissions and boundaries)
  Spec ref: `spec.md > Decisions and Open Issues` ("Switch view" sidebar interaction), `spec.md > Components` (`internal/layout`, `internal/widgets`, `internal/auth` RequireAdmin)
  Build: Admin-only, same-family member endpoints for viewing a member's homepage, their articles and events, and updating their profile; a member selection carried in the URL (`?as=`); the switched view shows that member's page read-only plus their interests editor.
  Verify (mechanical): `go vet`/`go build` pass; a `curl` script: as admin, read a member's homepage and set their interests, then confirm that member's article queries reflect them; as a kid, every other-member request returns 403; a user from a second test family gets 403/404 on this family's members; `npm run build` passes; switch views in the browser and confirm the content changes.
  Learner check: As yourself, switch to your daughter's page, set her interests to something she'd actually like, and watch her cards change. Then log in as a kid account and confirm only your own page is visible.
  Commit: `Add parent view switching and member interest editing`

## Hands-on Checkpoints

- [ ] Early usable behavior explored — after slice 1: the describe-a-widget moment in the browser, while its look and output quality can still shape the curated widgets and the canvas
- [ ] Final kick-the-tires exploration and feedback completed

## Final Review

- [ ] Final review complete — feedback resolved and learner confirms ready to ship

## Code Tour and App Map

- [ ] Learning activity complete — guided route, focused alternative, prior practice connected, or brief recap
- [ ] Optional edit and transfer reflection addressed — offered/declined/already covered/not applicable as appropriate
- [ ] `devpost/app-map.html` generated from finished code, checked, and shown, including a project-grounded practice to reuse

Activity and evidence:
Route and stops:
Edit outcome:
Reflection:
Activity mode:

## Revisions

- Go is built and run with `CGO_ENABLED=0 GOTOOLCHAIN=local` through a Makefile (`make run`), and Go modules are pinned to versions that support go1.22.0 (e.g. scs v2.8.0, not v2.9.0) — pre-build research found that go1.22.0 binaries built with cgo crash at launch on this Mac, and newer module versions would silently pull in a newer toolchain.
- The frontend runs on port 5174 (strict) instead of 5173 — another app on this machine already uses 5173.
- Puck is `@puckeditor/core` 0.23.0 (the `@measured/puck` package is deprecated), and right-edge resize is custom code on top of it — Puck has no built-in component resize.
- Tailwind is v4 through its Vite plugin, with theme tokens in CSS, instead of the CDN script plus config files — that's the current setup for Vite.
- The kernel's LLM output is `{title, source, query, archetype}` followed by a second LLM call that shapes search results into one fixed item shape — the spec's single call asked for render content before any search results existed, and strict structured output can't describe free-form content.
- The LLM output is validated in Go instead of trusted — OpenRouter can silently ignore `response_format`, and can return HTTP 200 with an error and no choices.
- Default model is `openai/gpt-4.1-mini`, checked at startup — the spec's example free model no longer exists on OpenRouter.
- Brave Search facts updated (paid Search plan with $5/month free credit, separate news endpoint, no events feed) and `SESSION_SECRET` removed — the spec's Brave free tier is outdated, and scs doesn't sign cookies.
- Signup creates the family and the admin user in one transaction, and the schema gains a `sessions` table and `invites.is_admin` — `users.family_id` is `NOT NULL`, scs needs its own table, and co-admin invites need a role.
