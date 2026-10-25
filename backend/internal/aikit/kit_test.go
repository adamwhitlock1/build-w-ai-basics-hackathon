package aikit_test

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/genkit"

	"kinlight/internal/aikit"
)

func TestNew(t *testing.T) {
	t.Run("without MiniMax", func(t *testing.T) {
		kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "openai/gpt-4.1-mini"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if got, want := kit.DefaultModel(), "openrouter/openai/gpt-4.1-mini"; got != want {
			t.Errorf("DefaultModel = %q, want %q", got, want)
		}
		if genkit.LookupModel(kit.Genkit(), kit.DefaultModel()) == nil {
			t.Error("the default model doesn't resolve")
		}
		if kit.MiniMaxModel() != "" {
			t.Errorf("MiniMaxModel = %q, want none", kit.MiniMaxModel())
		}
		if genkit.LookupModel(kit.Genkit(), "anthropic/"+aikit.DefaultMiniMaxModel) != nil {
			t.Error("an anthropic model resolves without a MiniMax key")
		}
	})

	t.Run("with MiniMax", func(t *testing.T) {
		kit, err := aikit.New(context.Background(), aikit.Config{
			OpenRouterKey:   "test-key",
			OpenRouterModel: "openai/gpt-4.1-mini",
			MiniMaxKey:      "test-minimax-key",
			MiniMaxBaseURL:  "https://minimax.test/anthropic",
			MiniMaxModel:    "MiniMax-Test",
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if got, want := kit.MiniMaxModel(), "anthropic/MiniMax-Test"; got != want {
			t.Errorf("MiniMaxModel = %q, want %q", got, want)
		}
		if genkit.LookupModel(kit.Genkit(), kit.MiniMaxModel()) == nil {
			t.Error("the MiniMax model doesn't resolve")
		}
	})

	t.Run("explicit default model", func(t *testing.T) {
		kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "openai/gpt-4.1-mini", DefaultModel: "openrouter/openai/gpt-4o-mini"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if got := kit.DefaultModel(); got != "openrouter/openai/gpt-4o-mini" {
			t.Errorf("DefaultModel = %q", got)
		}
	})

	t.Run("missing config is an error", func(t *testing.T) {
		for _, cfg := range []aikit.Config{
			{OpenRouterModel: "openai/gpt-4.1-mini"},
			{OpenRouterKey: "test-key"},
		} {
			if _, err := aikit.New(context.Background(), cfg); err == nil {
				t.Errorf("New(%+v) succeeded, want an error", cfg)
			}
		}
	})

	t.Run("a Genkit panic is an error", func(t *testing.T) {
		t.Setenv("GENKIT_REFLECTION_ENABLED", "not-a-bool")
		if _, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "openai/gpt-4.1-mini"}); err == nil {
			t.Error("New succeeded with an invalid GENKIT_REFLECTION_ENABLED, want an error")
		}
	})
}
