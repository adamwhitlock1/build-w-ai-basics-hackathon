## Why

The family agent (`kid-tutor-agent`, `agent-tools`, `parent-agent`) needs to call what Porchlight already does: build a widget from a sentence, list a member's day, fetch their articles or nearby events, and write their briefing. Today that logic lives inside HTTP handler methods that write responses as they go (`Handler.FromPrompt`, `CuratedHandler.Articles`/`Events`, `TodayHandler.Today`, `BriefingHandler.Briefing`), so nothing but an HTTP request can use it. Four other gaps would also hurt the agent work:
- every LLM call's token usage is thrown away;
- the logging middleware hides `Flush`, which blocks streamed replies;
- `routes()` already takes eight parameters;
- nothing runs the tests on each push.

Fixing all of this before any agent code keeps the agent changes about the agent.

## What Changes

- **Services pulled out of handlers.** The widget package gets plain Go services, each with a context and a member in and a result or typed error out:
  - building a described widget;
  - refreshing one;
  - Articles;
  - Local Events;
  - a member's Today;
  - the morning briefing.

  Handlers shrink to decoding the request, calling a service, and turning its error into the same status and message as today. Existing route tests pass unchanged, and that is the proof that nothing visible moved.
- **`routes()` takes one dependencies struct** instead of eight parameters, and `main.go` builds it once. Tests build it with fakes, as they do now.
- **Streaming-ready middleware.** `statusRecorder` gains `Unwrap()`, so `http.ResponseController` can reach `Flush` and `SetWriteDeadline` through the logging middleware. A test proves a flushed write reaches the client before the handler returns.
- **Cross-origin checks use the standard library.** For writes, the Origin and `Sec-Fetch-Site` check delegates to `http.CrossOriginProtection` (Go 1.25). The stricter Porchlight rules stay: cross-site *reads* are refused (they spend credit), and request bodies must be JSON.
- **LLM usage is tracked per member.** Every completion's token counts (`resp.Usage`) are recorded with:
  - the member it was for and the viewer who asked;
  - the feature;
  - the call's name;
  - the model.

  The record never includes prompt or reply text. A failed record is logged and never fails the request. The recorder sits behind a small interface, so the coming Genkit integration can feed the same table. There's no screen for it yet; a store query summarizes usage by member for the parent view that comes later.
- **CI on GitHub Actions** for every push and pull request:
  - **Backend:** vet, tests, govulncheck and build on go1.27.3, plus a separate race-detector job.
  - **Frontend:** lint and build on Node 22.
- **`testing/synctest` is not adopted yet.** I measured the suite: no current test waits on a real timer (the slowest, at 0.33s, is bcrypt), so there's nothing for it to speed up. It comes in with the agent's turn timeouts.

## Capabilities

### New Capabilities
- `llm-usage`: recording token usage for every LLM call, attributed to a family member and a feature, without storing any message content, plus a per-member summary.

### Modified Capabilities

None. The widget, Today, briefing, kid-safety and freshness specs keep every requirement. The services must reproduce their behavior exactly.

## Impact

- **Backend:**
  - `internal/widgets` is split into service and handler files: `routes.go`, `main.go`, `middleware.go`.
  - `internal/llm` reads usage, and its `Client` takes a usage recorder.
  - `internal/store` gets usage methods, and a new migration `005_llm_usage.sql` adds one table.
  - A new small `internal/usage` package holds the background writer.
- **Tests:** new service-level tests (no HTTP), a middleware flush test, and usage-recording tests. Existing route and handler tests stay unchanged as the no-behavior-change guard.
- **New files:** `.github/workflows/ci.yml`.
- **Docs:** the README gets CI and usage notes. On archive, one line goes in "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
- **No API, frontend or dependency changes.**
- **Depends on** `go-upgrade` (Go 1.27). This branch is stacked on it, so PR #10 should merge first.
