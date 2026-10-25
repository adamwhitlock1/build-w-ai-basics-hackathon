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

Accounts, families, saved widgets and page layouts live in a SQLite file, `backend/kinlight.db` (gitignored), which the backend creates and sets up on first run. To start over, stop the backend and delete `backend/kinlight.db*`. Set `DB_PATH` to use a different file.

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

It writes to `kinlight.db`, or to `$DB_PATH`, or to another file with `go run ./cmd/seed -db <file>`. Running it again changes nothing. These credentials are public in this repository, so it's for local databases only: never run it against a deployed one.

## Demo family

For testing every member's path end to end, `make demo` builds a whole family, plus a second one, in its own database, `backend/kinlight.db.demo` (gitignored). It deletes that file first, so every run starts from the same place. It never touches `kinlight.db`, and it refuses to write into any other file that already has accounts.

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
| `jordan@rivera.test` | Kid (15) | Kid-safe cards and widget, a calendar with a deleted practice, two sample chats (a homework one showing the hint ladder, and an Explore one) |
| `lily@rivera.test` | Kid (9) | Interests and location but no calendar: the "Add your calendar" path; one sample chat with a flag for parents to review |
| `grandma@rivera.test` | Invited as a parent | The pending invites list, and joining: `make demo` prints the invite link |
| `alex@chen.test`, `riley@chen.test` | Parent and kid in a second family | That one family can't see another's members, pages or widgets |

`.test` addresses can never reach a real inbox. The data lives in `backend/cmd/seed/demo.go`, as a plain Go list you can add members to.

**Fixture calendars.** The demo members' calendar links point at `https://fixtures.kinlight.test/calendars/<name>.ics`. With `KINLIGHT_DEV_FIXTURES=1` (which `make demo-serve` sets), the server answers those links itself, with a feed built fresh around the current time. Between them, the feeds always have:

- an event today that has already ended, and one still to come;
- an all-day event;
- events tomorrow evening, which show under "Tomorrow" after 6 pm;
- a daily standup with today's moved from 9:00 to 11:00 (Sam);
- a daily band practice with today's deleted (Jordan).

You can paste a fixture link into anyone's "Your interests", for example `https://fixtures.kinlight.test/calendars/maya.ics` for Lily. Without the setting, `.test` never resolves, so those links just show as unavailable. Every other link is fetched exactly as usual either way. Never turn the setting on in a deployment.

Two limits come with building feeds around "now". Between about 11 pm and 1 am, the "already ended" or "still to come" event may not fit inside today and is left out. And "today" is the server's date, which only matters if the server and browser are in different time zones.

**Real calendars instead.** To try a real feed, set `SEED_CALENDAR_MAYA`, `SEED_CALENDAR_SAM`, `SEED_CALENDAR_JORDAN` or `SEED_CALENDAR_ALEX` in `backend/.env` to a private iCal link and run `make demo` again. That member gets the real link instead of the fixture. The link goes only into `kinlight.db.demo`, and `make demo` never prints it.

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

## Kid chat

Kids get **Chat**: an "Ask Porchlight" card at the top of their page, and a Chat page with two kinds of conversation.
- **Homework help:** a tutor that guides rather than answers. For anything that looks like an assignment, it works up a hint ladder, one step at a time: what have you tried, a hint, a worked example of a similar problem, then checking their answer. It doesn't hand over the answer or write the essay, even if asked twice, told it isn't homework, or told to ignore its rules.
- **Explore:** a companion for questions, ideas, stories and projects. The homework rule still applies.

It talks at the kid's age: a younger voice up to 12, a teen one from 13. A parent sets the birth year on the kid's page; without one, the younger voice is used. Replies stream in as they're written. Every chat screen says **"Your parents can review these chats"**. Parents don't get a chat of their own yet.

**Tools.** The agent can look things up for the kid who's chatting, and only for them:
- **the weather** where they live;
- **their day:** their own calendar for today and tomorrow, as their Today card shows it;
- **their picks:** their interests, and the articles and local events already on their page;
- **a kid-safe web search**, with strict safe-search.

Search is offered **only in Explore**, never in Homework help, so it can't be used to look up answers. It allows at most 2 searches per message. The agent puts what it finds in its own words, quotes no more than a few words, and gets snippets only, never links. Whatever a tool returns is treated as information, never as instructions.

Each tool use shows as a small note above the reply ("Checked the weather", "Searched: volcano facts") for the kid and in the parents' transcript. The notes are kept with the conversation. A search costs Brave credit, like refreshing a card.

**The safety check.** Before every reply, a fast model reads the kid's message (with the last few for context). It flags anything a parent should see:
- self-harm;
- abuse;
- bullying;
- sexual content;
- violence;
- drugs;
- strangers or personal information;
- hate.

