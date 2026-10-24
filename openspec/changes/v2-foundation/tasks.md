## 1. Dependencies struct

- [x] 1.1 Add `type deps struct { Logger, Store, Sessions, Planner, Curator, Briefer, Sources, Calendars }` in `backend/routes.go` and change `routes(...)` to `routes(d deps)`. Fill it once in `main.go`, and update only the server-building helper at `routes_test.go:153`. Verify with `make test`. `git diff -- '*_test.go'` must show only that helper changed. Commit.

## 2. Middleware

- [x] 2.1 Add `func (r *statusRecorder) Unwrap() http.ResponseWriter` in `backend/middleware.go`, with a comment on why `http.ResponseController` needs it. Add `TestFlushReachesClientThroughMiddleware` in `middleware_test.go`:
  - The handler runs behind the full stack (`logRequests`, `recoverPanics`, `rejectCrossSite`, sessions).
  - It writes `"first"`, flushes with `http.NewResponseController(w).Flush()`, then waits on a channel.
  - The client must read `"first"` before the test releases the handler.

  Also assert that `SetWriteDeadline` returns no error. Verify the test fails without `Unwrap` (temporarily remove it) and passes with it. Commit.
- [x] 2.2 Delegate the origin check for writes in `rejectCrossSite` to a package-level `http.CrossOriginProtection`, whose deny handler writes the same `respond.Error(403, "That request came from another site.", "")`. Remove `sameHost`. Keep the cross-site-read and JSON-body rules. Verify with `TestCrossSiteWritesRejected`, `TestCrossSiteReadsRejected` and `make test`, all unchanged and passing. Commit.

## 3. Services out of handlers

- [x] 3.1 Add the error plumbing in `internal/widgets/errors.go`:
  - `ErrNoSuchWidget`;
  - `StepError{Step string; Err error}` (with `Unwrap`);
  - one `writeServiceError(w, r, logger, err)` that reproduces today's `writeLLMError`, `writeSearchError` and `writeIfCancelled` statuses, messages and hints.

  Add a table test that each error kind maps to the same status and body the current handlers produce, using the existing messages as the expected values. Verify with `go test ./internal/widgets/`. Commit.
- [x] 3.2 Add `Agenda` (`agenda.go`) with `NewAgenda(fetcher, store, logger)` and `Day(ctx, member, loc, opts) (TodayWidget, error)`. It holds the link lookup, the remembered day, `ErrLinkGone`, the fetch, `Between`, and remembering. `TodayHandler.Today` becomes: subject, then `locationFrom`, then `Day`, then respond or `writeServiceError`. `NewTodayHandler` keeps its signature and builds the Agenda. Add `agenda_test.go` service tests (no HTTP) for no calendar, remembered day, link gone, and a fixture day. Verify that `go test ./internal/widgets/ -run 'Today|Agenda'` and `make test` pass with the existing Today tests unchanged. Commit.
- [x] 3.3 Add `Curation` (`curation.go`) with `Articles(ctx, viewer, member, opts)` and `Events(ctx, viewer, member, opts)`. These hold the empty states, remembered results and freshness, `Refresh`, planning with its own deadline, the fallbacks, `openRouterRefused`, search, pick or extract, remember, and the "curated" log line. The `CuratedHandler` methods become thin. Add service tests for each empty state, a remembered hit, the planning fallback, and an extraction failure surfacing as a `StepError`. Verify with `make test`; `curated_test.go`, `refresh_test.go` and `freshness_test.go` stay unchanged. Commit.
- [x] 3.4 Add `WidgetBuilder` (`builder.go`) with `Build(ctx, member, prompt) (SavedWidget, error)` and `Refresh(ctx, member, widgetID, opts) (SavedWidget, error)`. These hold the planning, the unknown source (422 kept through a `StepError`), search, shaping with the raw-results fallback, the save with `context.WithoutCancel`, the layout append, and the refresh guard. `Handler.FromPrompt` and `RefreshCustom` become thin. Add service tests: a successful build saves and appends, an unknown source, shaping falls back, and a refresh within the guard returns the remembered widget. Verify that `make test` passes with `handler_test.go` and `refresh_test.go` unchanged. Commit.
- [x] 3.5 Add `Briefings` (`briefings.go`) with `Today(ctx, viewer, member, loc) (BriefingWidget, error)`. It holds the per-member lock, the remembered briefing, the fingerprint and `shouldWrite`, `gatherFacts`, the write, and remembering. `BriefingHandler.Briefing` becomes thin. Add service tests for a fresh write, a remembered hit with an unchanged fingerprint, and a rewrite cap. Verify that `make test` passes with `briefing_test.go` unchanged, including `TestBriefingConcurrentRequestsWriteOnce`. Commit.
- [ ] 3.6 Confirm there were no behavior changes:
  - `git diff aw-/v2-go-upgrade -- 'backend/**/*_test.go'` shows only new test files, new test functions, and the routes helper from 1.1 (no edited assertions).
  - `go test -race ./...` passes.
  - `go vet` is clean.

  Record the diff summary in this task.

