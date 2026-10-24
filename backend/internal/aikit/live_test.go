package aikit_test

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/joho/godotenv"

	"porchlight/internal/aikit"
)

// TestLive checks the real providers: structured output, one tool call, and
// usage reporting, for OpenRouter and (with MINIMAX_API_KEY set) MiniMax.
// It spends a fraction of a cent, so it runs only with PORCHLIGHT_LIVE_AI=1
// (`make ai-smoke`), never in CI. Keys come from the environment or
// backend/.env.
func TestLive(t *testing.T) {
	if os.Getenv("PORCHLIGHT_LIVE_AI") != "1" {
		t.Skip("set PORCHLIGHT_LIVE_AI=1 (make ai-smoke) to call the real providers")
	}
	// Like main, values already in the environment win over the file.
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("read backend/.env: %v", err)
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Fatal("OPENROUTER_API_KEY isn't set")
	}
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-4.1-mini"
	}
	minimaxURL := os.Getenv("MINIMAX_BASE_URL")
	if minimaxURL == "" {
		minimaxURL = "https://api.minimax.io/anthropic"
	}
	kit, err := aikit.New(context.Background(), aikit.Config{
		OpenRouterKey:   os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel: model,
		MiniMaxKey:      os.Getenv("MINIMAX_API_KEY"),
		MiniMaxBaseURL:  minimaxURL,
		MiniMaxModel:    os.Getenv("MINIMAX_MODEL"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := &fakeRecorder{}
	kit.SetUsageRecorder(rec)

	g := kit.Genkit()
	type addInput struct {
		A int `json:"a" jsonschema_description:"The first number"`
		B int `json:"b" jsonschema_description:"The second number"`
	}
	var adds atomic.Int32
	add := genkit.DefineTool(g, "add", "Adds two whole numbers and returns their sum.",
		func(ctx *ai.ToolContext, in addInput) (int, error) {
			adds.Add(1)
			return in.A + in.B, nil
		})

	models := map[string]string{"OpenRouter": kit.DefaultModel()}
	if kit.MiniMaxModel() != "" {
		models["MiniMax"] = kit.MiniMaxModel()
	} else {
		t.Log("MiniMax: skipped (no MINIMAX_API_KEY)")
	}
	for provider, name := range models {
		t.Run(provider, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			t.Run("structured output", func(t *testing.T) {
				type capital struct {
					City    string `json:"city" jsonschema_description:"The capital city"`
					Country string `json:"country" jsonschema_description:"The country"`
				}
				before := len(rec.entries())
				out, _, err := genkit.GenerateData[capital](ctx, g,
					ai.WithModelName(name),
					ai.WithSystem("Answer briefly."),
					ai.WithPrompt("What is the capital of France?"),
					ai.WithConfig(map[string]any{"temperature": 0.2}),
					kit.TrackUsage("live_smoke"),
				)
				if err != nil {
					t.Fatalf("GenerateData: %v", err)
				}
				if !strings.Contains(out.City, "Paris") {
					t.Errorf("city = %q, want Paris", out.City)
				}
				t.Logf("%s structured output: %+v", name, *out)
				checkUsage(t, rec, before)
			})

			t.Run("one tool call", func(t *testing.T) {
				before := len(rec.entries())
				adds.Store(0)
				resp, err := genkit.Generate(ctx, g,
					ai.WithModelName(name),
					ai.WithPrompt("Use the add tool to add 2 and 3, then tell me the result."),
					ai.WithTools(add),
					ai.WithMaxTurns(3),
					kit.TrackUsage("live_smoke"),
				)
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				if adds.Load() < 1 {
					t.Errorf("the add tool wasn't called; the model said %q", resp.Text())
				}
				if !strings.Contains(resp.Text(), "5") {
					t.Errorf("answer = %q, want it to mention 5", resp.Text())
				}
				t.Logf("%s tool answer: %q (add called %d times)", name, resp.Text(), adds.Load())
				if n := len(rec.entries()) - before; n < 2 {
					t.Errorf("recorded %d usage entries, want one per model call (at least 2)", n)
				}
			})

			t.Run("tool choice required", func(t *testing.T) {
				resp, err := genkit.Generate(ctx, g,
					ai.WithModelName(name),
					ai.WithPrompt("What is 2 plus 3?"),
					ai.WithTools(add),
					ai.WithToolChoice(ai.ToolChoiceRequired),
					ai.WithReturnToolRequests(true),
				)
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				if len(resp.ToolRequests()) == 0 {
					t.Errorf("tool_choice=required, but the model answered %q without calling a tool", resp.Text())
				}
			})
		})
	}
}

// checkUsage checks that a usage entry with token counts was recorded after
// the first before entries.
func checkUsage(t *testing.T, rec *fakeRecorder, before int) {
	t.Helper()
	got := rec.entries()[before:]
	if len(got) == 0 {
		t.Fatal("no usage recorded")
	}
	u := got[len(got)-1].usage
	if u.PromptTokens == 0 || u.CompletionTokens == 0 || u.TotalTokens == 0 {
		t.Errorf("usage = %+v, want token counts", u)
	}
	t.Logf("usage: %+v", u)
}
