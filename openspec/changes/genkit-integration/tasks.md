## 1. Shared briefing rules (no behavior change)

- [x] 1.1 In `internal/llm`, export `BriefingPrompt(facts, aud) (system, user string)`, `CleanBriefing(text string) (string, error)` and `UpstreamFor(status int, err error) *UpstreamError`, carved out of `briefing.go` and `client.go`, and make `Client.WriteBriefing` and `upstreamError` use them. Add table tests for `CleanBriefing` (empty, whitespace-only, normal, over 320 cut at a sentence end, over 400 rejected) and `UpstreamFor` (401, 402, 429, 500, an unknown 4xx, deadline, cancelled). Verify with `go test ./internal/llm/` and `make test`, with all existing llm and briefing tests unchanged and passing. Commit.

## 2. Genkit in the module

- [x] 2.1 Run `go get github.com/firebase/genkit/go@v1.14.0` (exact version; `GOTOOLCHAIN=local`) and `go mod tidy`. Verify that `go.mod` keeps `go 1.27.0` with no toolchain line, and that `make check` (including govulncheck) passes. Record any reachable vulnerability here, and fix it before continuing. Commit together with 3.1, since an unused dependency would be tidied away.
  - Result (2026-10-24): `go.mod` still says `go 1.27.0` with no toolchain line. Genkit brought in `openai-go` v1.8.2 and `anthropic-sdk-go` v1.23.0. govulncheck: 0 reachable vulnerabilities; the one module-level finding is the existing GO-2026-5932 (`x/crypto/openpgp`, never called).

## 3. `internal/aikit`

- [x] 3.1 Add `aikit.Config` (OpenRouter key and base URL, default model, MiniMax key, base URL and model) and `aikit.New(ctx, cfg) (*Kit, error)`:
  - OpenRouter is registered with `option.WithBaseURL` and `option.WithMaxRetries(0)`.
  - The anthropic plugin is registered for MiniMax only when its key is set.
  - Missing required config returns an error, never a panic.

  Add `Kit.Genkit()` for later changes. Tests:
  - `New` without a MiniMax key registers no anthropic model, and doesn't panic.
  - `New` with a key resolves `anthropic/<MINIMAX_MODEL>` (`genkit.LookupModel` is non-nil).
  - The default model name is `openrouter/` + `OPENROUTER_MODEL`.

  Verify with `go test ./internal/aikit/`.
- [x] 3.2 Add the usage middleware: an `ai.MiddlewareFunc` whose `WrapModel` reports each response's `Usage` to an `llm.UsageRecorder`, with the call's context and step name. Add `Kit.SetUsageRecorder`. Add `aitest` (3.3) first if that's easier. Test with the scripted model:
  - one call with usage produces one `llm.Usage`, with the attribution taken from the context;
  - a call that errors records nothing;
  - a nil recorder is fine.

  Verify with `go test ./internal/aikit/...`.
  - As built, the middleware is `Kit.TrackUsage(step) ai.GenerateOption`, an `ai.WithUse(ai.MiddlewareFunc(...))`. It takes the model name from `WrapGenerate` and records it as the legacy client does: `openrouter/` is dropped, and `anthropic/` becomes `minimax/`.