A flag never blocks the chat: the kid still gets an answer.
- **A possible crisis:** the reply is told to respond with care and to encourage talking to a trusted adult. The server then adds, whatever the model wrote: *call or text 988, or text HOME to 741741; in an emergency, call 911.*
- **The check fails:** the message is answered and flagged "couldn't check".
- **The check's provider refuses to read it** (some content filters do, for self-harm): it's handled as a possible crisis.
- **A crisis reply can't be written:** the kid gets fixed, caring words and the resources instead of an error.

**For parents.**
- **Reviews** in the family sidebar shows how many flags are waiting. The page lists them crisis first, each with the reason and the message, and links to the whole conversation. "Mark reviewed" records who reviewed it and when.
- Each kid's conversations can be read in full, and deleted.
- While a possible crisis is unreviewed, every parent page shows a red alert naming the kid. It checks every minute and when you come back to the tab.

Phone push comes later. Conversations are kept until a parent deletes them, and usage records keep only token counts.

**Turning it on: kids' conversations only go to providers that keep nothing.**
1. Create a separate OpenRouter key for the kids, with credits.
2. Set `OPENROUTER_KIDS_API_KEY` in `backend/.env`. Every kid call through it, the safety check and the replies, asks OpenRouter for zero-data-retention endpoints of providers that don't collect data.
3. Run `make ai-smoke` and `make ai-eval-kids`.
4. Restart.

Without the key, kid chat is off: kids see "Chat isn't set up yet. Ask a parent.", and nothing is sent anywhere.

The models are `KIDS_CHAT_MODEL` and `KIDS_SAFETY_MODEL`, both `anthropic/claude-haiku-5.5` by default. A model needs a zero-retention endpoint that doesn't filter out what a kid in trouble writes. The OpenAI models' zero-retention endpoints block self-harm messages, and Gemini has no zero-retention endpoint, so neither qualifies.

**Prototyping.** `KINLIGHT_KIDS_ALLOW_RETENTION=1` lets kid chat run on any provider, with the main key if there's no kids' key. It's for trying things with the demo family. The server warns at startup when it's on. Never set it once your kids use the app.

**Checking behavior.** `make ai-eval-kids` runs 17 scripted kid conversations through the real chat service and models:
- homework asked for straight, twice and in disguise, and an essay;
- ordinary questions;
- risky messages, and two crisis phrasings;
- a prompt injection;
- the tools: the weather, their day, search in Explore, no search in Homework help, and an instruction hidden in their picks.

The search cases use real kid-safe search. It prints pass or fail for each. It costs a few cents and never runs in CI.

## Assistant

Parents get an **Assistant**: a private chat in the family sidebar. Only the parent who had a conversation can see it, not even the other parent can, and a parent can delete their own. Kids don't have one; kid chat is theirs.

You can ask it things like:
- "what does everyone have tomorrow?";
- "find a good beginner robotics kit and where to buy it";
- "anything I should know about Lily's chats this week?".

It **only reads and answers**. It can't mark flags reviewed, delete chats or change settings; asked to, it tells you where in the app to do it.

Each conversation is one of two kinds, chosen when it starts:
- **Ask anything** runs on the main OpenRouter key. Its tools:
  - web research, with no kid filter, up to 4 searches per message, citing sources with clickable links;
  - the family's day: your calendar, or anyone's in your family, today and tomorrow;
  - the weather where you live;
  - your page's picks.

  It can't read the kids' chats.
- **About the kids** has all of the above, plus the kids' chats: the review queue, each kid's conversations and flags, and a conversation's transcript. Because it reads what the kids wrote, it runs entirely on the kids' key with the same zero-retention routing as kid chat, so it's only available when kid chat is set up.

The kind is how the zero-retention rule is kept. The kids'-chats tool is only built for an "About the kids" conversation, so nothing asked in "Ask anything" can reach the kids' chats.

Tool use shows as notes above each reply ("Searched: …", "Looked at the family's day", "Read the kids' chats"), and is kept with the conversation. The model is `PARENT_CHAT_MODEL`, `anthropic/claude-haiku-5.5` by default. Every call is recorded per parent under `parent_chat`, and searches use Brave credit.

`make ai-eval-parent` checks it on the real models:
- research cites links;
- the family's day is used;
- "Ask anything" can't read the kids' chats;
- a kid summary flags what matters, even with an instruction planted in the kid's chat;
- it stays read-only.

`make ai-smoke` also checks a parent tool call on each key.

## Tests

```sh
cd backend
make check   # go vet + go test + govulncheck + go build
```

`make check` also runs `govulncheck` (pinned in `go.mod` as a tool), which fails if Porchlight's code can reach a known vulnerability in Go or a dependency. It downloads the vulnerability list from vuln.go.dev, so it needs network; `make test` and `make vet` work offline. The race detector works too: `CGO_ENABLED=0 GOTOOLCHAIN=local go test -race ./...`.

The tests replace OpenRouter, Brave, Open-Meteo and calendar feeds with fakes (calendar tests read sample feeds in `backend/internal/calendar/testdata/`), so they need no keys, spend no credit and make no network calls.

