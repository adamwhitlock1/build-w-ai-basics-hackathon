# Porchlight

Porchlight is a homepage for each member of a family where every widget is AI-driven. Type a sentence like "show me upcoming SpaceX launches" and a new widget appears, filled with live data. It has two halves that talk over HTTP. A **Go backend** is the only part that talks to the LLM (through OpenRouter) and to Brave Search. A **React frontend** runs in the browser and calls the Go API. When you describe a widget, the backend asks the LLM to choose a data source, a search query and a display shape. It then runs the search on Brave and asks the LLM to turn the results into widget items, using only what the search returned. Your API keys never leave the server.

> Status: slice 6 (accounts, family setup, widgets that are saved to you, Articles and Local Events cards that pick themselves from your interests and location, a page you can arrange, a family sidebar where parents invite the rest of the family, each of whom gets their own page, and parents switching to any member's page to set their interests).

## Prerequisites

- **Go 1.22+**: check with `go version`
- **Node.js 20.19+ or 22.12+** (what Vite 8 requires): check with `node --version`
- **An OpenRouter API key**: <https://openrouter.ai/settings/keys>
- **A Brave Search API key**: <https://api-dashboard.search.brave.com>. The Search plan asks for a card but includes about $5 of free credit each month, which is plenty for local use.

## Setup

```sh
cp backend/.env.example backend/.env
# then open backend/.env and fill in OPENROUTER_API_KEY and BRAVE_API_KEY
```

`backend/.env` is gitignored, so your keys stay on your machine.

## Run

```sh
# Terminal 1: the Go API on http://127.0.0.1:8080
cd backend
make run

# Terminal 2: the web app
cd frontend
npm install
npm run dev
```

Then open **<http://localhost:5174>**. Port 5173 is taken by another app on this machine, so Vite uses 5174 and proxies `/api` to the Go server.

On startup the backend checks that both keys are set, that your `OPENROUTER_MODEL` exists and supports structured outputs, and that OpenRouter accepts your key. If any check fails, it stops and prints what to fix. These checks cost no credit. The Brave key isn't checked at startup, because every Brave call uses credit.

To arrange your page, press **Arrange** at the top (it shows on screens at least tablet-wide). Drag a card to move it, or drag its right edge to make it 1/4 to 4/4 of the page wide (the width buttons above a selected card do the same). Changes save as you go; press **Done** (or Escape) to use the page again. A widget you described can also be deleted there, with **Remove**.

To invite your family, use **Invite member** in the sidebar (on a phone, open **Family** first): enter their email and choose **Parent** (can invite and manage) or **Kid**. Porchlight sends no email. Instead it shows a link to copy and send them however you like. The link uses the address you opened the app at (for example `http://localhost:5174/invite/...`), so they need to be able to reach that address. A link works for 14 days, and until someone uses it, it stays under **Pending invites**, where you can copy it again or revoke it. Co-parents you named while setting up your family are listed there too. Opening the link while logged out asks the new member to choose a password, and then they land on their own page.

To look at a family member's page, a parent picks them in the sidebar's member list (on a phone, under **Family**). Their page opens at `/?as=<their id>`: their cards in their own order, read-only, since arranging it and adding widgets is up to them. **Their interests** sets the interests and location their Articles and Local Events are picked from. Pick yourself, or press **Back to your page**, to return. Kids see only their own page; the server refuses anyone else's.

Widgets remember their results, so a page you've opened before shows its cards straight away, without searching. Every card says when it was last updated. Each kind stays fresh for a while: Articles for 3 hours, Local Events for 12, and widgets you described for 6. After that, the card still shows what it has and quietly fetches new results in the background. New interests or a new location pick that member's Articles and Local Events again at once. Every card's refresh button gets new results whenever you like, and widgets you described have one too. If a refresh fails, the card keeps its current items and offers "Try again".

What that costs: picking Articles takes up to 5 Brave news searches, Local Events up to 3 web searches, each with a few small LLM calls, and refreshing a described widget takes 1 search and 1 LLM call. A visit while everything is fresh spends nothing. Pressing refresh twice within a minute spends only once. A family member's cards are shared with their parents' view of them, so whoever opens them first pays, and the other gets them for free.

Accounts, families, saved widgets and page layouts live in a SQLite file, `backend/porchlight.db` (gitignored), which the backend creates and sets up on first run. To start over, stop the backend and delete `backend/porchlight.db*`. Set `DB_PATH` to use a different file.

### Why `make run` and not `go run .`?

The Makefile sets `CGO_ENABLED=0` for every Go command. On this Mac, Go 1.22.0 binaries linked with cgo crash at launch with `dyld: missing LC_UUID`. Porchlight is pure Go, and so is its SQLite driver (modernc.org/sqlite), so turning cgo off costs nothing. The Makefile also sets `GOTOOLCHAIN=local` so Go never downloads a different toolchain. To run Go commands by hand, prefix them: `CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...`.

## Data sources

- **[Brave Search](https://brave.com/search/api/)**: news, web and events searches. Needs `BRAVE_API_KEY`.
- **[Open-Meteo](https://open-meteo.com/)**: weather widgets (the place lookup and the 7-day forecast). It needs no key. Its free API is for non-commercial use only and its data is licensed [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), so every weather card says "Weather data by Open-Meteo.com". A commercial Porchlight would need Open-Meteo's paid plan.

## Tests

```sh
cd backend
make check   # go vet + go test + go build
```

The tests replace OpenRouter, Brave and Open-Meteo with fakes, so they need no keys, spend no credit and make no network calls.

## How it was planned

The proof of concept was planned and built with the [Devpost Learn skill pack](https://github.com/challengepost/learn-ai-basics). Its planning documents are in `devpost/`: `scope.md`, `prd.md`, `spec.md` and the build `checklist.md`. The git tag `poc-complete` marks the finished proof of concept.

Features added after that are planned with [OpenSpec](https://github.com/Fission-AI/OpenSpec), in `openspec/`, so the original planning documents stay as they were. Each feature starts as a proposal in `openspec/changes/` and, once built, is archived into `openspec/specs/`. In Claude Code, `/opsx:propose` starts one, `/opsx:apply` builds it and `/opsx:archive` finishes it.
