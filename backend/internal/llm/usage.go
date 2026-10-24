package llm

import "context"

// Usage is one model answer's token counts, as the provider reported them.
type Usage struct {
	Model            string // the model that answered
	Step             string // which call within a feature, e.g. "widget_spec"
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Attribution says whom an AI call is for: the feature it serves, the family
// member whose content it produces, and the viewer who caused it (a parent
// opening a kid's page is the viewer; the kid is the member).
type Attribution struct {
	FamilyID int64
	MemberID int64
	ViewerID int64
	Feature  string // e.g. "describe_widget", "articles", "briefing"
}

// UsageRecorder receives the usage of every model answer the Client gets.
// RecordUsage must return quickly and never fail the call: it's handed the
// request's context, which may carry an Attribution (see AttributionFrom).
type UsageRecorder interface {
	RecordUsage(ctx context.Context, u Usage)
}

// attributionKey is the context key for an Attribution. An unexported type
// means no other package can read or overwrite the value by accident.
type attributionKey struct{}

// WithAttribution returns a copy of ctx that says whom the AI calls made with
// it are for.
func WithAttribution(ctx context.Context, a Attribution) context.Context {
	return context.WithValue(ctx, attributionKey{}, a)
}

// AttributionFrom returns the Attribution that WithAttribution put in ctx,
// and whether there was one.
func AttributionFrom(ctx context.Context) (Attribution, bool) {
	a, ok := ctx.Value(attributionKey{}).(Attribution)
	return a, ok
}

// SetUsageRecorder makes c report every model answer's usage to r. Call it
// before c is used; a nil r (the default) records nothing.
func (c *Client) SetUsageRecorder(r UsageRecorder) { c.usage = r }

// recordUsage reports one answer's usage to c's recorder, if it has one.
func (c *Client) recordUsage(ctx context.Context, step, model string, prompt, completion, total int) {
	if c.usage == nil {
		return
	}
	if model == "" {
		model = c.model
	}
	c.usage.RecordUsage(ctx, Usage{Model: model, Step: step, PromptTokens: prompt, CompletionTokens: completion, TotalTokens: total})
}
