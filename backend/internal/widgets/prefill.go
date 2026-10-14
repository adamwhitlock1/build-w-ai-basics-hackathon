package widgets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// The functions in this file are for seeding local test data (cmd/seed's
// demo family). They build results exactly as the handlers remember them,
// using the same matching rules, so a seeded page opens without searching.
// Keeping them here means a change to what's remembered changes them too.

// PrefillArticles remembers items as member's Articles, picked at at for the
// interests in profile.
func PrefillArticles(ctx context.Context, results ResultStore, member store.User, profile store.MemberProfile, items []llm.Item, at time.Time) error {
	w := articlesWidget(audienceFor(member))
	w.Items = items
	return prefill(ctx, results, member.ID, kindArticles, articlesInputs(profile), w, at)
}

// PrefillEvents remembers items as member's "Happening near you" events,
// picked at at for the interests and location in profile.
func PrefillEvents(ctx context.Context, results ResultStore, member store.User, profile store.MemberProfile, items []llm.Item, at time.Time) error {
	w := eventsWidget(audienceFor(member))
	w.Items = items
	return prefill(ctx, results, member.ID, kindEvents, eventsInputs(profile), w, at)
}

func prefill(ctx context.Context, results ResultStore, userID int64, kind, inputs string, w Widget, at time.Time) error {
	if w.Items == nil {
		w.Items = []llm.Item{}
	}
	payload, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("encode %s for user %d: %w", kind, userID, err)
	}
	return results.PutCuratedResult(ctx, store.CuratedResult{
		UserID: userID, Kind: kind, Inputs: inputs, Payload: payload, UpdatedAt: at.Truncate(time.Second),
	})
}

// NewCustomWidget returns a described widget for member, with items already
// in it, ready for store.CreateCustomWidget. It's kid-safe when member is a
// kid, as a widget a kid describes is.
func NewCustomWidget(member store.User, title, source, query, prompt string, archetype llm.Archetype, items []llm.Item) (store.CustomWidget, error) {
	if items == nil {
		items = []llm.Item{}
	}
	spec, err := json.Marshal(renderSpec{
		Title: title, Archetype: archetype, Items: items, KidSafe: audienceFor(member) == sources.Kids,
	})
	if err != nil {
		return store.CustomWidget{}, fmt.Errorf("encode render spec: %w", err)
	}
	return store.CustomWidget{
		UserID:     member.ID,
		Name:       title,
		Source:     source,
		Query:      query,
		Prompt:     prompt,
		RenderSpec: spec,
	}, nil
}
