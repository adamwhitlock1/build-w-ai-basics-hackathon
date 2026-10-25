---
doc: spec
status: approved
---
# Porchlight — Technical Spec

## How This Works, In Plain Language

Porchlight is two halves that talk to each other over HTTP:

- **A Go backend** is a single program that runs on your computer. It owns the database (a small SQLite file), handles logins, and is the only thing that talks to the LLM (via OpenRouter) and to Brave Search. It speaks JSON over a REST API.
- **A React frontend** is a single-page app that runs in a browser. It uses **Puck** to render and edit the homepage canvas, and TanStack Query to fetch data from the Go API. The user logs in, the React app asks the Go API for the family's data, and Puck renders the homepage using the user's saved layout.

The **kernel — "describe a widget"** — works like this: the user clicks the "+" button in Puck, types a sentence, the React app POSTs that sentence to the Go API, the Go API calls OpenRouter with a structured-output prompt, the LLM returns a small JSON spec (a title, which data source to use, what to search for, and which display shape to use), the Go API calls Brave Search to fetch the data, asks the LLM once more to shape those results into widget items (grounded only in what the search returned), persists the new widget, and returns it. Puck adds the new widget to the page. The whole loop is server-mediated — the LLM is never called from the browser, so the OpenRouter key stays on the server.

**Why this shape:** it's a clean separation of concerns. The Go side is pure API + database + LLM proxy, which is exactly the work the learner wants to learn. The React side is a thin Puck-powered canvas that talks to the Go API. Both halves are sized for a single afternoon.

## The Core Journey Through the System

PRD ref: `prd.md > The Core Journey`. Walking the 8 steps through the architecture:

1. **Sign up** — POST `/api/signup` with email + password. Go hashes the password (bcrypt) and, in one transaction, creates a `families` row (name still empty) and then the `users` row as admin — the family must exist first because `users.family_id` is `NOT NULL`.
2. **Create family** — POST `/api/families` with family name, co-admin emails, parent's interests, parent's location. Go names the family, creates a `member_profiles` row for the parent, records co-admin emails as pending admin invites, and creates a `homepage_layouts` row with a default Puck data structure that has the two pre-seeded widget blocks plus empty slots. A user whose family has no name yet is routed back to the create-family intro.
3. **Land on homepage** — GET `/api/me/homepage`. Go returns the user's Puck data + their member profile. React renders the homepage with Puck in view mode.
4. **Invite family** — POST `/api/families/:id/invites` with the invitee's email. Go creates an `invites` row with a random token, returns the invite URL. (Email sending is out of scope; the link is shown in the sidebar for the admin to copy/share.)
5. **Each opens theirs** — invitee clicks the link → POST `/api/invites/:token/accept` with their chosen password. Go creates the user, links them to the family, and creates a `member_profiles` row. They land on their own homepage, which fetches the same way as step 3.
6. **Rearrange** — Puck's drag-and-drop reorders the blocks; on drop, Puck emits a `data` change event. The React app debounces and POSTs the new Puck data to PUT `/api/me/homepage`. Go persists to `homepage_layouts`.
7. **Describe-a-widget (kernel)** — User clicks "+" in Puck's block library → React opens the prompt modal. User types a sentence and submits. React POSTs to `/api/widgets/from-prompt`. Go:
   - Calls OpenRouter (via `sashabaranov/go-openai`, base URL pointed at OpenRouter) with a strict JSON-schema structured output: `{title: string, source: "news_search" | "events_search" | "web_search", query: string, archetype: "list" | "cards" | "hero" | "key_value" | "table"}`. Go decodes and validates it (retrying once).
   - Calls Brave Search via the chosen source.
   - Calls the LLM a second time to shape the search results into up to 8 items of one fixed shape (`{title, url, snippet, date, label, value, thumbnail}`, all strings) that every archetype renders from — grounded only in the results. If shaping fails, the raw results are mapped directly.
   - Persists a `custom_widgets` row with the render spec (title, archetype, items) + the `source`/`query` for re-fetching.
   - Returns the new block spec to the React app.
   - React calls `puck.dispatch` to add the block to the page.
