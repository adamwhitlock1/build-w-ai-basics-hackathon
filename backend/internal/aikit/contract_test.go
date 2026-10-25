package aikit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"kinlight/internal/aikit"
	"kinlight/internal/aikit/aitest"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
)

// scripted is one model answer in a contract scenario: reply text, or an
// HTTP error status.
type scripted struct {
	text   string
	status int
}

func answer(briefing string) scripted {
	b, _ := json.Marshal(map[string]string{"briefing": briefing})
	return scripted{text: string(b)}
}

// outcome is what a briefer produced, in comparable form.
type outcome struct {
	text       string
	noBriefing bool // the error wraps llm.ErrNoBriefing
	status     int  // the UpstreamError's status, if any
	reason     string
	failed     bool
}

func outcomeOf(text string, err error) outcome {
	if err == nil {
		return outcome{text: text}
	}
	o := outcome{failed: true, noBriefing: errors.Is(err, llm.ErrNoBriefing)}
	var ue *llm.UpstreamError
	if errors.As(err, &ue) {
		o.status, o.reason = ue.Status, ue.Reason
	}
	return o
}

// turn is one message sent to the model, with Genkit's roles mapped onto
// the OpenAI API's.
type turn struct{ role, text string }

// unreadable matches the parser's own words inside a correction message.
// encoding/json and Genkit's parser describe bad JSON differently; that
// detail is the one part of the messages allowed to differ.
var unreadable = regexp.MustCompile(`(?s)(couldn't read: ).*?(\)\. Reply again)`)

func newTurn(role, text string) turn {
	return turn{role, unreadable.ReplaceAllString(text, "${1}…${2}")}
}

// TestBriefersAgree runs the same scripted answers through the legacy
// client (against a fake OpenRouter) and the Genkit briefer (against the
// scripted model), and checks that they produce the same briefing or error
// and send the same messages. The Genkit briefer also runs through the real
// OpenRouter plugin against the fake, to check the outcome end to end.
func TestBriefersAgree(t *testing.T) {
	long := strings.Repeat(strings.Repeat("a", 98)+". ", 3) + "And then one more sentence that pushes it over."
	scenarios := []struct {
		name    string
		aud     sources.Audience
		answers []scripted
	}{
		{"clean", sources.Everyone, []scripted{answer("Sunny and 75 today. Soccer at 4.")}},
		{"kids", sources.Kids, []scripted{answer("Sunny today. Soccer at 4.")}},
		{"trimmed at a sentence end", sources.Everyone, []scripted{answer(long)}},
		{"empty, then good", sources.Everyone, []scripted{answer(""), answer("Sunny today.")}},
		{"not JSON, then good", sources.Everyone, []scripted{{text: "Sure! Here you go."}, answer("Sunny today.")}},
		{"empty twice", sources.Everyone, []scripted{answer(""), answer(" ")}},
		{"far too long twice", sources.Everyone, []scripted{answer(strings.Repeat("word ", 90)), answer(strings.Repeat("word ", 95))}},
		{"429, then good", sources.Everyone, []scripted{{status: 429}, answer("Sunny today.")}},
		{"429 twice", sources.Everyone, []scripted{{status: 429}, {status: 429}}},
		{"bad key", sources.Everyone, []scripted{{status: 401}}},
		{"no credits", sources.Everyone, []scripted{{status: 402}}},
		{"provider trouble", sources.Everyone, []scripted{{status: 500}}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel() // the legacy client's 429 pause is a full second
			ctx := context.Background()
			facts := testFacts()

			// Legacy: llm.Client against the fake OpenRouter.
			var legacyReplies []reply
			for _, a := range sc.answers {
				if a.status != 0 {
					legacyReplies = append(legacyReplies, apiFailure(a.status))
				} else {
					legacyReplies = append(legacyReplies, completion(a.text, 10, 5))
				}
			}
			fake, baseURL := newFakeOpenRouter(t, legacyReplies...)
			legacyText, legacyErr := llm.NewClient("test-key", "test/model", baseURL).WriteBriefing(ctx, facts, sc.aud)
			legacy := outcomeOf(legacyText, legacyErr)

			// Genkit: aikit.Briefer against the scripted model.
			var scriptedReplies []aitest.Reply
			for _, a := range sc.answers {
				if a.status != 0 {
					scriptedReplies = append(scriptedReplies, aitest.Fail(a.status))
				} else {
					scriptedReplies = append(scriptedReplies, aitest.Text(a.text))
				}
			}
			b, model, _ := newBriefer(t, scriptedReplies...)
			genkitText, genkitErr := b.WriteBriefing(ctx, facts, sc.aud)
			gk := outcomeOf(genkitText, genkitErr)

			if legacy != gk {
				t.Errorf("outcomes differ:\nlegacy: %+v (%v)\ngenkit: %+v (%v)", legacy, legacyErr, gk, genkitErr)
			}

			// The same messages, call by call.
			var legacyTurns, genkitTurns [][]turn
			for _, msgs := range fake.requests() {
				var turns []turn
				for _, m := range msgs {
					turns = append(turns, newTurn(m.Role, fmt.Sprint(m.Content)))
				}
				legacyTurns = append(legacyTurns, turns)
			}
			for _, req := range model.Requests() {
				var turns []turn
				for _, m := range req.Messages {
					role := string(m.Role)
					if role == "model" {
						role = "assistant"
					}
					turns = append(turns, newTurn(role, promptText(m)))
				}
				genkitTurns = append(genkitTurns, turns)
			}
			if !slices.EqualFunc(legacyTurns, genkitTurns, slices.Equal) {
				t.Errorf("messages differ:\nlegacy: %q\ngenkit: %q", legacyTurns, genkitTurns)
			}

			// Genkit end to end: the real OpenRouter plugin against the fake.
			_, pluginURL := newFakeOpenRouter(t, legacyReplies...)
			kit, err := aikit.New(ctx, aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: pluginURL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			aikit.SetRetryDelay(kit, time.Millisecond)
			pluginText, pluginErr := aikit.NewBriefer(kit).WriteBriefing(ctx, facts, sc.aud)
			if plugin := outcomeOf(pluginText, pluginErr); plugin != legacy {
				t.Errorf("outcomes differ through the plugin:\nlegacy: %+v (%v)\nplugin: %+v (%v)", legacy, legacyErr, plugin, pluginErr)
			}
		})
	}
}
