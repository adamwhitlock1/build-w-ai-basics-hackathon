package aikit

import (
	"context"
	"strings"

	"github.com/firebase/genkit/go/ai"

	"porchlight/internal/llm"
)

// TrackUsage returns the option every Kit generate call passes: middleware
// that reports each model answer's token usage to the Kit's recorder under
// step (e.g. "morning_briefing"), with the call's context, which carries the
// llm.Attribution. It runs once per model call, so each turn of a tool loop
// is recorded.
func (k *Kit) TrackUsage(step string) ai.GenerateOption {
	return ai.WithUse(ai.MiddlewareFunc(func(context.Context) (*ai.Hooks, error) {
		// New runs once per Generate call, so model is this call's own.
		// WrapGenerate always runs before the WrapModel calls inside it.
		var model string
		return &ai.Hooks{
			WrapGenerate: func(ctx context.Context, p *ai.GenerateParams, next ai.GenerateNext) (*ai.ModelResponse, error) {
				if p.Options != nil {
					model = p.Options.Model
				}
				return next(ctx, p)
			},
			WrapModel: func(ctx context.Context, p *ai.ModelParams, next ai.ModelNext) (*ai.ModelResponse, error) {
				resp, err := next(ctx, p)
				// The provider answered, so it counted tokens, even if Genkit
				// later finds the answer doesn't match the schema.
				if err == nil && resp != nil && resp.Usage != nil && k.usage != nil {
					k.usage.RecordUsage(ctx, llm.Usage{
						Model:            usageModel(model, k.defaultModel),
						Step:             step,
						PromptTokens:     resp.Usage.InputTokens,
						CompletionTokens: resp.Usage.OutputTokens,
						TotalTokens:      resp.Usage.TotalTokens,
					})
				}
				return resp, err
			},
		}, nil
	}))
}

// usageModel names the model in usage records the way the legacy client
// does. OpenRouter ids already name the vendor ("openai/gpt-4.1-mini"), so
// the "openrouter/" prefix goes; in Porchlight the anthropic plugin serves
// only MiniMax, so its prefix becomes "minimax/".
func usageModel(name, fallback string) string {
	if name == "" {
		name = fallback
	}
	if rest, ok := strings.CutPrefix(name, "openrouter/"); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(name, "anthropic/"); ok {
		return "minimax/" + rest
	}
	return name
}