8. **Come back tomorrow** — the layout, custom widgets, interests, and location are all in SQLite. The pre-built Articles and Local Events widgets are recomputed on each page load (LLM + Brave Search) — no caching for v1.

## Stack

| Layer | Choice | Why |
|---|---|---|
| **Backend language** | Go 1.22+ (built with `CGO_ENABLED=0`) | Stated learning goal; idiomatic for APIs. On this Mac, go1.22.0 binaries built with cgo crash at launch, so the Makefile builds pure-Go binaries. |
| **HTTP router** | Stdlib `net/http` (Go 1.22+ ServeMux with patterns) | Zero deps, most idiomatic for learning Go. Supports path params, methods, wildcards. Middleware written as small wrappers. |
| **Sessions** | `alexedwards/scs/v2` (SQLite-backed) | Server-side sessions, httpOnly cookies, no JWTs. |
| **SQLite driver** | `modernc.org/sqlite` (pure Go) | No CGO toolchain required; builds anywhere. |
| **LLM client** | `sashabaranov/go-openai` (pointed at OpenRouter's OpenAI-compatible endpoint) | Most-used Go OpenAI client; OpenRouter is OpenAI-compatible. |
| **External search** | Brave Search API (direct HTTP) | Single backing service for `news_search`, `events_search`, `web_search`; abstracted behind a registry. |
| **Frontend framework** | React 18 + TypeScript | Learner's preference. |
| **Build tool** | Vite | Fast, well-supported. |
| **Routing** | TanStack Router (SPA mode) | Learner's preference. **Not** TanStack Start — Start is full-stack and would overlap with the Go API. |
| **Server state** | TanStack Query | For data fetching, caching, optimistic updates. |
| **Styling** | Tailwind CSS v4 via `@tailwindcss/vite` (theme tokens in CSS `@theme`, no config files) | Speed of iteration for a PoC; learner's preference. Gambarino + Switzer loaded from Fontshare (one `<link>` per family) with system fallback. |
| **Canvas** | Puck — `@puckeditor/core` 0.23.0, pinned (the old `@measured/puck` package is deprecated) | Learner's preference; gives drag-and-drop reordering and the layout data structure. Resize is not built in — it's custom code on top of Puck. |
| **Password hashing** | `golang.org/x/crypto/bcrypt` | Stdlib-quality, idiomatic. |
| **LLM output validation** | Strict JSON-schema structured output generated from Go structs, then decoded and validated in Go | OpenRouter can silently ignore `response_format` for some providers, so Go never trusts the shape blindly. |

**OpenRouter model:** defaults to `openai/gpt-4.1-mini` (supports structured outputs, inexpensive). Configured in a single `OPENROUTER_MODEL` env var; the server checks the id against OpenRouter's live model list at startup.

**Pinned Go modules (verified together on go1.22.0):** `sashabaranov/go-openai` v1.43.0, `joho/godotenv` v1.5.1, `modernc.org/sqlite` v1.36.1, `alexedwards/scs/v2` v2.8.0 (v2.9.0 needs Go 1.23), `alexedwards/scs/sqlite3store`, `golang.org/x/crypto` v0.33.0.

## Where It Runs and How Someone Tries It

**Runtime:** local processes. The Go backend listens on `127.0.0.1:8080`; the Vite dev server runs on `:5174` (strict — `:5173` is used by another app on this machine) and proxies `/api/*` to `127.0.0.1:8080`. Always open `http://localhost:5174` (cookies are host-scoped). SQLite is a single file at `backend/porchlight.db`.

**Environment requirements:**
- Go 1.22 or newer (`go version` to check)
- Node.js 20+ (`node --version` to check)
- An OpenRouter API key (set as `OPENROUTER_API_KEY` in `backend/.env`)
- A Brave Search API key (set as `BRAVE_API_KEY` in `backend/.env`)
- A `.env.example` is committed; `.env` is gitignored.

**Start command (from repo root):**
```
# Terminal 1
cd backend
make run          # = CGO_ENABLED=0 GOTOOLCHAIN=local go run .

# Terminal 2
cd frontend
npm install
npm run dev
```

**Open in browser:** `http://localhost:5174`. Sign up, create family, land on the homepage.

**Submission:** the demo video is a 60-second screen recording of (1) setup, (2) drag-and-drop rearrange in Puck, (3) typing "show me upcoming SpaceX launches" and watching the new widget appear. Public GitHub repo with all the planning docs in `devpost/`. Deployment is **not** required and is **not** in scope.

## Look and Feel

Carried forward from `prd.md > Look and Feel` and `scope.md > Inspiration & Identity`, in terms the build can act on:

- **Typography:** Gambarino (Fontshare) for display and headings; Switzer (Fontshare) for body. Both loaded via `<link>` in `index.html`; system stack fallback if Fontshare is offline.
- **Palette:** soft warm neutrals for surfaces (`bg: #fafaf7`, surfaces `#ffffff`), one calm primary (`#4f46e5` indigo) for UI, one warm accent (`#f59e0b` amber) for the kernel moments (the "+" button, the thinking state, the freshly-arrived widget's border).
- **Density:** generous whitespace, large widget card padding, soft shadows. Widget cards feel like cards on a kitchen table, not cells in a spreadsheet.
- **Motion:** Puck's built-in drag-and-drop animations; the kernel's "thinking" state uses a calm spinner; the new widget fades + slides in on `puck.dispatch`.
- **Tone of copy:** warm, plain language. "Your page" not "Dashboard." "Add a widget" not "Create block." Errors say what to do next.
- **References:** OpenUI / OpenIntelligentUI for the AI-generates-UI feel.

## Components

### Backend (Go)

#### `main.go` — entrypoint
Wires the router, sessions, store, and handlers. Reads env, opens SQLite, starts the server.
PRD ref: implements all behaviors.

#### `internal/store` — SQLite access
Wraps the SQLite driver. Exposes typed methods per table: `GetUser`, `CreateUser`, `GetFamily`, `CreateInvite`, `GetMemberProfile`, `UpsertMemberProfile`, `GetLayout`, `UpsertLayout`, `CreateCustomWidget`, `GetCustomWidget`, `ListCustomWidgets`.
PRD ref: implements `prd.md > Data Model`.

#### `internal/auth` — sessions, passwords, middleware
- `HashPassword(plain) ([]byte, error)` and `CheckPassword(plain, hash) bool` using bcrypt.
- `RequireAuth` middleware: reads the session cookie, looks up the user, attaches to request context.
- `RequireAdmin` middleware: checks `is_admin` on the user.
PRD ref: implements `prd.md > Authentication and Account`.

#### `internal/family` — family + member management handlers
- POST `/api/families` — create the family on first signup.
- GET `/api/me/family` — the sidebar's data: members (admins see everyone; a kid sees only themselves) and, for admins, pending invites with their links.
- POST `/api/families/:id/invites` — issue an invite (admin only; `role` parent or kid; "Already a member" for existing members; re-inviting returns the same link).
- DELETE `/api/families/:id/invites/:inviteId` — revoke a pending invite (admin only).
- GET `/api/invites/:token` — public lookup for the set-password page (404 unknown or revoked, 410 expired or used).
- POST `/api/invites/:token/accept` — accept an invite, set password, create user.
PRD ref: implements `prd.md > Family Management`.

#### `internal/layout` — Puck layout persistence
- GET `/api/me/homepage` — returns `{profile, layout (Puck data), customWidgets}` for the logged-in user.
- PUT `/api/me/homepage` — accepts new Puck data, upserts the layout row.
- PUT `/api/me/profile` — accepts new interests/location.
PRD ref: implements `prd.md > Drag-and-Drop Layout` and the persistence parts of `prd.md > Homepage and Pre-Built Widgets`.

#### `internal/widgets` — pre-built and custom widget handlers
- GET `/api/widgets/articles` — returns 3–5 articles for the logged-in user.
- GET `/api/widgets/events` — returns 3–5 local events.
- POST `/api/widgets/from-prompt` (the **kernel**) — accepts `{prompt: string}`, calls OpenRouter + Brave Search, persists a `custom_widgets` row, returns the new block spec.
- GET `/api/widgets/custom/:id` — renders a custom widget (called by the frontend on initial load; may be re-rendered on refresh).
- DELETE `/api/widgets/custom/:id` — permanently deletes one of your custom widgets and its card (the Arrange mode "Remove" action).
PRD ref: implements `prd.md > Articles Widget`, `prd.md > Local Events Widget`, and `prd.md > Custom Widgets ("Describe-a-Widget")`.

#### `internal/sources` — data source registry
Interface-driven so v2 can add PredictHQ/Google Events with a config change:

```go
type Source interface {
    Name() string  // "news_search", "events_search", "web_search"
    Search(ctx, query string) ([]Result, error)
    Shape() string // hints the LLM what kind of results this source returns
}
```

A `Registry` maps names to implementations. v1 ships with one implementation: `BraveSource`, which calls the Brave Search API and adapts results to a uniform `[]Result` shape (`{title, url, snippet, date?}`).
PRD ref: implements `prd.md > Data Sources`.

#### `internal/llm` — OpenRouter client + prompts
- `NewClient(apiKey, model) *Client` — wraps `sashabaranov/go-openai` pointed at `https://openrouter.ai/api/v1`.
- `PickWidgetSpec(ctx, prompt string, availableSources []string) (*WidgetSpec, error)` — calls the LLM with a strict structured output `{title, source, query, archetype}`, decoded and validated in Go.
- `ShapeItems(ctx, prompt, spec, results)` — turns search results into up to 8 fixed-shape items (`{title, url, snippet, date, label, value, thumbnail}`) grounded in those results. The model also returns a machine-readable `sort_date` per item and a `future_only` flag; for "upcoming" requests, Go (not the model) drops anything dated before today.
- Handles OpenRouter's quirks: an HTTP 200 carrying an error with no choices, `finish_reason: "length"`, and 401/402/429/5xx mapped to clear errors.
- `CurateArticles(ctx, interests string) (*ArticlesSpec, error)` and `CurateEvents(ctx, interests, location string) (*EventsSpec, error)` — for the pre-built widgets. Constrained to a known shape.
PRD ref: implements the LLM parts of `prd.md > Articles Widget`, `prd.md > Local Events Widget`, and the entire `prd.md > Custom Widgets ("Describe-a-Widget")` flow.

### Frontend (React + Puck)

#### Routes
- `/login`, `/signup` — auth screens.
- `/invite/:token` — set-password-then-homepage.
- `/create-family` — the create-family intro.
- `/` — the homepage, Puck-powered.
PRD ref: implements `prd.md > Screens and Layout`.

#### Puck configuration
- Lives in `frontend/src/puck/config.tsx`.
- Defines custom block types: `Articles`, `LocalEvents`, `Custom`.
- Each block has a `render` function (used in view mode) and a `fields` schema (used in edit mode for any per-block settings; for v1, most fields are empty — the data is fetched on render).
- The block library shows `Articles`, `LocalEvents`, and `Custom`. Clicking `Custom` opens the "describe a widget" prompt.
- **Arrange toggle (learner's choice):** normally the page renders the saved layout read-only, so links and buttons inside widgets work. "Arrange" mounts the Puck editor (composition mode: `<Puck iframe={{enabled:false}}>` with only `<Puck.Preview/>` and our own toolbar, no default editor chrome) for drag-to-reorder and right-edge resize; "Done" returns to the normal page. Blocks live in a root slot field `blocks` rendered as a 4-column CSS grid; each block is `inline: true` with `puck.dragRef` and a `span` prop of 1–4.
PRD ref: implements `prd.md > Drag-and-Drop Layout` and the canvas part of `prd.md > Custom Widgets`.

#### Custom blocks
- `Articles.tsx` — fetches `/api/widgets/articles`, renders a list of `{title, snippet, source}` rows.
- `LocalEvents.tsx` — fetches `/api/widgets/events`, renders a list of `{name, date, venue, snippet}` rows.
- `Custom.tsx` — receives a `widgetId` prop; fetches `/api/widgets/custom/:id`, renders the `render` archetype.
PRD ref: implements the per-widget behavior in `prd.md > Features and Behavior`.

#### The "describe-a-widget" prompt modal
- Triggered from the Puck block library (or a "+" button next to the canvas).
- Single text input with example placeholders.
- Submit → POST `/api/widgets/from-prompt` → on success, calls `puck.dispatch({type: "insert", block: newBlock})`.
- "Thinking" state disables the input and shows a spinner.
- On failure: inline "Try again" button.
PRD ref: implements the kernel moment in `prd.md > Custom Widgets ("Describe-a-Widget")`.

## Data Model

SQLite, file at `backend/porchlight.db`. Schema:

```sql
-- One row per user. Family membership is via family_id.
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash BLOB NOT NULL,
  family_id     INTEGER NOT NULL REFERENCES families(id),
  is_admin      BOOLEAN NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The family unit.
CREATE TABLE families (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pending invites. Token is the URL-safe secret in the invite link.
-- is_admin marks a co-parent invite (from the create-family intro or the sidebar).
CREATE TABLE invites (
  id          INTEGER PRIMARY KEY,
  family_id   INTEGER NOT NULL REFERENCES families(id),
  email       TEXT NOT NULL,
  token       TEXT NOT NULL UNIQUE,
  is_admin    BOOLEAN NOT NULL DEFAULT 0,
  expires_at  DATETIME NOT NULL,
  accepted_at DATETIME,
  revoked_at  DATETIME   -- added by migrations/002: revoking marks the row instead of deleting it
);

-- Server-side sessions for alexedwards/scs (sqlite3store expects this exact shape).
CREATE TABLE sessions (
  token  TEXT PRIMARY KEY,
  data   BLOB NOT NULL,
  expiry REAL NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions(expiry);

-- Per-user profile data: interests (freeform) and location (freeform).
CREATE TABLE member_profiles (
  user_id    INTEGER PRIMARY KEY REFERENCES users(id),
  interests  TEXT NOT NULL DEFAULT '',
  location   TEXT NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Per-user homepage layout. `puck_data` is the JSON tree Puck produces.
CREATE TABLE homepage_layouts (
  user_id    INTEGER PRIMARY KEY REFERENCES users(id),
  puck_data  TEXT NOT NULL,  -- JSON
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User-described widgets. `render_spec` is the LLM's chosen shape + content.
-- `source` and `query` are stored so we can re-fetch on demand.
CREATE TABLE custom_widgets (
  id          INTEGER PRIMARY KEY,
  user_id     INTEGER NOT NULL REFERENCES users(id),
  name        TEXT NOT NULL,
  source      TEXT NOT NULL,        -- "news_search" | "events_search" | "web_search"
  query       TEXT NOT NULL,
  render_spec TEXT NOT NULL,        -- JSON
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**State and where things live:**
- The Go server is the single source of truth.
- Sessions are server-side (SQLite via `scs`); the client only holds an httpOnly cookie.
- Puck's `Data` is JSON; we persist it as TEXT in SQLite. The shape:
  ```
  {
    "content": [
      { "type": "Articles", "props": {} },
      { "type": "LocalEvents", "props": {} },
      { "type": "Custom", "props": { "widgetId": 7 } }
    ],
    "root": { "props": {} }
  }
  ```
- Pre-built widget content (Articles, Local Events) is **not persisted** — recomputed on every page load via LLM + Brave Search.
- Custom widget content is persisted (the LLM's `render_spec` and the `source`/`query`); re-fetching from Brave Search is a future enhancement.

## File Structure

```
build-w-ai-basics-hackathon/
├── devpost/                       # planning docs (already exists)
│   ├── learner-profile.md
│   ├── scope.md
│   ├── scope.html
│   ├── prd.md
│   └── prd.html
├── backend/                       # Go API
│   ├── .env.example
│   ├── Makefile                   # run/build/vet/test with CGO_ENABLED=0 GOTOOLCHAIN=local
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                    # entrypoint
│   ├── internal/
│   │   ├── auth/                  # sessions, password hashing, middleware
│   │   ├── family/                # family + invites handlers
│   │   ├── layout/                # Puck layout persistence handlers
│   │   ├── widgets/               # articles / events / custom widget handlers
│   │   ├── sources/               # Source interface + BraveSource + Registry
│   │   ├── llm/                   # OpenRouter client + prompt templates
│   │   └── store/                 # SQLite access (one file: store.go)
│   └── migrations/
│       └── 001_init.sql           # the schema above
├── frontend/                      # React SPA
│   ├── package.json
│   ├── vite.config.ts             # port 5174 (strict), proxies /api to 127.0.0.1:8080, Tailwind v4 plugin
│   ├── tsconfig.json
│   ├── index.html                 # loads Gambarino + Switzer from Fontshare
│   └── src/
│       ├── main.tsx
│       ├── routes/                # TanStack Router file-based routes
│       │   ├── __root.tsx
│       │   ├── login.tsx
│       │   ├── signup.tsx
│       │   ├── invite.$token.tsx
│       │   ├── create-family.tsx
│       │   └── index.tsx          # the homepage
│       ├── components/            # shared UI (Button, Input, Modal, etc.)
│       ├── puck/
│       │   ├── config.tsx         # Puck block types: Articles, LocalEvents, Custom
│       │   └── blocks/
│       │       ├── Articles.tsx
│       │       ├── LocalEvents.tsx
│       │       └── Custom.tsx
│       ├── widgets/
│       │   ├── DescribeWidgetPrompt.tsx  # the kernel prompt (inline)
│       │   └── WidgetCard.tsx            # renders the 5 archetypes; reused by Puck blocks
│       ├── api/                   # typed API client + TanStack Query hooks
│       │   ├── client.ts
│       │   └── hooks.ts
│       └── styles/
│           └── index.css          # tailwind directives + a few custom utilities
├── README.md                      # how to run, how to record the demo
├── SPEC.md → see devpost/spec.md  # pointer
└── .gitignore                     # includes /devpost/learner-profile.md
```

## External Services and Dependencies

### OpenRouter (LLM proxy)
- **Endpoint:** `https://openrouter.ai/api/v1/chat/completions` (OpenAI-compatible)
- **Auth:** `Authorization: Bearer $OPENROUTER_API_KEY`
- **Model:** `openai/gpt-4.1-mini` by default (cheaper alternative: `openai/gpt-4o-mini`; a free option with structured outputs: `nvidia/nemotron-3-super-120b-a12b:free`). Change via `OPENROUTER_MODEL`; the server verifies the id and its `structured_outputs` support against `GET /api/v1/models` at startup, and validates the key via `GET /api/v1/key`.
- **Library:** `github.com/sashabaranov/go-openai` pointed at OpenRouter's base URL.
- **Output format:** strict `json_schema` structured output, always decoded and validated in Go.
- **Rate limits / cost:** the user has $25 in credits. Free models are limited to 20 requests/minute and 1,000/day for accounts with purchased credits.
- **Doc:** https://openrouter.ai/docs

### Brave Search
- **Endpoints:** `https://api.search.brave.com/res/v1/web/search?q=...` and `https://api.search.brave.com/res/v1/news/search?q=...`
- **Auth:** `X-Subscription-Token: $BRAVE_API_KEY`
- **Plan:** the Search plan costs $5 per 1,000 requests with $5 of free credit each month (about 1,000 searches), 50 requests/second. A credit card is required. To protect the credit, the frontend doesn't refetch widgets on window focus, and the server doesn't test the Brave key at startup.
- **Doc:** https://brave.com/search/api/
- **Usage in v1:** `news_search` uses the news endpoint; `web_search` uses the web endpoint; `events_search` uses the web endpoint with a location- and date-anchored query (Brave has no events feed), and the LLM extracts event name, date, and venue from the results. The Brave result shapes are adapted to the unified `[]Result` shape by the `BraveSource` adapter. Never set `Accept-Encoding` manually from Go (it disables transparent gzip decoding).

### Environment variables
```
OPENROUTER_API_KEY=...     # required
OPENROUTER_MODEL=...       # optional, defaults to openai/gpt-4.1-mini
BRAVE_API_KEY=...          # required
PORT=8080                  # optional, defaults to 8080
```
(No `SESSION_SECRET`: scs sessions are random server-side tokens, not signed cookies. `OPENROUTER_BASE_URL` / `BRAVE_BASE_URL` exist only so tests can point at fake servers.)

## Important Failure Modes

- **OpenRouter call fails or returns malformed JSON** → the kernel prompt shows an inline "Try again" button; no widget is created. The Go side retries the JSON parse once before giving up.
- **Brave Search returns no results** → the widget shows a calm "Nothing new today — try refreshing later" message instead of empty space.
- **Brave Search rate-limit hit (HTTP 429)** → exponential backoff once, then surface the same "try again" treatment.
- **Session expired** → any 401 from the API triggers a redirect to `/login`. The React side has a single `fetch` wrapper that handles this.
- **LLM picks a data source we don't have** → the Go side validates the source name against the `Registry`; if unknown, it returns a 422 and the prompt shows "Try again" with a hint ("Couldn't pick a source for that prompt — try rephrasing").
- **LLM returns an unknown render archetype** → the frontend falls back to a generic "card" renderer that shows whatever content the LLM returned.
- **OpenRouter key missing or invalid** → startup fails fast with a clear error message in the console. Same for Brave.

## What Was Simplified and Why

- **One OpenRouter model at a time, configured via env var.** No per-widget model selection, no fallback chain. v1 is one model; swap by changing the env var.
- **No email sending for invites.** The admin sees the invite link in the sidebar and copies it manually. Email is a real cost and an external service; out of scope for the PoC.
- **No password reset.** Out of scope; the user can re-invite if needed.
- **No caching of pre-built widget results.** Articles and Events are recomputed on every page load. v1 cares about correctness over efficiency; caching is in `Possible Later Enhancements` of the PRD.
- **No HTTPS, no production deployment.** Local PoC only. The Go server is plain HTTP on `:8080`.
- **No multi-tenant or multi-family concerns.** The `families` table exists, but every user belongs to exactly one family in v1. Multi-family is a v2 product concern (per `scope.md`).
- **"Free-form" render spec is actually constrained free-form.** The LLM picks from 5 archetypes (`list`, `cards`, `hero`, `key_value`, `table`), and the content is a list of items in one fixed shape that every archetype renders from. Strict structured output can't describe truly free-form content, and truly open JSON would be research-grade. **See Decisions and Open Issues** for the call.
- **Light validation of LLM output.** Structured output plus a decode-and-validate step in Go (known source, known archetype, non-empty fields, item URLs that really came from the search results). No separate schema-validation library.
- **Reordering is Puck's; resizing is ours.** Puck handles drag-and-drop reordering. It has no resize, so the right-edge resize (snapping to quarters) is a small custom handle that updates the block's width through Puck's API.
- **No rate limiting.** Local PoC, one family. Add later if it goes public.

## Decisions and Open Issues

Decisions made in the spec interview:

- **Single-binary split architecture: Go API + React SPA.** Learner preference; matches the React/TanStack/Puck stack they want to use.
- **OpenRouter via `sashabaranov/go-openai`.** OpenRouter is OpenAI-compatible; this is the most-used Go client. User accepted.
- **Stdlib `net/http` for routing.** Most idiomatic for learning Go; zero deps.
- **`alexedwards/scs/v2` for sessions.** Server-side, SQLite-backed, httpOnly cookies.
- **`modernc.org/sqlite` for the driver.** Pure Go, no CGO requirement.
- **Puck for the canvas.** Learner preference; handles drag, drop, resize, and the layout data structure.
- **TanStack Router in SPA mode, not TanStack Start.** Start would overlap with the Go API.
- **Constrained free-form render spec.** LLM picks from 5 archetypes; content is free. See "What Was Simplified" for why not truly free-form.
- **OpenRouter model is configurable via env var.** Default TBD; user can pick a free model at runtime.

**One genuine learner uncertainty surfaced and addressed:** the Go LLM integration path. The user proposed `gollm`; the recommendation is `sashabaranov/go-openai` for OpenAI-compatibility, broader usage, and active maintenance. **To verify during the build:** that the chosen model on OpenRouter supports JSON-mode / structured output for the kernel prompt. If it doesn't, fall back to prompt-engineering for the JSON shape with a retry-on-parse-failure loop. (Most free models on OpenRouter do support it; this is a sanity-check during the first kernel run, not a blocker.)

**Open issues:**

- **Exact OpenRouter model.** Resolved: default `openai/gpt-4.1-mini`, verified at startup; swappable via `OPENROUTER_MODEL`.
- **Puck version pinning.** Resolved: `@puckeditor/core` 0.23.0, which supports React 18.
- **The "+" button placement in Puck.** Puck has a block library UI; we'll use that for adding `Custom` widgets, but we also need an entry point for the kernel prompt. The build will resolve the exact UX in the first pass.
- **"Switch view" sidebar interaction.** Resolved: the URL carries `/?as=<memberId>`, and parent-only routes `GET /api/members/{id}/homepage`, `GET /api/members/{id}/widgets/articles|events` and `PUT /api/members/{id}/profile` reuse the `/api/me` handlers through a "subject user" in the request context (`auth.RequireMember` / `auth.SubjectFrom`). Parents can view another member's page and edit their interests and location, but not their layout or widgets.

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
- **Lantern moments** (2026-10-25): the lantern behaves as `brand/lantern-ux.md` describes. It kindles when the app opens in a tab and after signing in. One breathing lantern, always beside words, replaces every spinner, pulsing placeholder and "Thinking…" line, and it's the only animation that repeats. A card that's new, or whose content changes during the visit, catches the light along its top edge while the header lantern swings. Every card's edge is warm after an update and fades over six hours, beside its update time. Changing the theme is dusk: night spreads out from the header lantern, or the light draws back in, through a view transition. At most one thing glows at full strength (a wait, then an arrival, then the morning briefing), and reduced motion turns every moment into a plain fade. Motion tokens sit beside the colors, and the rules are plain functions tested by `npm test` in CI. Specs: `openspec/specs/lantern-motion/`, with `widget-freshness` updated; change: `lantern-moments` in `openspec/changes/archive/`.
- **Brand surfaces** (2026-10-25): Kinlight gets its materials and voice from `brand/`. A brass hairline edges only the briefing card and the add-a-card button. Lantern glass (frosted, faintly warm) covers what floats over the page: the arrange toolbar, the card action bar and the chat composers. Wood grain and porch boards sit on the welcome pages, the sidebar and empty states, never behind content. A worst-case contrast check on glass and wood sets their strengths (95% glass, 7% seams). The porch illustration has slots on the welcome pages and in empty states, both themed, with a plain stand-in until a commissioned drawing replaces one file (`brand/illustration.md`). The voice: they're "cards", never widgets, in everything people read, front and back. Load errors are plain sentences, and parents see "Picked with Jordan's age in mind". `npm run check:copy` in CI holds the line. Specs: `app-identity` and `kid-safe-content` updated; change: `brand-surfaces` in `openspec/changes/archive/`.
