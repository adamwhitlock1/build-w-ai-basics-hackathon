package assistant_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"porchlight/internal/aikit"
	"porchlight/internal/assistant"
	"porchlight/internal/chat"
	"porchlight/internal/sources"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

type noFetcher struct{}

func (noFetcher) Fetch(context.Context, string) ([]byte, error) {
	panic("no calendar should be fetched")
}

type webSource struct{ auds []sources.Audience }

func (w *webSource) Name() string  { return "web_search" }
func (w *webSource) Shape() string { return "test" }
func (w *webSource) Search(_ context.Context, _ string, aud sources.Audience) ([]sources.Result, error) {
	w.auds = append(w.auds, aud)
	return []sources.Result{{Title: "Kits", URL: "https://example.com/kits", Site: "example.com"}}, nil
}

// kidChatAgent answers kid chat with a fixed reply and a flag, to seed
// conversations through the real chat service.
type kidChatAgent struct{ verdict aikit.Verdict }

func (a *kidChatAgent) Check(context.Context, []aikit.Turn, string, bool) (aikit.Verdict, error) {
	return a.verdict, nil
}
func (a *kidChatAgent) Reply(_ context.Context, _ aikit.ReplyInput, onChunk func(string)) (string, error) {
	onChunk("ok")
	return "ok", nil
}

func TestToolSources(t *testing.T) {
	ctx := context.Background()
	f := newFamily(t)
	chats := chat.NewService(&kidChatAgent{verdict: aikit.Verdict{Flag: true, Category: aikit.CategoryStrangers, Severity: "medium", Reason: "Asked for an address.", Title: "Drawing a horse"}}, f.st, quiet())
	web := &webSource{}
	ts := &assistant.ToolSources{
		Members: f.st,
		Lookups: &chat.ToolSources{Profiles: f.st, Agenda: widgets.NewAgenda(noFetcher{}, f.st, quiet()), Results: f.st, Forecast: web, Web: web},
		Web:     web,
		Chats:   chats,
	}

	// Seed one flagged kid chat for Lily, and one for the Chen kid.
	for _, kid := range []store.User{f.lily, f.riley} {
		c, err := chats.Start(ctx, kid, aikit.ModeExplore)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := chats.Send(ctx, kid, c.ID, "someone asked where I live", time.UTC, chat.SendEvents{OnKid: func(store.ChatMessage) {}, OnChunk: func(string) {}}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("search is unfiltered and keeps links", func(t *testing.T) {
		results, err := ts.Search(ctx, "robot kits")
		if err != nil || len(results) != 1 || results[0].URL != "https://example.com/kits" || web.auds[len(web.auds)-1] != sources.Everyone {
			t.Errorf("results = %+v, %v", results, err)
		}
	})

	t.Run("family day: everyone, a name, and someone not in the family", func(t *testing.T) {
		out, err := ts.FamilyDay(ctx, f.maya, "everyone", time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Maya (you):", "Sam:", "Jordan:", "Lily:"} {
			if !strings.Contains(out, name) {
				t.Errorf("everyone's day doesn't include %q:\n%s", name, out)
			}
		}
		if strings.Contains(out, "Alex") || strings.Contains(out, "Riley") {
			t.Error("everyone's day includes another family")
		}
		out, _ = ts.FamilyDay(ctx, f.maya, "jordan", time.UTC)
		if !strings.HasPrefix(out, "Jordan:") || strings.Contains(out, "Lily") {
			t.Errorf("Jordan's day = %q", out)
		}
		out, _ = ts.FamilyDay(ctx, f.maya, "Alex", time.UTC)
		if !strings.Contains(out, "no one named") {
			t.Errorf("another family's member = %q, want not found", out)
		}
	})

	t.Run("kids' chats: overview, one kid, a transcript, and only this family", func(t *testing.T) {
		out, err := ts.KidsChats(ctx, f.maya, aikit.KidsChatsQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Lily") || !strings.Contains(out, "strangers") || !strings.Contains(out, "Asked for an address.") || strings.Contains(out, "Riley") {
			t.Errorf("overview = %s", out)
		}
		// A flagged first message gives the neutral title, so conversations
		// are listed and read by id.
		lily, _ := chats.List(ctx, f.lily)
		id := fmt.Sprint(lily[0].ID)
		out, _ = ts.KidsChats(ctx, f.maya, aikit.KidsChatsQuery{Kid: "lily"})
		if !strings.Contains(out, "Lily's flags") || !strings.Contains(out, "[id "+id+`] "A conversation"`) {
			t.Errorf("Lily's summary = %s", out)
		}
		for _, which := range []string{id, "#" + id, "A conversation"} {
			out, _ = ts.KidsChats(ctx, f.maya, aikit.KidsChatsQuery{Kid: "Lily", Conversation: which})
			if !strings.Contains(out, "Lily: someone asked where I live [flagged: strangers, medium]") {
				t.Errorf("transcript by %q = %s", which, out)
			}
		}
		// Another family's conversation id finds nothing under Lily.
		riley, _ := chats.List(ctx, f.riley)
		out, _ = ts.KidsChats(ctx, f.maya, aikit.KidsChatsQuery{Kid: "Lily", Conversation: fmt.Sprint(riley[0].ID)})
		if !strings.Contains(out, "no conversation like") {
			t.Errorf("another family's conversation = %s", out)
		}
		out, _ = ts.KidsChats(ctx, f.maya, aikit.KidsChatsQuery{Kid: "Riley"})
		if !strings.Contains(out, "no kid named") {
			t.Errorf("another family's kid = %s, want not found", out)
		}
	})
}
