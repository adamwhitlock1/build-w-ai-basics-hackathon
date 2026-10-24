## Context

See proposal.md for why.

**Today, on `aw-/v2-foundation`:**
- `widgets.Briefings` calls a `widgets.Briefer` (`WriteBriefing(ctx, llm.BriefingFacts, sources.Audience) (string, error)`), implemented by `*llm.Client`.
- `llm.Client.WriteBriefing` builds a system prompt with `briefingSystemPrompt` and `forAudience`, which adds kid guidance, and a user message with `briefingUserMessage`.
- It asks for a strict JSON-schema reply through go-openai and cleans it: whitespace is normalized, a reply over 400 characters is retried, and a reply over 320 is cut at a sentence end (`trimBriefing`). An unusable reply gets one retry with a correction (`callJSON`), and a 429 gets one retry (`complete`). Errors become `*llm.UpstreamError`, which `widgets.writeServiceError` turns into the messages people see.
- Token usage goes to an `llm.UsageRecorder`, attributed through the context (`llm.WithAttribution`).

**What Genkit Go v1.14.0 provides** (checked in its source, module cache `github.com/firebase/genkit/go@v1.14.0`; it needs Go 1.25):
- **Setup and calls:** `genkit.Init(ctx, genkit.WithPlugins(...))`, `genkit.GenerateData[Out](ctx, g, opts...)` returning `(*Out, *ai.ModelResponse, error)`, `ai.WithModelName`, `ai.WithSystem`, `ai.WithPrompt`, `ai.WithConfig`, `ai.WithUse(ai.MiddlewareFunc)`.
- **Usage:** `ai.GenerationUsage{InputTokens, OutputTokens, TotalTokens}`, read from `resp.Usage`.
- **Test fakes:** `genkit.DefineModelAction`. (`DefineModel` is deprecated.)
- **`plugins/compat_oai/openrouter.OpenRouter{APIKey, SiteURL, AppName, Opts}`:**
  - It resolves any `openrouter/<vendor>/<model>` on demand.
  - Structured output goes into the prompt as schema instructions, not native constrained decoding.
  - API errors are wrapped as `status.Error`, with the `*openai.Error` (`StatusCode`) still reachable through `errors.As`.
  - It's built on `github.com/openai/openai-go`, whose client retries 429s, 5xx responses and timeouts twice by default.
- **`plugins/anthropic.Anthropic{APIKey, BaseURL, Opts}`:** uses the native Messages API through `anthropic-sdk-go`, with native structured output. `Init` panics without a key. The plugin's name is fixed as `anthropic`.
- **Reflection (Dev UI) server:** starts only with `GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED`, on 127.0.0.1:3100, with no app authentication.

## Goals / Non-Goals

**Goals:**
- One place (`internal/aikit`) that knows how to build Genkit for Porchlight, which models exist, and how Genkit errors and usage map onto Porchlight's own types.
- The briefing on Genkit indistinguishable from today's, both to members and to the existing tests: `briefing_test.go`, `briefings_test.go` and `routes_test.go` pass unchanged.
- A test kit the agent changes reuse.
- Real-provider behavior for OpenRouter and MiniMax checked by a test you can rerun, not assumed.

**Non-Goals:**
- Moving the other AI calls (describe-a-widget, articles, events) to Genkit. They move when the agent needs them as tools, or in a later cleanup.
- Tools, flows, chat, streaming, sessions, MCP, Genkit's `exp` packages, Gemini.
- Routing kid traffic to zero-data-retention providers. That gate belongs to `kid-tutor-agent`.
- Exposing anything through `genkit.Handler`.

## Decisions

### 1. `internal/aikit` owns Genkit; nothing else imports the plugins
`aikit.New(ctx, aikit.Config) (*Kit, error)` returns a small `Kit` that wraps the `*genkit.Genkit` plus Porchlight's choices:
- the default model name;
- the usage recorder;
- the retry policy.

Plugins:
- `openrouter.OpenRouter{APIKey: cfg.OpenRouterKey, SiteURL: "http://localhost:5174", AppName: "Porchlight", Opts: [option.WithBaseURL(cfg.OpenRouterBaseURL), option.WithMaxRetries(0)]}`.
- `anthropic.Anthropic{APIKey: cfg.MiniMaxKey, BaseURL: cfg.MiniMaxBaseURL, Opts: [option.WithMaxRetries(0)]}`, only when `cfg.MiniMaxKey != ""`. Registering it unconditionally would make `Init` panic.

The base URLs reuse the existing `OPENROUTER_BASE_URL` override, so tests point Genkit at a fake server just as they do the old client.
- *Alternative:* call `genkit.*` from `widgets` directly. That would scatter plugin and model knowledge, make the briefer depend on Genkit, and make the services harder to test.

### 2. Model names
`GENKIT_MODEL` defaults to `openrouter/` + `OPENROUTER_MODEL`, so the briefing keeps using `openai/gpt-4.1-mini` through the same OpenRouter account. `MINIMAX_MODEL` (default `MiniMax-M3.1-Flash`) registers as `anthropic/<id>` when the key is set. The startup check of `OPENROUTER_MODEL` (`CheckModel`) still guards the shared id.

### 3. One retry policy, the same as today's
The SDKs' own retries are turned off (`WithMaxRetries(0)`), because the openai-go default of two retries on 429, 5xx and timeouts would change latency and cost. `Kit` retries exactly as `internal/llm` does:
- one retry after about 1 second on a 429;
- one retry with a correction when the reply can't be used.

