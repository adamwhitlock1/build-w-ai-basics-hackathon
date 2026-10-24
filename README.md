# Porchlight

Porchlight is a homepage for each member of a family where every widget is AI-driven. Type a sentence like "show me upcoming SpaceX launches" and a new widget appears, filled with live data. It has two halves that talk over HTTP. A **Go backend** is the only part that talks to the LLM (through OpenRouter) and to Brave Search. A **React frontend** runs in the browser and calls the Go API. When you describe a widget, the backend asks the LLM to choose a data source, a search query and a display shape. It then runs the search on Brave and asks the LLM to turn the results into widget items, using only what the search returned. Your API keys never leave the server.

> Status: slice 6 (accounts, family setup, widgets that are saved to you, Articles and Local Events cards that pick themselves from your interests and location, a page you can arrange, a family sidebar where parents invite the rest of the family, each of whom gets their own page, and parents switching to any member's page to set their interests).

## Prerequisites

- **Go 1.27+**: check with `go version`. On a Mac, install it with the official installer from <https://go.dev/dl/> (it goes in `/usr/local/go`). Don't also install Homebrew's `go`: `/opt/homebrew/bin` usually comes first in `PATH`, so the two would silently compete.
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

The Makefile sets two things for every Go command:

- **`CGO_ENABLED=0`:** Porchlight is pure Go, and so is its SQLite driver (modernc.org/sqlite), so it builds without C. The result is one static binary that also cross-compiles to Linux for a home server (`GOOS=linux make build`). It was first turned off because Go 1.22.0 binaries built with cgo crashed on this Mac. Go 1.24+ fixed that, so it's a deliberate choice now.
- **`GOTOOLCHAIN=local`:** Go always uses the installed toolchain and never downloads one. If `go.mod` ever needs a newer Go than you have, the build stops and says so.

To run Go commands by hand, prefix them: `CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...`.

## Data sources

- **[Brave Search](https://brave.com/search/api/)**: news, web and events searches. Needs `BRAVE_API_KEY`.
- **[Open-Meteo](https://open-meteo.com/)**: weather widgets (the place lookup and the 7-day forecast). It needs no key. Its free API is for non-commercial use only and its data is licensed [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), so every weather card says "Weather data by Open-Meteo.com". A commercial Porchlight would need Open-Meteo's paid plan.

## Local test accounts

For trying things out locally, `make seed` (run in `backend/`) adds a test family to your database:

| Who | Email | Password |
| --- | --- | --- |
| Parent | `contact.codestuff@gmail.com` | `password1` |
| Kid | `contact.codestuff+kid@gmail.com` | `password1` |

It writes to `porchlight.db`, or to `$DB_PATH`, or to another file with `go run ./cmd/seed -db <file>`. Running it again changes nothing. These credentials are public in this repository, so it's for local databases only: never run it against a deployed one.

## Demo family

For testing every member's path end to end, `make demo` builds a whole family, plus a second one, in its own database, `backend/porchlight.db.demo` (gitignored). It deletes that file first, so every run starts from the same place. It never touches `porchlight.db`, and it refuses to write into any other file that already has accounts.

```sh
# Terminal 1: rebuild the demo, then serve it with fixture calendars on
cd backend
make demo
make demo-serve

# Terminal 2: the web app, as usual
cd frontend
npm run dev
```

Every account's password is `password1`.

| Account | Who | What it tests |
| --- | --- | --- |
| `maya@rivera.test` | Parent, created the family | Interests, location, calendar, three described widgets (one weather), a custom page arrangement |
| `sam@rivera.test` | Parent | Different interests, the default page, a calendar with a moved meeting |
| `jordan@rivera.test` | Kid (~15) | Kid-safe cards and widget, a calendar with a deleted practice |
| `lily@rivera.test` | Kid (~9) | Interests and location but no calendar: the "Add your calendar" path |
| `grandma@rivera.test` | Invited as a parent | The pending invites list, and joining: `make demo` prints the invite link |
| `alex@chen.test`, `riley@chen.test` | Parent and kid in a second family | That one family can't see another's members, pages or widgets |

`.test` addresses can never reach a real inbox. The data lives in `backend/cmd/seed/demo.go`, as a plain Go list you can add members to.

**Fixture calendars.** The demo members' calendar links point at `https://fixtures.porchlight.test/calendars/<name>.ics`. With `PORCHLIGHT_DEV_FIXTURES=1` (which `make demo-serve` sets), the server answers those links itself, with a feed built fresh around the current time. Between them, the feeds always have:

- an event today that has already ended, and one still to come;
- an all-day event;
- events tomorrow evening, which show under "Tomorrow" after 6 pm;
- a daily standup with today's moved from 9:00 to 11:00 (Sam);
- a daily band practice with today's deleted (Jordan).

You can paste a fixture link into anyone's "Your interests", for example `https://fixtures.porchlight.test/calendars/maya.ics` for Lily. Without the setting, `.test` never resolves, so those links just show as unavailable. Every other link is fetched exactly as usual either way. Never turn the setting on in a deployment.

Two limits come with building feeds around "now". Between about 11 pm and 1 am, the "already ended" or "still to come" event may not fit inside today and is left out. And "today" is the server's date, which only matters if the server and browser are in different time zones.

**Real calendars instead.** To try a real feed, set `SEED_CALENDAR_MAYA`, `SEED_CALENDAR_SAM`, `SEED_CALENDAR_JORDAN` or `SEED_CALENDAR_ALEX` in `backend/.env` to a private iCal link and run `make demo` again. That member gets the real link instead of the fixture. The link goes only into `porchlight.db.demo`, and `make demo` never prints it.

**What browsing the demo costs.** `make demo` fills every member's Articles, "Happening near you" and described widgets with realistic results, marked as just updated. Their links go to real home pages; refreshing a card replaces them with real results. While they're fresh, opening any page as anyone runs no Brave search and no AI call. Two things still cost:

- **The morning briefing:** it's written on each member's first visit of the day, with one small AI call.
- **Stale results:** once a result passes its freshness window (Articles after 3 hours, for example), or someone presses refresh, that card searches again and costs credit as usual.

Run `make demo` again to get back to a free, fresh start.

## Calendars

The Today card at the top of each page lists that day's events from a calendar link the member (or a parent, for them) pastes into "Your interests":

- **Google Calendar:** the calendar's settings → "Integrate calendar" → "Secret address in iCal format".
- **iCloud:** share the calendar as a public calendar and copy its `webcal://` link.

A calendar link is a secret: anyone who has it can read the calendar. Porchlight keeps it on the server, never sends it back to a browser or to the AI, and never logs it. The page only learns that a calendar is connected and which service it's from. The server fetches links over `https` only, and never from private or local network addresses.

Google can take a few hours to publish changes to its iCal link, so a newly added event may not show up straight away.

## Morning briefing

A card at the top of each page greets the member ("Good morning, Maya") and sums up their day in a few sentences the AI writes. It draws on:

- today's forecast for their saved location (tomorrow's, from 6 pm), from Open-Meteo;
- their Today card's events;
- a few headlines from their Articles;
- a few events from "Happening near you".

It never runs a search of its own. The page waits for those cards to load, then asks for the briefing.

The briefing is written once on the member's first visit of the day, and rewritten only when what it's built from changes. Rewrites are at least 10 minutes apart, and there are at most 4 AI calls per member per day. Kids' briefings use the same under-18 guidance as their other cards. The AI is never sent a name, an email address, a location or a calendar link.

## Tests

```sh
cd backend
make check   # go vet + go test + govulncheck + go build
```

`make check` also runs `govulncheck` (pinned in `go.mod` as a tool), which fails if Porchlight's code can reach a known vulnerability in Go or a dependency. It downloads the vulnerability list from vuln.go.dev, so it needs network; `make test` and `make vet` work offline. The race detector works too: `CGO_ENABLED=0 GOTOOLCHAIN=local go test -race ./...`.

The tests replace OpenRouter, Brave, Open-Meteo and calendar feeds with fakes (calendar tests read sample feeds in `backend/internal/calendar/testdata/`), so they need no keys, spend no credit and make no network calls.

## How it was planned

The proof of concept was planned and built with the [Devpost Learn skill pack](https://github.com/challengepost/learn-ai-basics). Its planning documents are in `devpost/`: `scope.md`, `prd.md`, `spec.md` and the build `checklist.md`. The git tag `poc-complete` marks the finished proof of concept.

Features added after that are planned with [OpenSpec](https://github.com/Fission-AI/OpenSpec), in `openspec/`, so the original planning documents stay as they were. Each feature starts as a proposal in `openspec/changes/` and, once built, is archived into `openspec/specs/`. In Claude Code, `/opsx:propose` starts one, `/opsx:apply` builds it and `/opsx:archive` finishes it.
