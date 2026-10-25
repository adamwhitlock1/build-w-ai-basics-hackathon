package aikit_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"

	"kinlight/internal/aikit"
	"kinlight/internal/aikit/aitest"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
)

var briefingMorning = time.Date(2026, 10, 13, 7, 10, 0, 0, time.UTC)

func testFacts() llm.BriefingFacts {
	return llm.BriefingFacts{
		Now:     briefingMorning,
		Weather: "Sunny. High 75°F, low 58°F. 10% chance of rain.",
		Today:   []llm.BriefingEvent{{Time: "4:00 PM", Title: "Soccer practice", Location: "Lincoln Park"}},
	}
}

func briefing(text string) aitest.Reply { return aitest.JSON(map[string]string{"briefing": text}) }

// newBriefer returns a Briefer on the scripted model, with a fast retry.
func newBriefer(t *testing.T, replies ...aitest.Reply) (*aikit.Briefer, *aitest.Model, *aikit.Kit) {
	kit, model := aitest.New(t, replies...)
	aikit.SetRetryDelay(kit, time.Millisecond)
	return aikit.NewBriefer(kit), model, kit
}

// promptText returns the text of a message's parts, leaving out the output
// instructions Genkit adds.
func promptText(m *ai.Message) string {
	var b strings.Builder
	for _, p := range m.Content {
		if !p.IsOutputInstructions() {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func TestBrieferWriteBriefing(t *testing.T) {
	ctx := context.Background()

	t.Run("a clean reply", func(t *testing.T) {
		b, model, _ := newBriefer(t, briefing("  Sunny and 75 today.\n Soccer at 4 in Lincoln Park. "))
		got, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone)
		if err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if want := "Sunny and 75 today. Soccer at 4 in Lincoln Park."; got != want {
			t.Errorf("briefing = %q, want %q", got, want)
		}
		reqs := model.Requests()
		if len(reqs) != 1 {
			t.Fatalf("got %d model calls, want 1", len(reqs))
		}
		system, user := llm.BriefingPrompt(testFacts(), sources.Everyone)
		msgs := reqs[0].Messages
		if len(msgs) != 2 || msgs[0].Role != ai.RoleSystem || promptText(msgs[0]) != system || promptText(msgs[1]) != user {
			t.Errorf("the model wasn't sent the shared briefing prompt: %+v", msgs)
		}
	})

	t.Run("an unusable reply, then a good one", func(t *testing.T) {
		b, model, _ := newBriefer(t, briefing(""), briefing("Sunny today."))
		got, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone)
		if err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if got != "Sunny today." {
			t.Errorf("briefing = %q", got)
		}
		reqs := model.Requests()
		if len(reqs) != 2 {
			t.Fatalf("got %d model calls, want 2", len(reqs))
		}
		last := reqs[1].Messages[len(reqs[1].Messages)-1]
		if !strings.Contains(last.Text(), "That reply was invalid") || !strings.Contains(last.Text(), "empty") {
			t.Errorf("the retry didn't explain the problem: %q", last.Text())
		}
	})

	t.Run("a reply that isn't JSON, then a good one", func(t *testing.T) {
		b, model, _ := newBriefer(t, aitest.Text("Sure! Here's your briefing."), briefing("Sunny today."))
		if _, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone); err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if n := len(model.Requests()); n != 2 {
			t.Errorf("got %d model calls, want 2", n)
		}
	})

	t.Run("unusable twice", func(t *testing.T) {
		b, _, _ := newBriefer(t, briefing(""), briefing(strings.Repeat("word ", 90)))
		_, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone)
		if !errors.Is(err, llm.ErrNoBriefing) {
			t.Errorf("err = %v, want ErrNoBriefing", err)
		}
	})

	t.Run("a 429, then success", func(t *testing.T) {
		b, model, _ := newBriefer(t, aitest.Fail(429), briefing("Sunny today."))
		if _, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone); err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		if n := len(model.Requests()); n != 2 {
			t.Errorf("got %d model calls, want 2", n)
		}
	})

	t.Run("429 twice", func(t *testing.T) {
		b, _, _ := newBriefer(t, aitest.Fail(429), aitest.Fail(429))
		_, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone)
		var ue *llm.UpstreamError
		if !errors.As(err, &ue) || ue.Status != 429 || !strings.Contains(ue.Reason, "rate limiting") {
			t.Errorf("err = %v, want an UpstreamError about rate limiting", err)
		}
	})

	t.Run("other API errors aren't retried", func(t *testing.T) {
		b, model, _ := newBriefer(t, aitest.Fail(401))
		_, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone)
		var ue *llm.UpstreamError
		if !errors.As(err, &ue) || ue.Status != 401 {
			t.Errorf("err = %v, want an UpstreamError with status 401", err)
		}
		if n := len(model.Requests()); n != 1 {
			t.Errorf("got %d model calls, want 1", n)
		}
	})

	t.Run("kid guidance follows the audience", func(t *testing.T) {
		for _, tt := range []struct {
			aud  sources.Audience
			want bool
		}{{sources.Kids, true}, {sources.Everyone, false}} {
			b, model, _ := newBriefer(t, briefing("Sunny today."))
			if _, err := b.WriteBriefing(ctx, testFacts(), tt.aud); err != nil {
				t.Fatalf("WriteBriefing: %v", err)
			}
			system := promptText(model.Requests()[0].Messages[0])
			if got := strings.Contains(system, "This member is a child or teenager, under 18."); got != tt.want {
				t.Errorf("audience %v: kid guidance in system prompt = %v, want %v", tt.aud, got, tt.want)
			}
		}
	})

	t.Run("usage is recorded once per model call", func(t *testing.T) {
		b, _, kit := newBriefer(t, briefing("").WithUsage(100, 5), briefing("Sunny today.").WithUsage(120, 8))
		rec := &fakeRecorder{}
		kit.SetUsageRecorder(rec)
		if _, err := b.WriteBriefing(ctx, testFacts(), sources.Everyone); err != nil {
			t.Fatalf("WriteBriefing: %v", err)
		}
		got := rec.entries()
		if len(got) != 2 {
			t.Fatalf("got %d usage entries, want 2", len(got))
		}
		for i, wantPrompt := range []int{100, 120} {
			if got[i].usage.Step != "morning_briefing" || got[i].usage.PromptTokens != wantPrompt {
				t.Errorf("entry %d = %+v, want step morning_briefing with %d prompt tokens", i, got[i].usage, wantPrompt)
			}
		}
	})
}