## 4. LLM usage tracking

- [ ] 4.1 Add migration `backend/migrations/005_llm_usage.sql` (table and index as in design.md decision 7). Add `store.InsertLLMUsage(ctx, LLMUsage)` and `store.UsageSummary(ctx, familyID, since) ([]MemberUsage, error)`, grouped by member and feature. Add store tests:
  - an insert round-trip;
  - the summary totals per member and feature;
  - rows from another family excluded;
  - rows before `since` excluded;
  - a row with a null member (unattributed) kept but not attributed to anyone.

  Verify with `go test ./internal/store/`. Commit.
- [ ] 4.2 In `internal/llm`, add:
  - `Usage{Model, Step, PromptTokens, CompletionTokens, TotalTokens}`;
  - `Attribution{MemberID, ViewerID, FamilyID, Feature}` with `WithAttribution` and `AttributionFrom`;
  - the `UsageRecorder` interface;
  - `Client.SetUsageRecorder`.

  `completeOnce` records `resp.Usage` after every provider answer (including answers later rejected as invalid), using `call.name` as the step. It records nothing when the request errored before an answer. Add tests against the fake OpenRouter server returning `usage`:
  - one entry per answered attempt (2 for an invalid-then-valid reply);
  - none for a transport error;
  - the attribution passes through the context;
  - a nil recorder is fine.

  Verify with `go test ./internal/llm/`. Commit.
- [ ] 4.3 Add `internal/usage` with `Recorder`:
  - `New(store, logger, capacity)`;
  - `Run(ctx)`, the single writer goroutine;
  - `RecordUsage`, a non-blocking send that drops and counts when the queue is full;
  - `Close(timeout)`, which drains;
  - `Flush()` for tests.

  It maps `Attribution` and `Usage` to `store.LLMUsage`, and uses feature `unattributed` with a warning when the attribution is missing. Tests:
  - entries reach a fake store;
  - a full queue drops without blocking (`RecordUsage` returns quickly while the fake store is blocked);
  - `Close` drains pending entries;
  - an insert error is logged and the writer keeps going.

  Verify with `go test -race ./internal/usage/`. Commit.
- [ ] 4.4 Wire it up:
  - `main.go` creates the Recorder, starts `Run` on the server's context, closes it after `srv.Shutdown`, and calls `llmClient.SetUsageRecorder`.
  - Each service sets `llm.WithAttribution` before calling the planner, curator or briefer, with features `describe_widget`, `refresh_widget`, `articles`, `events` and `briefing`, the member, the viewer and the family.

  Add route-level tests in `routes_test.go` (new test functions only) with a fake LLM that reports usage through the same recorder:
  - describing a widget records 2 entries for the member;
  - a parent opening a kid's Articles records member = kid and viewer = parent;
  - no column of any `llm_usage` row contains the prompt sentence;
  - a failing usage store still returns 201 for describe-a-widget.

  Verify with `make test`. Commit.

## 5. CI

- [ ] 5.1 Add `.github/workflows/ci.yml` triggered on `push` and `pull_request`, with three jobs:
  - **backend:** `actions/setup-go` with `go-version: 1.27.3` and cache on `backend/go.sum`, then `make check` in `backend/`;
  - **backend-race:** `CGO_ENABLED=1 go test -race ./...` in `backend/`;
  - **frontend:** `actions/setup-node` with Node 22 and an npm cache, then `npm ci`, `npm run lint` and `npm run build` in `frontend/`.

  Pin each action to its current major version. Verify with `actionlint` if available, or by reading the YAML; the real check is 5.2.
- [ ] 5.2 Push the branch and confirm all three jobs pass on GitHub (the PR's checks or the Actions run). If a job fails, fix it and record the cause here.

## 6. Docs and end-to-end

- [ ] 6.1 Update `README.md`:
  - "Tests" mentions CI and its three jobs.
  - A short "AI usage" note says every AI call records token counts per member and feature in `llm_usage`, with no message content, and that there's no screen for it yet.
  - The comments on `routes()` and the services describe the new structure.

  Verify by reading the README.
- [ ] 6.2 Run the demo end to end: `make demo`, then `make demo-serve` on a side port with the frontend. Log in as Maya and as Jordan, and confirm:
  - The cards load as before.
  - Describing a widget as Jordan works.
  - The `llm_usage` table in `backend/porchlight.db.demo` has rows for Jordan's describe-a-widget (2 steps) and Maya's briefing, with token counts and no text columns holding the prompt.
  - `UsageSummary` returns the per-member totals. Check it through a tiny test or by reading the table with the Go store in a scratch program.

  Record the numbers here.
- [ ] 6.3 Run `openspec validate v2-foundation` and confirm it passes. Add the "v2 foundation" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`. Verify that both files gain exactly one line.
