package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
)

// recorded is one RecordUsage call: the usage and the attribution its
// context carried.
type recorded struct {
	usage       llm.Usage
	attribution llm.Attribution
	attributed  bool
}

// fakeRecorder collects what the Client reports.
type fakeRecorder struct {
	mu  sync.Mutex
	got []recorded
}

func (r *fakeRecorder) RecordUsage(ctx context.Context, u llm.Usage) {
	a, ok := llm.AttributionFrom(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, recorded{u, a, ok})
}

// withUsage is a successful completion whose body reports token usage.
func withUsage(content string, prompt, completion int) reply {
	body, _ := json.Marshal(map[string]any{
		"id": "gen-1", "object": "chat.completion", "model": "test/model-answered",
		"choices": []map[string]any{{
			"index": 0, "message": map[string]string{"role": "assistant", "content": content}, "finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion},
	})
	return reply{status: http.StatusOK, body: string(body)}
}

func TestUsageIsRecordedPerAnswer(t *testing.T) {
	// An unusable first answer still cost tokens, so both answers count.
	_, client := newFake(t, withUsage(`{"title": "Spa`, 100, 5), withUsage(validSpec, 120, 30))
	rec := &fakeRecorder{}
	client.SetUsageRecorder(rec)

	who := llm.Attribution{FamilyID: 1, MemberID: 3, ViewerID: 2, Feature: "describe_widget"}
	ctx := llm.WithAttribution(context.Background(), who)
	if _, err := client.PickWidgetSpec(ctx, "show me upcoming SpaceX launches", testCatalog, sources.Everyone, ""); err != nil {
		t.Fatal(err)
	}

	want := []recorded{
		{llm.Usage{Model: "test/model-answered", Step: "widget_spec", PromptTokens: 100, CompletionTokens: 5, TotalTokens: 105}, who, true},
		{llm.Usage{Model: "test/model-answered", Step: "widget_spec", PromptTokens: 120, CompletionTokens: 30, TotalTokens: 150}, who, true},
	}
	if !slices.Equal(rec.got, want) {
		t.Errorf("recorded =\n%+v\nwant\n%+v", rec.got, want)
	}
}

func TestUsageNotRecordedWithoutAnAnswer(t *testing.T) {
	// A server that's already gone: the call never reaches a provider.
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	client := llm.NewClient("test-key", "test/model", srv.URL)
	rec := &fakeRecorder{}
	client.SetUsageRecorder(rec)

	if _, err := client.PickWidgetSpec(context.Background(), "anything", testCatalog, sources.Everyone, ""); err == nil {
		t.Fatal("PickWidgetSpec succeeded against a closed server")
	}
	if len(rec.got) != 0 {
		t.Errorf("recorded %d entries for a call that never got an answer, want 0", len(rec.got))
	}
}

func TestUsageWithoutAttribution(t *testing.T) {
	_, client := newFake(t, withUsage(validSpec, 10, 2))
	rec := &fakeRecorder{}
	client.SetUsageRecorder(rec)
	if _, err := client.PickWidgetSpec(context.Background(), "x", testCatalog, sources.Everyone, ""); err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 || rec.got[0].attributed {
		t.Errorf("recorded = %+v, want one entry with no attribution", rec.got)
	}
}
