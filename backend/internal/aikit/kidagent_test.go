package aikit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"

	"porchlight/internal/aikit"
	"porchlight/internal/aikit/aitest"
	"porchlight/internal/llm"
)

const kidGuidanceMarker = "This member is a child or teenager, under 18."

func TestKidSystemPrompt(t *testing.T) {
	tests := []struct {
		persona, mode string
		crisis        bool
		want, notWant []string
	}{
		{aikit.PersonaYounger, aikit.ModeHomework, false,
			[]string{aikit.PersonaYoungerPrompt, aikit.ModeHomeworkPrompt},
			[]string{aikit.PersonaTeenPrompt, aikit.ModeExplorePrompt, aikit.CrisisPrompt}},
		{aikit.PersonaTeen, aikit.ModeExplore, false,
			[]string{aikit.PersonaTeenPrompt, aikit.ModeExplorePrompt},
			[]string{aikit.PersonaYoungerPrompt, aikit.ModeHomeworkPrompt, aikit.CrisisPrompt}},
		{aikit.PersonaTeen, aikit.ModeHomework, true,
			[]string{aikit.PersonaTeenPrompt, aikit.ModeHomeworkPrompt, aikit.CrisisPrompt}, nil},
		// An unknown persona gets the younger one.
		{"", aikit.ModeExplore, false, []string{aikit.PersonaYoungerPrompt}, []string{aikit.PersonaTeenPrompt}},
	}
	for _, tt := range tests {
		got := aikit.KidSystemPrompt(tt.persona, tt.mode, tt.crisis, nil)
		// In every prompt: the hint ladder, the injection rule and the
		// under-18 guidance.
		for _, s := range append(tt.want, aikit.HomeworkRule, aikit.InjectionRule, kidGuidanceMarker) {
			if !strings.Contains(got, s) {
				t.Errorf("%s/%s/crisis=%v: prompt is missing %.50q", tt.persona, tt.mode, tt.crisis, s)
			}
		}
		for _, s := range tt.notWant {
			if strings.Contains(got, s) {
				t.Errorf("%s/%s/crisis=%v: prompt has %.50q", tt.persona, tt.mode, tt.crisis, s)
			}
		}
	}
}

func verdictJSON(flag bool, category, severity string, crisis bool, reason, title string) aitest.Reply {
	return aitest.JSON(map[string]any{"flag": flag, "category": category, "severity": severity, "crisis": crisis, "reason": reason, "title": title})
}

func TestKidAgentCheck(t *testing.T) {
	ctx := context.Background()
	recent := []aikit.Turn{{FromUser: true, Text: "hi"}, {FromUser: false, Text: "Hi! What are we exploring?"}}

	t.Run("clean, with a title", func(t *testing.T) {
		kit, model := aitest.New(t, verdictJSON(false, "none", "none", false, "", "How volcanoes form").WithUsage(50, 10))
		rec := &fakeRecorder{}
		kit.SetUsageRecorder(rec)
		v, err := aikit.NewKidAgent(kit, "", "").Check(ctx, nil, "how do volcanoes form?", true)
		if err != nil {
			t.Fatal(err)
		}
		if v != (aikit.Verdict{Title: "How volcanoes form"}) {
			t.Errorf("verdict = %+v, want unflagged with the title", v)
		}
		req := model.Requests()[0]
		if user := promptText(req.Messages[1]); !strings.HasSuffix(user, aikit.SafetyTitleRequest) {
			t.Error("the first message's check doesn't ask for a title")
		}
		if got := rec.entries(); len(got) != 1 || got[0].usage.Step != aikit.StepKidCheck {
			t.Errorf("usage = %+v, want one kid_chat_check entry", got)
		}
	})

	t.Run("later messages ask for no title, and send the context", func(t *testing.T) {
		kit, model := aitest.New(t, verdictJSON(false, "none", "none", false, "", "Ignored title"))
		v, err := aikit.NewKidAgent(kit, "", "").Check(ctx, recent, "what about lava?", false)
		if err != nil {
			t.Fatal(err)
		}
		if v.Title != "" {
			t.Errorf("title = %q, want none when not asked", v.Title)
		}
		req := model.Requests()[0]
		if strings.Contains(promptText(req.Messages[1]), aikit.SafetyTitleRequest) {
			t.Error("a later message's check asks for a title")
		}
		user := promptText(req.Messages[1])
		if !strings.Contains(user, "Kid: hi") || !strings.Contains(user, "Helper: Hi! What are we exploring?") || !strings.HasSuffix(user, "what about lava?") {
			t.Errorf("check message = %q, want the recent turns then the new message", user)
		}
	})

	t.Run("flagged", func(t *testing.T) {
		kit, _ := aitest.New(t, verdictJSON(true, "strangers", "medium", false, "Someone online asked for their address.", ""))
		v, err := aikit.NewKidAgent(kit, "", "").Check(ctx, recent, "a guy in my game wants my address", false)
		if err != nil {
			t.Fatal(err)
		}
		want := aikit.Verdict{Flag: true, Category: aikit.CategoryStrangers, Severity: "medium", Reason: "Someone online asked for their address."}
		if v != want {
			t.Errorf("verdict = %+v, want %+v", v, want)
		}
	})

	t.Run("a crisis is flagged high even if the model says otherwise", func(t *testing.T) {
		kit, _ := aitest.New(t, verdictJSON(false, "self_harm", "low", true, "They said they don't want to be alive.", ""))
		v, err := aikit.NewKidAgent(kit, "", "").Check(ctx, nil, "i don't want to be alive anymore", false)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Flag || !v.Crisis || v.Severity != "high" || v.Category != aikit.CategorySelfHarm {
			t.Errorf("verdict = %+v, want a high-severity self_harm crisis flag", v)
		}
	})

	t.Run("an unknown category becomes other", func(t *testing.T) {
		kit, _ := aitest.New(t, verdictJSON(true, "gambling", "spicy", false, "Asked about betting.", ""))
		v, err := aikit.NewKidAgent(kit, "", "").Check(ctx, nil, "how do i bet on games", false)
		if err != nil {
			t.Fatal(err)
		}
		if v.Category != aikit.CategoryOther || v.Severity != "medium" {
			t.Errorf("verdict = %+v, want category other and severity medium", v)
		}
	})

	t.Run("a check the provider blocks is ErrBlocked", func(t *testing.T) {
		kit, _ := aitest.New(t, aitest.Text("").WithFinish(ai.FinishReasonBlocked))
		if _, err := aikit.NewKidAgent(kit, "", "").Check(ctx, nil, "something", false); !errors.Is(err, aikit.ErrBlocked) {
			t.Errorf("err = %v, want ErrBlocked", err)
		}
	})

	t.Run("an error is an UpstreamError", func(t *testing.T) {
		kit, _ := aitest.New(t, aitest.Fail(500))
		_, err := aikit.NewKidAgent(kit, "", "").Check(ctx, nil, "hello", false)
		var ue *llm.UpstreamError
		if !errors.As(err, &ue) || ue.Status != 500 {
			t.Errorf("err = %v, want an UpstreamError with status 500", err)
		}
	})
}
