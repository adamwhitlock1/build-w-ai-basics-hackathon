package widgets

import (
	"context"

	"porchlight/internal/llm"
	"porchlight/internal/store"
)

// Features name what an AI call was for in the usage records (see
// internal/usage). A feature can make several calls, recorded as steps.
const (
	FeatureDescribeWidget = "describe_widget"
	FeatureRefreshWidget  = "refresh_widget"
	FeatureArticles       = "articles"
	FeatureEvents         = "events"
	FeatureBriefing       = "briefing"
)

// attribute returns a copy of ctx that marks the AI calls made with it as
// being for member's feature, caused by viewer. Every context derived from
// the result (deadlines, timeouts) keeps the mark, so the LLM client can
// attribute each call's token usage without being told.
func attribute(ctx context.Context, feature string, viewer, member store.User) context.Context {
	return llm.WithAttribution(ctx, llm.Attribution{
		FamilyID: member.FamilyID,
		MemberID: member.ID,
		ViewerID: viewer.ID,
		Feature:  feature,
	})
}
