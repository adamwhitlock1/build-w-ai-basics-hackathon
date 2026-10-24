package aitest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/openai/openai-go"

	"porchlight/internal/aikit/aitest"
)

func TestScriptedModel(t *testing.T) {
	ctx := context.Background()
	kit, model := aitest.New(t,
		aitest.Text("hello").WithUsage(7, 3),
		aitest.JSON(map[string]string{"answer": "42"}),
		aitest.ToolCall("add", map[string]any{"a": 1, "b": 2}),
		aitest.Fail(429),
	)
	g := kit.Genkit()
	if kit.DefaultModel() != aitest.ModelName {
		t.Errorf("DefaultModel = %q, want %q", kit.DefaultModel(), aitest.ModelName)
	}

	t.Run("text, with usage", func(t *testing.T) {
		resp, err := genkit.Generate(ctx, g, ai.WithModelName(aitest.ModelName), ai.WithPrompt("say hello"))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if resp.Text() != "hello" {
			t.Errorf("Text = %q, want hello", resp.Text())
		}
		if resp.Usage == nil || resp.Usage.InputTokens != 7 || resp.Usage.OutputTokens != 3 || resp.Usage.TotalTokens != 10 {
			t.Errorf("Usage = %+v, want 7 in, 3 out, 10 total", resp.Usage)
		}
	})

	t.Run("JSON", func(t *testing.T) {
		type answer struct {
			Answer string `json:"answer"`
		}
		out, _, err := genkit.GenerateData[answer](ctx, g, ai.WithModelName(aitest.ModelName), ai.WithPrompt("what is it?"))
		if err != nil {
			t.Fatalf("GenerateData: %v", err)
		}
		if out.Answer != "42" {
			t.Errorf("Answer = %q, want 42", out.Answer)
		}
	})

	t.Run("tool request", func(t *testing.T) {
		resp, err := genkit.Generate(ctx, g, ai.WithModelName(aitest.ModelName), ai.WithPrompt("add 1 and 2"), ai.WithReturnToolRequests(true))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		reqs := resp.ToolRequests()
		if len(reqs) != 1 || reqs[0].ToolRequest == nil || reqs[0].ToolRequest.Name != "add" {
			t.Fatalf("ToolRequests = %+v, want one call to add", reqs)
		}
	})

	t.Run("error with an HTTP status", func(t *testing.T) {
		_, err := genkit.Generate(ctx, g, ai.WithModelName(aitest.ModelName), ai.WithPrompt("again"))
		var apiErr *openai.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
			t.Fatalf("err = %v, want an *openai.Error with status 429", err)
		}
	})

	t.Run("requests are recorded", func(t *testing.T) {
		reqs := model.Requests()
		if len(reqs) != 4 {
			t.Fatalf("got %d requests, want 4", len(reqs))
		}
		if got := reqs[0].Messages[len(reqs[0].Messages)-1].Text(); got != "say hello" {
			t.Errorf("first request's last message = %q, want say hello", got)
		}
	})
}
