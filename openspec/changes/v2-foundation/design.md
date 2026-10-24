## Context

See proposal.md for why. Current state on `aw-/v2-go-upgrade` (Go 1.27):

- **`internal/widgets` holds most of the domain logic inside handlers.** Each handler reads the subject, plans, searches, shapes, remembers and logs, and writes HTTP errors along the way through `writeLLMError`, `writeSearchError` and `writeIfCancelled`:
  - `Handler.FromPrompt` and `Handler.RefreshCustom` (`handler.go`, 616 lines)
  - `CuratedHandler.Articles` and `CuratedHandler.Events` (`curated.go`, 614 lines)
  - `TodayHandler.Today` (`today.go`)
  - `BriefingHandler.Briefing` with `gatherFacts` (`briefing.go`, 573 lines)
- **Consumer-side interfaces already exist:** `Planner`, `Curator`, `Briefer`, `SourceFinder`, `WidgetStore`, `ProfileStore`, `ResultStore`, `CalendarStore`, `CalendarFetcher` and `Layouts`. Tests swap in fakes through the handler constructors (`NewHandler`, `NewCuratedHandler`, ...).
- **`routes(logger, st, sessions, planner, curator, briefer, srcs, calendars)`** is called from `main.go` and from one test helper in `routes_test.go:153`.
- **`middleware.go`:** `statusRecorder` embeds `http.ResponseWriter`, which hides any `Flush` and `SetWriteDeadline` methods of the real writer. `rejectCrossSite` hand-rolls the `Sec-Fetch-Site`/`Origin` checks.
- **`internal/llm`:** every model call goes through `completeOnce`, which ignores `resp.Usage`. The `*llm.Client` methods (`PickWidgetSpec`, `ShapeItems`, `CurateArticles`, `CurateEvents`, `ExtractEvents`, `WriteBriefing`) don't know which member a call is for.
- **No CI.** `make check` is run by hand.

## Goals / Non-Goals

**Goals:**
- An agent tool can call "build a widget", "refresh a widget", "articles", "events", "today" and "briefing" for a member, with no `http.Request` involved, and get typed results and errors.
- Zero visible behavior change, proven by the existing route and handler tests passing **without editing any test case or assertion**. Only the one helper that builds the server changes.
- Usage recording that works for today's go-openai client and for the Genkit integration that comes next.
- A red CI build on a broken push.

**Non-Goals:**
- Moving packages, or renaming `internal/widgets` into a domain/service layer hierarchy. That would be a big churn for no gain yet.
- Any agent code, streaming endpoint or Genkit.
- A usage screen, spending limits or rate limiting.
- Frontend tests. None exist yet, and CI only lints and builds the frontend.

## Decisions

### 1. Services live beside the handlers, and handlers build them
For each pipeline, a service type goes into `internal/widgets`, in its own file:
- `WidgetBuilder` (`Build`, `Refresh`)
- `Curation` (`Articles`, `Events`)
- `Agenda` (`Day`)
- `Briefings` (`Today`)

Each service method takes `ctx`, the member (and the viewer, where logs and usage need it) and plain inputs such as the prompt, the time zone and whether to refresh. Each returns the same widget structs the handlers return today, plus an `error`. The existing constructors keep their signatures and build their service from the dependencies they already receive, for example `NewCuratedHandler(...)` builds `NewCuration(...)`. Every existing handler test therefore compiles and passes unchanged, and later the agent package calls `NewCuration(...)` directly with the same interfaces.
- *Alternative:* a new `internal/service` package. It would force moving the shared types (`Widget`, `CuratedWidget`, freshness, kinds) and every test fake. That's churn without a benefit until a second consumer exists.

### 2. Typed service errors, mapped to HTTP in one place
Services return errors that carry what the handler needs:
- `ErrNoSuchWidget` for "not found";
- `*llm.UpstreamError`, which is passed through;
- search errors wrapped with a step;
- `context.Canceled` and `context.DeadlineExceeded`;
- a small `*widgets.StepError{Step: "event extraction", Err: ...}` where today's messages name the step.

One `writeError(w, r, err)` per handler type does the status and message mapping that `writeLLMError`, `writeSearchError` and `writeIfCancelled` do today, so the error bodies stay byte-for-byte the same. Successful empty states (no interests, no location, no calendar, calendar gone) are *results*, not errors, exactly as today.
- *Alternative:* return HTTP statuses from services. That would leak HTTP into what the agent calls.

### 3. "Remembered" results and refresh semantics move into the services
The freshness rules (`sendRemembered`/`remember`, the `?refresh=1` 1-minute guard, and stale marking) are behavior, so they move into the services. The handler only translates `?refresh=1` into a `Refresh bool` option. The agent then gets the same caching, which protects search credit.

### 4. `routes(d deps)` with a plain struct
```go
type deps struct {
    Logger    *slog.Logger
    Store     *store.Store
    Sessions  *scs.SessionManager
    Planner   widgets.Planner
    Curator   widgets.Curator
    Briefer   widgets.Briefer
    Sources   widgets.SourceFinder
    Calendars widgets.CalendarFetcher
}
```
Named fields make call sites readable, and zero values are obvious in tests. `main.go` fills it once. `routes_test.go`'s server helper is the only test code that changes.
- *Alternatives:* functional options, which are heavier than needed, or a global app container, which hides dependencies.

### 5. Streaming-ready middleware via `Unwrap`
`func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }` is the documented hook (Go 1.20+) that `http.ResponseController` uses to reach `Flush`, `SetWriteDeadline` and `Hijack` through wrappers. The scs `LoadAndSave` writer already supports `Unwrap`. A test sends a request through the full middleware stack (`logRequests`, `recoverPanics`, `rejectCrossSite`, sessions) to a handler that writes, flushes and then blocks, and checks that the client reads the first chunk before the handler finishes.