Genkit's `middleware.Retry` was considered. It's configurable, but its defaults and backoff differ, and matching today's behavior exactly is what lets the existing tests pass unchanged.

### 4. Shared briefing rules in `internal/llm`
Three functions are exported from `internal/llm`, and the old client is refactored to use them too, so the two implementations can't drift:
- `llm.BriefingPrompt(facts, aud) (system, user string)`
- `llm.CleanBriefing(text string) (string, error)`: normalize, then check empty, over 400 and over 320, then trim. It returns `ErrNoBriefing` on an unusable reply.
- `llm.UpstreamFor(status int, err error) *UpstreamError`: the HTTP status to reason and hint table.

`aikit.Briefer.WriteBriefing` then:
1. calls `genkit.GenerateData[briefingReply]` with the same system and user text;
2. runs `CleanBriefing`, retrying once with a correction if it fails;
3. on error, unwraps `*openai.Error` with `errors.As` and returns `llm.UpstreamFor(apiErr.StatusCode, err)`, or the deadline or cancellation equivalents. Genkit's `status.Error` codes are only a fallback.

The JSON field and its `description` stay the same, so Genkit's schema instructions ask for the same thing.

### 5. Usage through a `WrapModel` middleware
Every `Generate` that `Kit` makes passes `ai.WithUse(k.usageMiddleware)`. That's an `ai.MiddlewareFunc` whose `WrapModel` hook calls `next`, then, when it gets a response, reports `llm.Usage{Model, Step, InputTokens→PromptTokens, OutputTokens→CompletionTokens, TotalTokens}` to the same `llm.UsageRecorder`, with the request's context, which already carries the attribution.
- The step name comes from the caller, for example `morning_briefing`, so the usage rows look exactly like the old client's.
- `WrapModel` runs once per model call, so tool-loop turns in the agent will each be recorded.
- *Alternative:* read `resp.Usage` after `Generate`. That only gives the last turn's usage for tool loops, and every caller would have to remember to do it.

### 6. Choosing the briefer
`PORCHLIGHT_BRIEFER` is `genkit` (the default) or `legacy`. `main.go` passes the chosen implementation as `deps.Briefer`; nothing else changes. Any other value fails at startup with a clear message. The switch is temporary, until the agent work has run on Genkit for a while; removing it is a one-line follow-up.

### 7. Test kit: `aitest.New(t, replies...)`
`aitest.New` returns a `*aikit.Kit` built on a real `genkit.Init` with no plugins and one `DefineModelAction` model, `test/scripted`. That model:
- answers from a queue of scripted replies (text, a JSON value, a tool request, an error with a status);
- reports the given usage;
- records every request it gets, for assertions.

Tests get a real Genkit pipeline (schema instructions, middleware, output parsing) with no network. Plugin wiring is checked separately against an `httptest` OpenRouter fake through the base URL, so the real compat_oai plugin is exercised too.

### 8. Live smoke test, opt-in
`aikit` gets a `live_test.go` guarded by the `PORCHLIGHT_LIVE_AI=1` environment variable; `make ai-smoke` sets it. For OpenRouter, and for MiniMax when its key is set, it runs:
- `GenerateData` into a small struct;
- one tool call, using a toy `add(a, b)` tool;
- a check that usage was reported.

It costs a fraction of a cent. CI never sets the variable. The results go in tasks.md, settling the open MiniMax questions: whether `tool_choice` works and how structured output and thinking behave.

### 9. The Dev UI stays a developer tool
`make dev-ui` runs `GENKIT_ENV=dev npx --yes genkit-cli@<pinned> start -- go run .`, with `CGO_ENABLED=0` and `GOTOOLCHAIN=local` exported. The CLI version is pinned at implementation time. At startup the backend logs a warning when `GENKIT_ENV=dev` or `GENKIT_REFLECTION_ENABLED` is set, saying the reflection server runs any flow without login and must never be on in a deployment. The README says the same.

## Risks / Trade-offs

- [Prompt-based structured output from OpenRouter can fail to parse more often than today's strict `json_schema`] → the retry-with-correction plus `CleanBriefing` cover it. The live smoke test and the demo check show whether gpt-4.1-mini follows the schema instructions reliably. If it doesn't, `openrouter.ChatConfig` can request `response_format` through `Models` overrides, or `PORCHLIGHT_BRIEFER=legacy` stays the default until it's fixed.
- [Genkit's dependency tree is large] → module graph pruning limits what's built. govulncheck in CI keeps it honest. Only stable packages are imported.
- [Genkit API churn: v1.13.0 was retracted, and deprecations land monthly] → it's pinned to v1.14.0, and Genkit is only touched inside `aikit`, so an upgrade changes one package.
- [The reflection server is enabled by accident on the homelab] → it binds only to localhost, the startup warning flags it, and it's documented. Deployment hardening comes with the homelab change.
- [Two implementations of the briefing for a while] → they share prompts and rules through `internal/llm`, and the switch is removed after the agent changes.

## Migration Plan

1. Land everything on `aw-/v2-genkit-integration`, stacked on `aw-/v2-foundation`. Merge after PR #11.
2. The default is `genkit`. If anything misbehaves, set `PORCHLIGHT_BRIEFER=legacy` in `backend/.env` and restart. There's no data migration.
