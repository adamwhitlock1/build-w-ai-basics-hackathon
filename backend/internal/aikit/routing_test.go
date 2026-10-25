package aikit_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"kinlight/internal/aikit"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
)

// TestNoRetentionRouting checks the request OpenRouter actually receives:
// a no-retention Kit asks for zero-data-retention endpoints of providers
// that don't collect data, and an ordinary Kit asks for no routing at all.
func TestNoRetentionRouting(t *testing.T) {
	for _, noRetention := range []bool{true, false} {
		fake, baseURL := newFakeOpenRouter(t, completion(`{"briefing":"Sunny today."}`, 10, 5))
		kit, err := aikit.New(context.Background(), aikit.Config{
			OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: baseURL, NoRetention: noRetention,
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if kit.NoRetention() != noRetention {
			t.Errorf("NoRetention() = %v, want %v", kit.NoRetention(), noRetention)
		}
		if _, err := aikit.NewBriefer(kit).WriteBriefing(context.Background(), testFacts(), sources.Kids); err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		bodies := fake.requestBodies()
		if len(bodies) != 1 {
			t.Fatalf("got %d requests, want 1", len(bodies))
		}
		provider, hasProvider := bodies[0]["provider"].(map[string]any)
		switch {
		case noRetention && (!hasProvider || provider["zdr"] != true || provider["data_collection"] != "deny"):
			t.Errorf("no-retention request's provider = %v, want zdr true and data_collection deny", bodies[0]["provider"])
		case !noRetention && bodies[0]["provider"] != nil:
			t.Errorf("ordinary request's provider = %v, want none", bodies[0]["provider"])
		}
	}
}

// TestNoEndpoints checks OpenRouter's 404 for a model with no endpoint
// under the routing: the error names the settings to change.
func TestNoEndpoints(t *testing.T) {
	_, baseURL := newFakeOpenRouter(t, apiFailure(http.StatusNotFound))
	kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: baseURL, NoRetention: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = aikit.NewBriefer(kit).WriteBriefing(context.Background(), testFacts(), sources.Kids)
	var ue *llm.UpstreamError
	if !errors.As(err, &ue) || ue.Status != http.StatusNotFound || !strings.Contains(ue.Hint, "KIDS_CHAT_MODEL") {
		t.Errorf("err = %v, want an UpstreamError with status 404 whose hint names KIDS_CHAT_MODEL", err)
	}
}
