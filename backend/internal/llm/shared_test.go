package llm_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
)

func TestCleanBriefing(t *testing.T) {
	// 3 sentences of 100 characters each, then a fourth that runs past 320.
	sentence := strings.Repeat("a", 98) + ". "
	long := strings.Repeat(sentence, 3) + "And then one more sentence that pushes it over."

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "", wantErr: true},
		{name: "whitespace only", in: " \n\t ", wantErr: true},
		{name: "normal", in: "  Sunny and 75.\n Soccer at 4.  ", want: "Sunny and 75. Soccer at 4."},
		{name: "over 320 is cut at a sentence end", in: long, want: strings.TrimSpace(strings.Repeat(sentence, 3))},
		{name: "over 400 is rejected", in: strings.Repeat("word ", 90), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := llm.CleanBriefing(tt.in)
			if tt.wantErr {
				if !errors.Is(err, llm.ErrNoBriefing) {
					t.Fatalf("CleanBriefing(%q) error = %v, want ErrNoBriefing", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanBriefing: %v", err)
			}
			if got != tt.want {
				t.Errorf("CleanBriefing = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBriefingPrompt(t *testing.T) {
	facts := fullFacts()
	system, user := llm.BriefingPrompt(facts, sources.Kids)
	if !strings.Contains(system, llm.KidGuidance) {
		t.Error("kids' system prompt has no kid guidance")
	}
	if user != llm.BriefingUserMessage(facts) {
		t.Errorf("user message = %q, want the facts laid out", user)
	}
	system, _ = llm.BriefingPrompt(facts, sources.Everyone)
	if strings.Contains(system, llm.KidGuidance) {
		t.Error("everyone's system prompt has kid guidance")
	}
}

func TestUpstreamFor(t *testing.T) {
	cause := errors.New("boom")
	tests := []struct {
		name       string
		status     int
		err        error
		wantReason string
	}{
		{"bad key", 401, cause, "OpenRouter rejected the API key"},
		{"no credits", 402, cause, "the OpenRouter account is out of credits"},
		{"rate limited", 429, cause, "OpenRouter is rate limiting requests"},
		{"server error", 500, cause, "OpenRouter or the model's provider is having trouble"},
		{"unknown 4xx", 418, cause, "OpenRouter returned HTTP 418"},
		{"deadline", 0, fmt.Errorf("call: %w", context.DeadlineExceeded), "the AI model took too long to answer"},
		{"cancelled", 0, fmt.Errorf("call: %w", context.Canceled), "the request was cancelled"},
		{"no answer", 0, cause, "couldn't reach OpenRouter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := llm.UpstreamFor(tt.status, tt.err)
			if e.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", e.Reason, tt.wantReason)
			}
			if e.Status != tt.status {
				t.Errorf("Status = %d, want %d", e.Status, tt.status)
			}
			if !errors.Is(e, tt.err) {
				t.Error("the cause isn't reachable with errors.Is")
			}
		})
	}
}
