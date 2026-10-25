package widgets_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

func TestRememberedPicks(t *testing.T) {
	ctx := context.Background()
	kid := store.User{ID: 7, FamilyID: 1}
	profile := store.MemberProfile{UserID: 7, Interests: "dinosaurs", Location: "Denver"}
	now := time.Now()

	t.Run("nothing remembered", func(t *testing.T) {
		results := newMemResults()
		articles, events, err := widgets.RememberedPicks(ctx, results, kid, profile)
		if err != nil || len(articles) != 0 || len(events) != 0 {
			t.Errorf("got %v, %v, %v; want empty lists", articles, events, err)
		}
		if results.puts != 0 {
			t.Error("RememberedPicks wrote a result")
		}
	})

	t.Run("remembered picks come back, read-only", func(t *testing.T) {
		results := newMemResults()
		if err := widgets.PrefillArticles(ctx, results, kid, profile, []llm.Item{{Title: "New dinosaur found"}}, now); err != nil {
			t.Fatal(err)
		}
		if err := widgets.PrefillEvents(ctx, results, kid, profile, []llm.Item{{Title: "Fossil day"}}, now); err != nil {
			t.Fatal(err)
		}
		puts := results.puts
		articles, events, err := widgets.RememberedPicks(ctx, results, kid, profile)
		if err != nil || len(articles) != 1 || articles[0].Title != "New dinosaur found" || len(events) != 1 || events[0].Title != "Fossil day" {
			t.Errorf("got %v, %v, %v; want the remembered items", articles, events, err)
		}
		if results.puts != puts {
			t.Error("RememberedPicks wrote a result")
		}

		// Picks for an older profile don't count.
		changed := profile
		changed.Interests = "horses"
		articles, events, _ = widgets.RememberedPicks(ctx, results, kid, changed)
		if len(articles) != 0 || len(events) != 0 {
			t.Errorf("after the interests changed: %v, %v; want none", articles, events)
		}
	})

	t.Run("a read error is returned", func(t *testing.T) {
		results := newMemResults()
		results.getErr = errors.New("disk on fire")
		if _, _, err := widgets.RememberedPicks(ctx, results, kid, profile); err == nil {
			t.Error("no error")
		}
	})
}
