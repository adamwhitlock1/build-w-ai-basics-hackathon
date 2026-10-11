# Porchlight

Porchlight is a homepage for each member of a family where every widget is AI-driven. Type a sentence like "show me upcoming SpaceX launches" and a new widget appears, filled with live data. It has two halves that talk over HTTP. A **Go backend** is the only part that talks to the LLM (through OpenRouter) and to Brave Search. A **React frontend** runs in the browser and calls the Go API. When you describe a widget, the backend asks the LLM to choose a data source, a search query and a display shape. It then runs the search on Brave and asks the LLM to turn the results into widget items, using only what the search returned. Your API keys never leave the server.

> Status: slice 4 (accounts, family setup, widgets that are saved to you, Articles and Local Events cards that pick themselves from your interests and location, and a page you can arrange). Invites and the family sidebar come in later slices.

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

Nothing is cached, so the Articles and Local Events cards search again every time your page loads (or you press a card's refresh button): up to 5 Brave news searches for Articles and up to 3 web searches for Local Events, plus a few small LLM calls. That's at most 8 searches per visit.

Accounts, families, saved widgets and page layouts live in a SQLite file, `backend/porchlight.db` (gitignored), which the backend creates and sets up on first run. To start over, stop the backend and delete `backend/porchlight.db*`. Set `DB_PATH` to use a different file.

### Why `make run` and not `go run .`?

The Makefile sets `CGO_ENABLED=0` for every Go command. On this Mac, Go 1.22.0 binaries linked with cgo crash at launch with `dyld: missing LC_UUID`. Porchlight is pure Go, and so is its SQLite driver (modernc.org/sqlite), so turning cgo off costs nothing. The Makefile also sets `GOTOOLCHAIN=local` so Go never downloads a different toolchain. To run Go commands by hand, prefix them: `CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...`.

## Tests

```sh
cd backend
make check   # go vet + go test + go build
```

The tests replace OpenRouter and Brave with fakes, so they need no keys and spend no credit.