- [x] 3.3 Add `internal/aikit/aitest` with `aitest.New(t, replies...) *aikit.Kit`. It builds a real `genkit.Init` with one `DefineModelAction` model, `test/scripted`, which:
  - answers from a queue of `aitest.Reply` values (text, JSON, a tool request, an error with an HTTP status);
  - reports the configured usage;
  - records requests (`Requests()`).

  The default model is the scripted one. (As built, `New` returns `(*aikit.Kit, *aitest.Model)`: `Requests()` lives on the Model, which the Kit can't reach.) Add a `TestScriptedModel` covering each reply kind and request recording. Verify with `go test ./internal/aikit/aitest/`.
- [x] 3.4 Add the error translation `aikit.upstream(err) error`. An `*openai.Error` (found with `errors.As`) becomes `llm.UpstreamFor(StatusCode, err)`. Deadline and cancellation map as today. A Genkit `status.Error` with no SDK error maps by status. Anything else becomes "couldn't reach OpenRouter". Add a table test, including an error from the real compat_oai plugin against an `httptest` server returning 401, 402, 429 and 500 through the base URL. Verify with `go test ./internal/aikit/`.

## 4. The briefing on Genkit

- [ ] 4.1 Add `aikit.Briefer` implementing `widgets.Briefer`:
  - `genkit.GenerateData[briefingReply]`, using `llm.BriefingPrompt` and `llm.CleanBriefing`, step `morning_briefing`;
  - one retry with a correction when the reply can't be used, and one 1-second retry on a 429;
  - errors through `aikit.upstream`.

  Tests with `aitest`:
  - a clean reply;
  - an unusable reply, then a good one: 2 model calls;
  - unusable twice: an error wrapping `llm.ErrNoBriefing`;
  - a 429 then success;
  - 429 twice: an `UpstreamError` with "rate limiting";
  - kid guidance present in the system prompt for `sources.Kids` and absent for `Everyone`;
  - usage recorded once per model call.

  Verify with `go test ./internal/aikit/`.
- [ ] 4.2 Add a contract test that runs the same scripted replies through both briefers (`llm.Client` against the fake OpenRouter, and `aikit.Briefer` against `aitest`) and checks:
  - the same output text;
  - the same error kinds (`ErrNoBriefing`, the `UpstreamError` status and reason);
  - the same system and user messages sent.

  Verify with `go test ./internal/aikit/`.
- [ ] 4.3 Wire it in `main.go`:
  - build the Kit after the preflight checks;
  - read `PORCHLIGHT_BRIEFER` (`genkit` by default, or `legacy`; anything else exits with a clear message) and pass the chosen briefer in `deps.Briefer`;
  - give the Kit the same usage recorder;
  - log a warning when `GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED` is set.

  Add the new variables, commented, to `backend/.env.example`. Verify with `make check`, and by starting the server with each `PORCHLIGHT_BRIEFER` value and with an invalid one.

## 5. Tooling and docs

- [ ] 5.1 Add `make ai-smoke`, which runs `PORCHLIGHT_LIVE_AI=1 go test -run Live ./internal/aikit/`. Add `live_test.go`, skipped unless that variable is set, with three checks for OpenRouter and, if `MINIMAX_API_KEY` is set, for MiniMax:
  - `GenerateData` into a struct;
  - one tool call (`add`);
  - usage reported.

  Run it and record the results here, especially MiniMax's tool and structured-output behavior. Verify that `make ai-smoke` passes for OpenRouter, and record MiniMax as passed, failed (with the error) or skipped (no key).
- [ ] 5.2 Add `make dev-ui` (`GENKIT_ENV=dev npx --yes genkit-cli@1.44.1 start -- go run .`, with the Makefile's Go settings). Verify by starting it, opening the Dev UI, and seeing a briefing's model call traced after loading a page. Stop it afterwards.
- [ ] 5.3 Add a "Genkit" section to `README.md`:
  - which models are used and how to choose them;
  - `PORCHLIGHT_BRIEFER`;
  - MiniMax is for the owner's own use and never kid traffic, and why;
  - `make ai-smoke`;
  - `make dev-ui`, and why it must never run on the homelab.

  Verify by reading it.

## 6. End to end

- [ ] 6.1 With real keys, run `make demo` and then `make demo-serve` on a side port with the frontend:
  - with the default Genkit briefer, Maya's and Jordan's briefings are written and read naturally, and Jordan's follows kid guidance;
  - `llm_usage` has `morning_briefing` rows from the Genkit path, with token counts;
  - the same check with `PORCHLIGHT_BRIEFER=legacy`;
  - the API log has no warnings beyond the expected fixture notice.

  Record the results here.
- [ ] 6.2 Run `openspec validate genkit-integration`, and confirm CI passes on the PR. Add the "Genkit integration" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