### 6. `http.CrossOriginProtection` for writes; Porchlight's stricter rules stay
`rejectCrossSite` keeps its three behaviors:
- refuse cross-site and same-site `Sec-Fetch-Site` on every method (reads spend credit);
- an origin check on writes;
- JSON-only bodies.

The origin check on writes is delegated to a `http.CrossOriginProtection` value, which implements the same `Sec-Fetch-Site`/`Origin`-vs-`Host` logic in the standard library. That removes `sameHost`. Status and message stay the same: its deny handler is set to our `respond.Error(403, "That request came from another site.")`. The existing `TestCrossSiteWritesRejected` and `TestCrossSiteReadsRejected` guard this.

### 7. Usage: a recorder interface on `llm.Client`, attribution through the context
- `llm.Usage{Model, Step, PromptTokens, CompletionTokens, TotalTokens}` is filled from `resp.Usage` in `completeOnce`, on every provider answer, including ones then rejected as invalid JSON.
- The client calls `UsageRecorder.RecordUsage(ctx, Usage)` (an interface; nil means "don't record").
- `llm.WithAttribution(ctx, llm.Attribution{MemberID, ViewerID, Feature})` is set by each service before it calls the planner, curator or briefer. The recorder reads it with `llm.AttributionFrom(ctx)`.
- A missing attribution records feature `unattributed` with a null member, and logs a warning.
- **Writes go through a background writer so they never block a request.**
  - `usage.Recorder` (a small `internal/usage` package) implements `RecordUsage` by putting the entry on a buffered channel (capacity 256) and returning at once.
  - One goroutine, started in `run(ctx)`, drains the channel into `store.InsertLLMUsage`.
  - If the channel is full, the entry is dropped and counted, and a warning is logged once per minute.
  - On shutdown, `Close()` stops accepting entries, drains the queue with a 5-second limit, and returns.
  - It's a producer/consumer pattern using a channel and one goroutine, a core Go idiom. Tests read entries back with `Flush()`.
- Genkit's `resp.Usage` will feed the same interface later.
- Migration `005_llm_usage.sql`:
  ```sql
  CREATE TABLE llm_usage (
    id                INTEGER PRIMARY KEY,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    family_id         INTEGER REFERENCES families(id),
    member_id         INTEGER REFERENCES users(id),
    viewer_id         INTEGER REFERENCES users(id),
    feature           TEXT NOT NULL,
    step              TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens      INTEGER NOT NULL
  );
  CREATE INDEX llm_usage_family_time ON llm_usage(family_id, created_at);
  ```
  `family_id` is stored so the summary needs no join and can't leak another family's rows.
- `store.UsageSummary(ctx, familyID, since)` returns per-member, per-feature totals.
- *Alternative:* return usage from every `llm.Client` method. That changes six signatures and every fake, for data that's only ever logged.

### 8. CI: one workflow, three jobs
`.github/workflows/ci.yml` runs on push and pull_request:
- **backend:** `actions/setup-go` with `go-version: 1.27.3` and module cache on `backend/go.sum`, then `make check` (vet, test, govulncheck, build). `GOTOOLCHAIN=local` is already in the Makefile.
- **backend-race:** `CGO_ENABLED=1 go test -race ./...`. Linux's race detector needs cgo, unlike macOS. This is the only place cgo is turned on.
- **frontend:** `actions/setup-node` with Node 22 and an npm cache, then `npm ci`, `npm run lint` and `npm run build`.

Action versions are pinned to current major tags at implementation time. The jobs need no secrets, because the tests use fakes.

### 9. `testing/synctest` deferred
`synctest.Test` runs a test in a "bubble" with a fake clock, so timers fire as soon as every goroutine is blocked. Timings measured with `go test -json` show no test here that waits on a real timer:
- the slowest is 0.33s (bcrypt);
- `TestPlanningHasItsOwnDeadline` only inspects the deadline;
- the LLM retry test shortens its delay with `SetRetryDelay` and talks to an `httptest` server, which is real network I/O, so synctest can't treat it as "durably blocked".

Adopting synctest now would add a new idea without making anything faster. It comes in with the agent's turn and stream timeouts, where it pays off.

## Risks / Trade-offs

- [Moving a pipeline changes a subtle behavior, such as error text, logging fields or cache timing] → existing tests cover status and body. Moving one pipeline per commit with the full suite green after each keeps any regression local. The route tests' error-message assertions are the guard.
- [The services still take store interfaces that mention `store.*` types] → acceptable. The agent lives in the same module, and the types are plain data.
- [Usage recording adds a write to every LLM call] → the request only does a non-blocking channel send. One background goroutine does the insert, so even a 5-second SQLite busy wait never reaches a member.
- [Entries are lost if the process crashes, or the queue overflows during a burst] → acceptable for usage statistics with no billing attached. Drops are counted and logged.
- [CI's race job is slow (about 50s locally)] → it runs in parallel with the other jobs and doesn't block local `make check`.
- [`http.CrossOriginProtection` treats a missing `Sec-Fetch-Site` plus a missing `Origin` as allowed (non-browser clients), as our code does today] → same behavior. The JSON-body rule still blocks HTML form posts.

## Migration Plan

1. Land the change on `aw-/v2-foundation`, stacked on `aw-/v2-go-upgrade`. Merge PR #10 first, then rebase or retarget this branch to `main`.
2. Migration `005` runs automatically at startup and only adds a table. Rolling back the code leaves an unused table, which is harmless.
3. Once the workflow is on `main`, CI runs on pull requests. The first PR shows it working.