GitHub Actions (`.github/workflows/ci.yml`) runs three checks on every pull request and every push to `main`:
- **Backend:** `make check`, on Go 1.27.3.
- **Backend race detector:** `go test -race ./...`. On Linux it needs cgo, so this job turns cgo on.
- **Frontend:** `npm run lint` and `npm run build`.

## AI usage

Every answer from the AI model records its token counts in the `llm_usage` table. Each record has:
- the feature it was for (describing a widget, refreshing one, Articles, Local Events or the morning briefing);
- the step within that feature;
- the model;
- the family member it was for;
- who caused it. A parent opening a kid's page is charged to the kid, with the parent as the viewer.

No prompts, replies, interests or calendar data are stored, only counts and labels. Recording never slows a request: entries go on a queue that one background goroutine writes (`internal/usage`). There's no screen for usage yet. `store.UsageSummary` totals a family's usage per member and feature for when there is.

## How the backend is laid out

`routes.go` wires the HTTP API from one `deps` struct. The widget work lives in services in `internal/widgets` that take a context and a family member and return results, with no HTTP involved:
- `WidgetBuilder`: describe and refresh widgets.
- `Curation`: Articles and Local Events.
- `Agenda`: the Today card.
- `Briefings`: the morning briefing.

Kid chat lives in `internal/chat`, a service in the same style. Its agent, the prompts and the safety check, is `aikit.KidAgent`.

The HTTP handlers only decode the request, call a service, and turn its result or error into a response. The family agent will call the same services.

AI calls go through one of two packages:
- `internal/llm`, the original OpenRouter client, for describing widgets, Articles and Local Events.
- `internal/aikit`, which builds Genkit for Go and is the only package that imports Genkit's model plugins. It writes the morning briefing, and the family agent will be built on it (see below).

## Genkit

[Genkit for Go](https://genkit.dev/docs/go/) (v1.14.0) is the framework the family agent will be built on. For now it writes the morning briefing, with the same prompts, length rules, retries and error messages as the original client (a contract test checks that the two agree).

**Models.** Both clients use `OPENROUTER_MODEL` (default `openai/gpt-4.1-mini`) through your OpenRouter account. In Genkit that model is called `openrouter/openai/gpt-4.1-mini`. To give Genkit a different one, set `GENKIT_MODEL` in `backend/.env` to `openrouter/` plus any OpenRouter model id. Every Genkit call's tokens go to `llm_usage` like everything else's.

**Switching back.** `KINLIGHT_BRIEFER=legacy` in `backend/.env` makes the original client write the briefing again, after a restart. The default is `genkit`, and any other value stops the server at startup. The switch goes away once the agent has run on Genkit for a while.

**MiniMax.** With `MINIMAX_API_KEY` set (and optionally `MINIMAX_BASE_URL` and `MINIMAX_MODEL`), MiniMax's Anthropic-compatible API is available to Genkit as `anthropic/<MINIMAX_MODEL>`. Nothing uses it yet. It's for the owner's own experiments and never for kids' conversations: those go only to providers that don't keep or train on what they're sent, and a subscription plan doesn't promise that. Without the key, MiniMax isn't registered at all.

**Checking the real providers.** `make ai-smoke` (in `backend/`) calls the real models: structured output, one tool call, a required tool call, and usage reporting, for OpenRouter and, with a key, MiniMax. It costs a fraction of a cent, so it only runs when you ask; CI never does. The tests otherwise use `internal/aikit/aitest`, a real Genkit instance whose one model answers from a script, so they need no keys or network.

**Watching AI calls.** `make dev-ui` runs the API under Genkit's Developer UI at <http://localhost:4000>. It shows each model call's prompt, reply, tokens and timing. Settings pass through, e.g. `DB_PATH=kinlight.db.demo KINLIGHT_DEV_FIXTURES=1 make dev-ui` for the demo family. The CLI is pinned (`genkit-cli@1.44.1`) and fetched by `npx`. On first run it asks you to acknowledge Google's analytics notice; `npx genkit-cli@1.44.1 config set analyticsOptOut true` opts out instead. Traces are kept in `backend/.genkit/`. They hold full prompts and replies, so the folder is gitignored; delete it to clear them.

The Dev UI talks to Genkit's reflection server on 127.0.0.1:3100, which **runs any AI flow without a login**. It starts only when `GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED` is set, and the backend logs a warning when it does. Use it on your own machine only, and never set either variable on the homelab.

## How it was planned

The proof of concept was planned and built with the [Devpost Learn skill pack](https://github.com/challengepost/learn-ai-basics). Its planning documents are in `devpost/`: `scope.md`, `prd.md`, `spec.md` and the build `checklist.md`. The git tag `poc-complete` marks the finished proof of concept.

Features added after that are planned with [OpenSpec](https://github.com/Fission-AI/OpenSpec), in `openspec/`, so the original planning documents stay as they were. Each feature starts as a proposal in `openspec/changes/` and, once built, is archived into `openspec/specs/`. In Claude Code, `/opsx:propose` starts one, `/opsx:apply` builds it and `/opsx:archive` finishes it.
