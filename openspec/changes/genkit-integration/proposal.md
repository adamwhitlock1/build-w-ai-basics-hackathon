## Why

The family agent needs tool calling, multi-turn conversations and streaming. Porchlight's hand-written `internal/llm` client has none of those, and the agent framework chosen for v2 is Genkit for Go. Adopting Genkit by itself first, on a feature that already works and is well tested (the morning briefing), proves that its models, typed output, error handling, usage tracking and test fakes fit this codebase before any agent code depends on them.

## What Changes

- **Genkit for Go v1.14.0 is added**, using only its stable packages:
  - `genkit`, `ai` and `core`;
  - the `compat_oai/openrouter` plugin;
  - the `anthropic` plugin, for MiniMax's Anthropic-compatible API.

  Experimental (`exp`) packages, the Gemini plugin (its terms bar apps likely to be used by under-18s) and Genkit's own HTTP handler are deliberately not used.
- **A new `internal/aikit` package** builds one `*genkit.Genkit` from config:
  - **OpenRouter** is always registered, with the existing `OPENROUTER_API_KEY`.
  - **MiniMax** is registered only when `MINIMAX_API_KEY` is set. It's meant for the owner's own development and the parent assistant, never kid traffic.
  - **The model** comes from config and defaults to the same model the app uses today.
- **The morning briefing moves onto Genkit, with identical behavior:**
  - The same prompts and kid guidance, and the same length rules and trimming, now shared with the old client, so they can't drift apart.
  - The same retry on an unusable reply, and the same error messages: Genkit errors are translated into the existing `llm.UpstreamError`.
  - `PORCHLIGHT_BRIEFER=legacy` switches back to the old client, as a safety valve while Genkit beds in.
- **Usage keeps flowing into `llm_usage`.** A Genkit model middleware records every model call's token counts through the existing usage recorder, attributed through the context exactly like the old client.
- **A fake-model test kit** (`internal/aikit/aitest`) gives tests a real `*genkit.Genkit` with a scripted model, so Genkit code is tested without network or credit. The coming agent changes will use it.
- **An opt-in live smoke test** (`make ai-smoke`) checks the real providers through Genkit: typed output, a tool call and usage, for OpenRouter and (when its key is set) MiniMax. The research couldn't confirm MiniMax's tool and structured-output behavior without a live call.
- **The Genkit Dev UI is dev-only.** A `make dev-ui` target runs the API under Genkit's developer UI, to trace flows and model calls. The backend logs a warning whenever Genkit's reflection server is enabled, because that server runs any flow with no authentication.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The briefing's behavior, kid safety and usage recording keep their existing requirements (`morning-briefing`, `kid-safe-content`, `llm-usage`), so this change sets `skip_specs: true`.

## Impact

- **Backend:**
  - New package `internal/aikit`, with `aitest` inside it.
  - `internal/llm` exports the briefing prompt builder, the reply cleanup and the status-to-message mapping, which both clients share.
  - `main.go` builds Genkit and picks the briefer.
- **Dependencies:** `github.com/firebase/genkit/go` v1.14.0 and what it pulls in, such as `openai-go` and `anthropic-sdk-go`. `go.mod` and `go.sum` grow, and govulncheck has to stay clean.
- **Config:** `PORCHLIGHT_BRIEFER`, `GENKIT_MODEL` (optional), `MINIMAX_API_KEY`, `MINIMAX_BASE_URL` and `MINIMAX_MODEL` (all optional), documented in `.env.example`.
- **Docs:** the README gains a Genkit section covering models, the switch, `make ai-smoke` and `make dev-ui`, and why the Dev UI must never run on the homelab. On archive, one line goes in "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
- **Depends on** `v2-foundation` (the services, usage recorder and CI). This branch is stacked on it, so PR #11 merges first.
