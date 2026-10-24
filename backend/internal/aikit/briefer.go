package aikit

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
)

// briefingStep names the briefing's model calls in usage records and logs,
// the same as the legacy client.
const briefingStep = "morning_briefing"

// briefingReply is the model's answer. Its JSON field and description match
// internal/llm's, so the model is asked for the same thing. Genkit reads
// jsonschema_description (a plain jsonschema tag would split at the comma).
type briefingReply struct {
	Briefing string `json:"briefing" jsonschema_description:"1 to 3 short, warm sentences about the member's day, with no greeting and no name"`
}

// Briefer writes morning briefings through Genkit. It implements
// widgets.Briefer and behaves like llm.Client's WriteBriefing: the same
// prompts, length rules, retries and errors.
type Briefer struct{ k *Kit }

// NewBriefer returns a Briefer that uses k's default model.
func NewBriefer(k *Kit) *Briefer { return &Briefer{k: k} }

// WriteBriefing asks the model for a member's morning briefing: 1 to 3
// short sentences built only from facts, at most llm.MaxBriefingChars long.
// For sources.Kids the model is told the member is under 18. Failed calls
// are returned as *llm.UpstreamError; a reply that stays blank or far too
// long wraps llm.ErrNoBriefing.
func (b *Briefer) WriteBriefing(ctx context.Context, facts llm.BriefingFacts, aud sources.Audience) (string, error) {
	system, user := llm.BriefingPrompt(facts, aud)
	call := structuredCall{step: briefingStep, system: system, messages: []*ai.Message{ai.NewUserTextMessage(user)}}
	out, err := generateJSON(ctx, b.k, call, func(r *briefingReply) error {
		text, err := llm.CleanBriefing(r.Briefing)
		r.Briefing = text
		return err
	})
	if err != nil {
		return "", fmt.Errorf("write briefing: %w", err)
	}
	return out.Briefing, nil
}
