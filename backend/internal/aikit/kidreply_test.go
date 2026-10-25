package aikit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"

	"porchlight/internal/aikit"
	"porchlight/internal/aikit/aitest"
	"porchlight/internal/llm"
)

// newKidAgent returns a KidAgent on the scripted model, with a fast retry.
func newKidAgent(t *testing.T, replies ...aitest.Reply) (*aikit.KidAgent, *aitest.Model, *aikit.Kit) {
	kit, model := aitest.New(t, replies...)
	aikit.SetRetryDelay(kit, time.Millisecond)
	return aikit.NewKidAgent(kit, "", ""), model, kit
}

func kidAsks(text string) aikit.ReplyInput {
	return aikit.ReplyInput{Persona: aikit.PersonaTeen, Mode: aikit.ModeHomework, History: []aikit.Turn{{FromKid: true, Text: text}}}
}

func TestKidAgentReply(t *testing.T) {
	ctx := context.Background()
	const answer = "Good question. What have you tried so far on this one?"

	t.Run("streams chunks that make up the reply", func(t *testing.T) {
		agent, model, kit := newKidAgent(t, aitest.Text(answer).WithUsage(300, 20))
		rec := &fakeRecorder{}
		kit.SetUsageRecorder(rec)
		var chunks []string
		got, err := agent.Reply(ctx, kidAsks("solve 3x + 5 = 20"), func(s string) { chunks = append(chunks, s) })
		if err != nil {
			t.Fatal(err)
		}
		if got != answer || strings.Join(chunks, "") != answer || len(chunks) < 2 {
			t.Errorf("reply = %q from chunks %q; want %q in several chunks", got, chunks, answer)
		}
		req := model.Requests()[0]
		if system := promptText(req.Messages[0]); system != aikit.KidSystemPrompt(aikit.PersonaTeen, aikit.ModeHomework, false, nil) {
			t.Error("the system prompt isn't the kid system prompt for a teen in homework mode")
		}
		if got := rec.entries(); len(got) != 1 || got[0].usage.Step != aikit.StepKidReply {
			t.Errorf("usage = %+v, want one kid_chat_reply entry", got)
		}
	})

	t.Run("sends the history in order, capped at 30 turns", func(t *testing.T) {
		agent, model, _ := newKidAgent(t, aitest.Text("Sure."))
		var history []aikit.Turn
		for i := range 40 {
			history = append(history, aikit.Turn{FromKid: i%2 == 0, Text: fmt.Sprintf("turn %d", i)})
		}
		in := aikit.ReplyInput{Persona: aikit.PersonaYounger, Mode: aikit.ModeExplore, Crisis: true, History: history}
		if _, err := agent.Reply(ctx, in, func(string) {}); err != nil {
			t.Fatal(err)
		}
		msgs := model.Requests()[0].Messages[1:] // after the system prompt
		if len(msgs) != 30 || msgs[0].Text() != "turn 10" || msgs[29].Text() != "turn 39" {
			t.Fatalf("sent %d messages from %q to %q; want turns 10 to 39", len(msgs), msgs[0].Text(), msgs[len(msgs)-1].Text())
		}
		if msgs[0].Role != ai.RoleUser || msgs[1].Role != ai.RoleModel {
			t.Errorf("roles = %s, %s; want the kid as user and the agent as model", msgs[0].Role, msgs[1].Role)
		}
		if system := promptText(model.Requests()[0].Messages[0]); !strings.Contains(system, aikit.CrisisPrompt) {
			t.Error("a crisis reply's system prompt has no crisis block")
		}
	})

	t.Run("a 429 before any text is retried", func(t *testing.T) {
		agent, model, _ := newKidAgent(t, aitest.Fail(429), aitest.Text(answer))
		got, err := agent.Reply(ctx, kidAsks("hi"), func(string) {})
		if err != nil || got != answer {
			t.Errorf("reply = %q, %v; want %q", got, err, answer)
		}
		if n := len(model.Requests()); n != 2 {
			t.Errorf("got %d model calls, want 2", n)
		}
	})

	t.Run("a failure after text isn't retried", func(t *testing.T) {
		agent, model, _ := newKidAgent(t, aitest.StreamThenFail("Let's start with", 429))
		var shown string
		_, err := agent.Reply(ctx, kidAsks("hi"), func(s string) { shown += s })
		var ue *llm.UpstreamError
		if !errors.As(err, &ue) || ue.Status != 429 {
			t.Errorf("err = %v, want an UpstreamError with status 429", err)
		}
		if shown != "Let's start with" {
			t.Errorf("shown = %q, want the text that arrived", shown)
		}
		if n := len(model.Requests()); n != 1 {
			t.Errorf("got %d model calls, want 1", n)
		}
	})

	t.Run("a reply the provider blocks is ErrBlocked", func(t *testing.T) {
		agent, _, _ := newKidAgent(t, aitest.Text("").WithFinish(ai.FinishReasonBlocked))
		if _, err := agent.Reply(ctx, kidAsks("hi"), func(string) {}); !errors.Is(err, aikit.ErrBlocked) {
			t.Errorf("err = %v, want ErrBlocked", err)
		}
	})

	t.Run("an empty reply is an error", func(t *testing.T) {
		agent, _, _ := newKidAgent(t, aitest.Text("  "))
		if _, err := agent.Reply(ctx, kidAsks("hi"), func(string) {}); err == nil {
			t.Error("an empty reply succeeded")
		}
	})
}
