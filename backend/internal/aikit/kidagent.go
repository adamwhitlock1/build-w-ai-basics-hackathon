package aikit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// Steps under which the kid agent's model calls are recorded.
const (
	StepKidCheck = "kid_chat_check"
	StepKidReply = "kid_chat_reply"
)

// checkTimeout bounds the safety check, retries included: it runs before
// every reply, so it must be quick or give up.
const checkTimeout = 8 * time.Second

// Turn is one message of a conversation.
type Turn struct {
	FromKid bool // false: the agent's reply
	Text    string
}

// Verdict is the safety check's reading of a kid's message.
type Verdict struct {
	Flag     bool
	Category string // one of the Category constants; "" when not flagged
	Severity string // "low", "medium" or "high"; "" when not flagged
	Crisis   bool
	Reason   string // one sentence for a parent
	Title    string // a conversation title, only when asked for
}

// KidAgent is the family agent for kids: the safety check and the replies.
// It must be built on the kids' Kit (see Config.NoRetention).
type KidAgent struct {
	kit         *Kit
	chatModel   string // Genkit model names; "" means the Kit's default
	safetyModel string
}

// NewKidAgent returns a KidAgent on kit that replies with chatModel and
// checks messages with safetyModel (Genkit model names; "" means kit's
// default).
func NewKidAgent(kit *Kit, chatModel, safetyModel string) *KidAgent {
	return &KidAgent{kit: kit, chatModel: chatModel, safetyModel: safetyModel}
}

// verdictReply is the safety model's answer. Category and severity are
// plain strings rather than schema enums: normalizeVerdict maps a near miss
// to "other" or "medium", where an enum would reject the whole answer and
// cost a retry.
type verdictReply struct {
	Flag     bool   `json:"flag" jsonschema_description:"true if a parent should review this message"`
	Category string `json:"category" jsonschema_description:"one of self_harm, abuse, bullying, sexual, violence, drugs, strangers, hate, other; or none when not flagged"`
	Severity string `json:"severity" jsonschema_description:"one of low, medium, high; or none when not flagged"`
	Crisis   bool   `json:"crisis" jsonschema_description:"true only if the kid may be in danger now, or thinking about suicide or self-harm"`
	Reason   string `json:"reason" jsonschema_description:"one plain sentence for the parent, or empty when not flagged"`
	Title    string `json:"title" jsonschema_description:"a 2 to 6 word conversation title when asked for one, otherwise empty"`
}

// Check classifies msg, the kid's latest message, in the light of recent
// turns. With wantTitle (the conversation's first message) it also asks
// for a conversation title. Failures are returned as *llm.UpstreamError;
// the caller decides what failing open means.
func (a *KidAgent) Check(ctx context.Context, recent []Turn, msg string, wantTitle bool) (Verdict, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	system := safetySystemPrompt
	if wantTitle {
		system += safetyTitleRequest
	}
	call := structuredCall{
		step:     StepKidCheck,
		model:    a.safetyModel,
		system:   system,
		messages: []*ai.Message{ai.NewUserTextMessage(checkMessage(recent, msg))},
	}
	r, err := generateJSON(ctx, a.kit, call, func(*verdictReply) error { return nil })
	if err != nil {
		return Verdict{}, fmt.Errorf("check kid message: %w", err)
	}
	return normalizeVerdict(r, wantTitle), nil
}

// checkMessage lays out the recent conversation and the message to check.
func checkMessage(recent []Turn, msg string) string {
	var b strings.Builder
	if len(recent) > 0 {
		b.WriteString("Recent conversation, for context:\n")
		for _, t := range recent {
			who := "Helper"
			if t.FromKid {
				who = "Kid"
			}
			b.WriteString(who + ": " + t.Text + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("The kid's latest message, to classify:\n" + msg)
	return b.String()
}

// normalizeVerdict makes the model's answer consistent: an unknown category
// becomes "other", a crisis is always flagged with high severity, a flag
// always has a severity, and an unflagged message carries no category.
func normalizeVerdict(r verdictReply, wantTitle bool) Verdict {
	v := Verdict{Flag: r.Flag || r.Crisis, Crisis: r.Crisis, Reason: strings.TrimSpace(r.Reason)}
	if wantTitle {
		v.Title = strings.Trim(strings.TrimSpace(r.Title), `"'.`)
	}
	if !v.Flag {
		v.Reason = ""
		return v
	}
	v.Category = r.Category
	if !slices.Contains(modelCategories, v.Category) {
		v.Category = CategoryOther
	}
	v.Severity = r.Severity
	if v.Severity != "low" && v.Severity != "medium" && v.Severity != "high" {
		v.Severity = "medium"
	}
	if v.Crisis {
		v.Severity = "high"
	}
	return v
}
