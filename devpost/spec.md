---
doc: spec
status: approved
---
# Porchlight — Technical Spec

## How This Works, In Plain Language

Porchlight is two halves that talk to each other over HTTP:

- **A Go backend** is a single program that runs on your computer. It owns the database (a small SQLite file), handles logins, and is the only thing that talks to the LLM (via OpenRouter) and to Brave Search. It speaks JSON over a REST API.
- **A React frontend** is a single-page app that runs in a browser. It uses **Puck** to render and edit the homepage canvas, and TanStack Query to fetch data from the Go API. The user logs in, the React app asks the Go API for the family's data, and Puck renders the homepage using the user's saved layout.

The **kernel — "describe a widget"** — works like this: the user clicks the "+" button in Puck, types a sentence, the React app POSTs that sentence to the Go API, the Go API calls OpenRouter with a structured-output prompt, the LLM returns a small JSON spec (which data source to use, what to search for, and how to render the result), the Go API calls Brave Search to fetch the data, persists the new widget, and returns the rendered spec. Puck adds the new widget to the page. The whole loop is server-mediated — the LLM is never called from the browser, so the OpenRouter key stays on the server.

**Why this shape:** it's a clean separation of concerns. The Go side is pure API + database + LLM proxy, which is exactly the work the learner wants to learn. The React side is a thin Puck-powered canvas that talks to the Go API. Both halves are sized for a single afternoon.

## The Core Journey Through the System

PRD ref: `prd.md > The Core Journey`. Walking the 8 steps through the architecture:

1. **Sign up** — POST `/api/signup` with email + password. Go hashes the password (bcrypt), creates a `users` row, creates a `families` row, sets the user as admin.
2. **Create family** — POST `/api/families` with family name, co-admin emails, parent's interests, parent's location. Go creates a `member_profiles` row for the parent and a `homepage_layouts` row with a default Puck data structure that has the two pre-seeded widget blocks plus empty slots.
3. **Land on homepage** — GET `/api/me/homepage`. Go returns the user's Puck data + their member profile. React renders the homepage with Puck in view mode.
4. **Invite family** — POST `/api/families/:id/invites` with the invitee's email. Go creates an `invites` row with a random token, returns the invite URL. (Email sending is out of scope; the link is shown in the sidebar for the admin to copy/share.)
5. **Each opens theirs** — invitee clicks the link → POST `/api/invites/:token/accept` with their chosen password. Go creates the user, links them to the family, and creates a `member_profiles` row. They land on their own homepage, which fetches the same way as step 3.
6. **Rearrange** — Puck's drag-and-drop reorders the blocks; on drop, Puck emits a `data` change event. The React app debounces and POSTs the new Puck data to PUT `/api/me/homepage`. Go persists to `homepage_layouts`.
7. **Describe-a-widget (kernel)** — User clicks "+" in Puck's block library → React opens the prompt modal. User types a sentence and submits. React POSTs to `/api/widgets/from-prompt`. Go:
   - Calls OpenRouter (via `sashabaranov/go-openai`, base URL pointed at OpenRouter) with a system prompt that constrains the LLM to return JSON: `{source: "news_search" | "events_search" | "web_search", query: string, render: {archetype: "list" | "cards" | "hero" | "key_value" | "table", ...content}}`.
   - Calls Brave Search via the chosen source.
   - Persists a `custom_widgets` row with the render spec + the fetched data (or just the query, for re-fetching).
   - Returns the new block spec to the React app.
   - React calls `puck.dispatch` to add the block to the page.
8. **Come back tomorrow** — the layout, custom widgets, interests, and location are all in SQLite. The pre-built Articles and Local Events widgets are recomputed on each page load (LLM + Brave Search) — no caching for v1.

## Stack

