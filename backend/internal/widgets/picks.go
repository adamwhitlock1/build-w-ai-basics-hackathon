package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// RememberedPicks returns the Articles and "Happening near you" items
// already remembered for member's page, read-only: it never searches or
// writes. Items count only if they were picked for profile as it is now and
// for member's audience, the same checks the cards and the briefing make;
// otherwise that list is empty.
func RememberedPicks(ctx context.Context, results ResultStore, member store.User, profile store.MemberProfile) (articles, events []llm.Item, err error) {
	aud := audienceFor(member)
	if profile.Interests != "" {
		if articles, _, err = rememberedWidgetItems(ctx, results, member.ID, kindArticles, normalizeInput(profile.Interests), aud); err != nil {
			return nil, nil, err
		}
	}
	if profile.Location != "" {
		if events, _, err = rememberedWidgetItems(ctx, results, member.ID, kindEvents, eventsInputs(profile), aud); err != nil {
			return nil, nil, err
		}
	}
	return articles, events, nil
}

// rememberedWidgetItems returns the items of userID's remembered curated
// widget of kind, if it was picked for inputs and for aud. Nothing
// remembered, or something picked for other inputs or another audience, is
// (nil, false, nil); an error is a failure to read it.
func rememberedWidgetItems(ctx context.Context, results ResultStore, userID int64, kind, inputs string, aud sources.Audience) ([]llm.Item, bool, error) {
	r, err := results.GetCuratedResult(ctx, userID, kind)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load remembered %s: %w", kind, err)
	}
	if r.Inputs != inputs {
		return nil, false, nil
	}
	var widget Widget
	if err := json.Unmarshal(r.Payload, &widget); err != nil {
		return nil, false, fmt.Errorf("read remembered %s: %w", kind, err)
	}
	if widget.KidSafe != (aud == sources.Kids) {
		return nil, false, nil
	}
	return widget.Items, true, nil
}
