package aikit_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/genkit"
	"github.com/openai/openai-go"

	"kinlight/internal/aikit"
	"kinlight/internal/llm"
)

func TestUpstream(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://openrouter.test/chat/completions", nil)
	sdkErr := func(code int) error {
		return status.Errorf(status.Base(status.FromHTTPCode(code)), "%w", &openai.Error{StatusCode: code, Request: req, Response: &http.Response{StatusCode: code}})
	}
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantReason string
	}{
		{"SDK 401", sdkErr(401), 401, "OpenRouter rejected the API key"},
		{"SDK 402", sdkErr(402), 402, "the OpenRouter account is out of credits"},
		{"SDK 429", sdkErr(429), 429, "OpenRouter is rate limiting requests"},
		{"SDK 500", sdkErr(500), 500, "OpenRouter or the model's provider is having trouble"},
		{"deadline", fmt.Errorf("generate: %w", context.DeadlineExceeded), 0, "the AI model took too long to answer"},
		{"cancelled", fmt.Errorf("generate: %w", context.Canceled), 0, "the request was cancelled"},
		{"Genkit status only", status.Errorf(status.ErrUnavailable, "no provider"), 503, "OpenRouter or the model's provider is having trouble"},
		{"anything else", errors.New("dial tcp: connection refused"), 0, "couldn't reach OpenRouter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ue *llm.UpstreamError
			if !errors.As(aikit.Upstream(tt.err), &ue) {
				t.Fatalf("upstream(%v) isn't an *llm.UpstreamError", tt.err)
			}
			if ue.Status != tt.wantStatus || ue.Reason != tt.wantReason {
				t.Errorf("got status %d, reason %q; want %d, %q", ue.Status, ue.Reason, tt.wantStatus, tt.wantReason)
			}
		})
	}
}

// TestUpstreamFromPlugin sends real requests through the OpenRouter plugin
// to a fake server, so the plugin's own error wrapping is what gets mapped.
func TestUpstreamFromPlugin(t *testing.T) {
	for _, code := range []int{401, 402, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			_, baseURL := newFakeOpenRouter(t, apiFailure(code))
			kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: baseURL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, err = genkit.Generate(context.Background(), kit.Genkit(), ai.WithModelName(kit.DefaultModel()), ai.WithPrompt("hello"))
			if err == nil {
				t.Fatal("Generate succeeded, want an error")
			}
			want := llm.UpstreamFor(code, err)
			var ue *llm.UpstreamError
			if !errors.As(aikit.Upstream(err), &ue) || ue.Status != code || ue.Reason != want.Reason {
				t.Errorf("upstream(%v) = %+v, want status %d, reason %q", err, ue, code, want.Reason)
			}
		})
	}
}

func TestUsageModel(t *testing.T) {
	for in, want := range map[string]string{
		"openrouter/openai/gpt-4.1-mini": "openai/gpt-4.1-mini",
		"anthropic/MiniMax-M3.1-Flash":   "minimax/MiniMax-M3.1-Flash",
		"test/scripted":                  "test/scripted",
	} {
		if got := aikit.UsageModel(in, "fallback"); got != want {
			t.Errorf("usageModel(%q) = %q, want %q", in, got, want)
		}
	}
	if got := aikit.UsageModel("", "openrouter/x/y"); got != "x/y" {
		t.Errorf("usageModel with no name = %q, want the fallback x/y", got)
	}
}