| Layer | Choice | Why |
|---|---|---|
| **Backend language** | Go 1.22+ | Stated learning goal; idiomatic for APIs. |
| **HTTP router** | Stdlib `net/http` (Go 1.22+ ServeMux with patterns) | Zero deps, most idiomatic for learning Go. Supports path params, methods, wildcards. Middleware written as small wrappers. |
| **Sessions** | `alexedwards/scs/v2` (SQLite-backed) | Server-side sessions, httpOnly cookies, no JWTs. |
| **SQLite driver** | `modernc.org/sqlite` (pure Go) | No CGO toolchain required; builds anywhere. |
| **LLM client** | `sashabaranov/go-openai` (pointed at OpenRouter's OpenAI-compatible endpoint) | Most-used Go OpenAI client; OpenRouter is OpenAI-compatible. |
| **External search** | Brave Search API (direct HTTP) | Single backing service for `news_search`, `events_search`, `web_search`; abstracted behind a registry. |
| **Frontend framework** | React 18 + TypeScript | Learner's preference. |
| **Build tool** | Vite | Fast, well-supported. |
| **Routing** | TanStack Router (SPA mode) | Learner's preference. **Not** TanStack Start — Start is full-stack and would overlap with the Go API. |
| **Server state** | TanStack Query | For data fetching, caching, optimistic updates. |
| **Styling** | Tailwind CSS (CDN) | Speed of iteration for a PoC; learner's preference. Gambarino + Switzer loaded from Fontshare with system fallback. |
| **Canvas** | Puck (latest 0.x) | Learner's preference; gives drag-and-drop, resize, and the layout data structure. |
| **Password hashing** | `golang.org/x/crypto/bcrypt` | Stdlib-quality, idiomatic. |
| **JSON Schema validation** | none for v1 | Trust the LLM's structured output; the system prompt constrains the shape. |

**OpenRouter model:** TBD. Free models include NVIDIA Nemotron Ultra and others; the user can pick. Configured in a single `OPENROUTER_MODEL` env var.

## Where It Runs and How Someone Tries It

**Runtime:** local processes. The Go backend listens on `:8080`; the Vite dev server runs on `:5173` and proxies `/api/*` to `:8080`. SQLite is a single file at `backend/porchlight.db`.

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
go run .

# Terminal 2
cd frontend
npm install
npm run dev
```

**Open in browser:** `http://localhost:5173`. Sign up, create family, land on the homepage.

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

#### `cmd/server` — entrypoint
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
- POST `/api/families/:id/invites` — issue an invite (admin only).
- POST `/api/invites/:token/accept` — accept an invite, set password, create user.
- GET `/api/me/family` — list family members (admin sees all; member sees self).
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
- `PickWidgetSpec(ctx, prompt string, availableSources []string) (*WidgetSpec, error)` — calls the LLM with a system prompt that constrains the output to `{source, query, render: {archetype, ...content}}`. Uses OpenAI's JSON-mode / structured output.
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
CREATE TABLE invites (
  id          INTEGER PRIMARY KEY,
  family_id   INTEGER NOT NULL REFERENCES families(id),
  email       TEXT NOT NULL,
  token       TEXT NOT NULL UNIQUE,
  expires_at  DATETIME NOT NULL,
  accepted_at DATETIME
);

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
│   ├── vite.config.ts             # proxies /api to :8080
│   ├── tsconfig.json
│   ├── tailwind.config.js
│   ├── postcss.config.js
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
│       │   └── DescribeWidgetPrompt.tsx  # the kernel modal
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
- **Model:** TBD — free options include `nvidia/nemotron-ultra-253b-v1:free` and others; user can change via `OPENROUTER_MODEL` env var.
- **Library:** `github.com/sashabaranov/go-openai` pointed at OpenRouter's base URL.
- **Output format:** JSON-mode for the kernel prompt (constrains LLM to return parseable JSON matching the widget spec shape).
- **Rate limits / cost:** the user has $25 in credits. Free models have their own rate limits (check OpenRouter's docs during build).
- **Doc:** https://openrouter.ai/docs

### Brave Search
- **Endpoint:** `https://api.search.brave.com/res/v1/web/search?q=...`
- **Auth:** `X-Subscription-Token: $BRAVE_API_KEY`
- **Free tier:** 2,000 queries/month, 1 query/second. Plenty for a local PoC.
- **Doc:** https://brave.com/search/api/
- **Usage in v1:** all three sources (`news_search`, `events_search`, `web_search`) call the same endpoint with different `q` and optional `result_filter` parameters. The Brave result shape is adapted to the unified `[]Result` shape by the `BraveSource` adapter.

### Environment variables
```
OPENROUTER_API_KEY=...     # required
OPENROUTER_MODEL=...       # optional, defaults to a sensible free model
BRAVE_API_KEY=...          # required
SESSION_SECRET=...         # random 32-byte hex; scs signs cookies with it
PORT=8080                  # optional, defaults to 8080
```

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
- **"Free-form" render spec is actually constrained free-form.** The LLM picks from 5 archetypes (`list`, `cards`, `hero`, `key_value`, `table`); the *content* within each is free. Truly open JSON would be research-grade. **See Decisions and Open Issues** for the call.
- **No JSON Schema validation of LLM output.** We trust the system prompt + JSON-mode for v1. Adding a schema validator is one line of work; doing it well is more.
- **Drag-and-drop is Puck's, not custom.** The PRD's "drag-and-drop layout" is fully delegated to Puck. We don't write any custom DnD code.
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

- **Exact OpenRouter model.** Pick one (or leave the default) at first run. The `OPENROUTER_MODEL` env var makes this trivially swappable.
- **Puck version pinning.** Latest 0.x; we should pin to a specific version once we know which one works smoothly with React 18 + TanStack Router.
- **The "+" button placement in Puck.** Puck has a block library UI; we'll use that for adding `Custom` widgets, but we also need an entry point for the kernel prompt. The build will resolve the exact UX in the first pass.
- **"Switch view" sidebar interaction.** PRD says the parent can switch to any member's view. The simplest implementation is a URL change (`/?as=spouse-id`) and a context switch on the Go side. We'll know quickly if there's a cleaner way during the build.
