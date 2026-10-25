package aikit_test

import (
	"context"
	"sync"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"kinlight/internal/aikit/aitest"
	"kinlight/internal/llm"
)

// recorded is one usage entry with the attribution its context carried.
type recorded struct {
	usage llm.Usage
	attr  llm.Attribution
	ok    bool
}

// fakeRecorder collects usage.
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

func (r *fakeRecorder) entries() []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recorded(nil), r.got...)
}

func TestTrackUsage(t *testing.T) {
	attr := llm.Attribution{FamilyID: 1, MemberID: 2, ViewerID: 3, Feature: "briefing"}
	ctx := llm.WithAttribution(context.Background(), attr)

	t.Run("one call with usage records one entry", func(t *testing.T) {
		kit, _ := aitest.New(t, aitest.Text("hi").WithUsage(12, 4))
		rec := &fakeRecorder{}
		kit.SetUsageRecorder(rec)
		if _, err := genkit.Generate(ctx, kit.Genkit(), ai.WithModelName(kit.DefaultModel()), ai.WithPrompt("hello"), kit.TrackUsage("greeting")); err != nil {
			t.Fatalf("Generate: %v", err)
		}
		got := rec.entries()
		if len(got) != 1 {
			t.Fatalf("got %d usage entries, want 1", len(got))
		}
		want := llm.Usage{Model: aitest.ModelName, Step: "greeting", PromptTokens: 12, CompletionTokens: 4, TotalTokens: 16}
		if got[0].usage != want {
			t.Errorf("usage = %+v, want %+v", got[0].usage, want)
		}
		if !got[0].ok || got[0].attr != attr {
			t.Errorf("attribution = %+v (present %v), want %+v", got[0].attr, got[0].ok, attr)
		}
	})

	t.Run("a failed call records nothing", func(t *testing.T) {
		kit, _ := aitest.New(t, aitest.Fail(500))
		rec := &fakeRecorder{}
		kit.SetUsageRecorder(rec)
		if _, err := genkit.Generate(ctx, kit.Genkit(), ai.WithModelName(kit.DefaultModel()), ai.WithPrompt("hello"), kit.TrackUsage("greeting")); err == nil {
			t.Fatal("Generate succeeded, want an error")
		}
		if got := rec.entries(); len(got) != 0 {
			t.Errorf("got %d usage entries, want none", len(got))
		}
	})

	t.Run("a nil recorder is fine", func(t *testing.T) {
		kit, _ := aitest.New(t, aitest.Text("hi").WithUsage(1, 1))
		if _, err := genkit.Generate(ctx, kit.Genkit(), ai.WithModelName(kit.DefaultModel()), ai.WithPrompt("hello"), kit.TrackUsage("greeting")); err != nil {
			t.Fatalf("Generate: %v", err)
		}
	})
}
