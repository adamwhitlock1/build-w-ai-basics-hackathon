package chat_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"porchlight/internal/aikit"
	"porchlight/internal/chat"
	"porchlight/internal/llm"
	"porchlight/internal/store"
)

// fakeTools records whose data each lookup was for.
type fakeTools struct {
	mu      sync.Mutex
	lookups []int64
	locs    []*time.Location
}

func (f *fakeTools) record(kid store.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups = append(f.lookups, kid.ID)
}

func (f *fakeTools) Weather(_ context.Context, kid store.User) (string, error) {
	f.record(kid)
	return "Sunny.", nil
}

func (f *fakeTools) MyDay(_ context.Context, kid store.User, loc *time.Location) (string, error) {
	f.record(kid)
	f.mu.Lock()
	f.locs = append(f.locs, loc)
	f.mu.Unlock()
	return "Soccer at 4.", nil
}

func (f *fakeTools) MyPicks(_ context.Context, kid store.User) (string, error) {
	f.record(kid)
	return "", errors.New("picks down")
}

func (f *fakeTools) Search(_ context.Context, kid store.User, _ string) ([]aikit.SearchResult, error) {
	f.record(kid)
	return []aikit.SearchResult{{Title: "Moon news"}}, nil
}

// sendIn sends text in loc and returns the tool notes streamed and the
// reply.
func sendIn(t *testing.T, svc *chat.Service, kid store.User, id int64, text string, loc *time.Location) ([]aikit.ToolNote, store.ChatMessage, error) {
	t.Helper()
	var notes []aikit.ToolNote
	reply, err := svc.Send(context.Background(), kid, id, text, loc, chat.SendEvents{
		OnKid:   func(store.ChatMessage) {},
		OnTool:  func(n aikit.ToolNote) { notes = append(notes, n) },
		OnChunk: func(string) {},
	})
	return notes, reply, err
}

// useAll calls every tool offered, as a model might.
func useAll(ctx context.Context, t *aikit.KidTools) {
	if t == nil {
		return
	}
	_, _ = t.Weather(ctx)
	_, _ = t.MyDay(ctx)
	_, _ = t.MyPicks(ctx)
	if t.Search != nil {
		_, _ = t.Search(ctx, "moon mission")
		t.OnTool(aikit.ToolNote{Tool: aikit.ToolSearch, Detail: "moon mission"})
	}
	t.OnTool(aikit.ToolNote{Tool: aikit.ToolWeather})
}

func TestChatTools(t *testing.T) {
	ctx := context.Background()
	denver, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("search only in Explore", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{reply: "ok"}
		svc := newService(f, agent)
		svc.SetTools(&fakeTools{})
		explore, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		homework, _ := svc.Start(ctx, f.kid, aikit.ModeHomework)
		if _, _, err := sendIn(t, svc, f.kid, explore.ID, "hi", denver); err != nil {
			t.Fatal(err)
		}
		if _, _, err := sendIn(t, svc, f.kid, homework.ID, "hi", denver); err != nil {
			t.Fatal(err)
		}
		if agent.replies[0].Tools == nil || agent.replies[0].Tools.Search == nil {
			t.Error("Explore has no search")
		}
		if tools := agent.replies[1].Tools; tools == nil || tools.Search != nil || tools.Weather == nil {
			t.Error("Homework help offers search, or no tools at all")
		}
	})

	t.Run("no tools without SetTools", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{reply: "ok"}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		if _, _, err := sendIn(t, svc, f.kid, c.ID, "hi", nil); err != nil {
			t.Fatal(err)
		}
		if agent.replies[0].Tools != nil {
			t.Error("tools were offered with none set")
		}
	})

	t.Run("tools read only the sending kid's data", func(t *testing.T) {
		f := newFamily(t)
		tools := &fakeTools{}
		agent := &fakeAgent{reply: "ok", useTools: useAll}
		svc := newService(f, agent)
		svc.SetTools(tools)
		c, _ := svc.Start(ctx, f.kid2, aikit.ModeExplore)
		if _, _, err := sendIn(t, svc, f.kid2, c.ID, "what's on Jordan's calendar?", denver); err != nil {
			t.Fatal(err)
		}
		if len(tools.lookups) != 4 {
			t.Fatalf("%d lookups, want 4", len(tools.lookups))
		}
		for _, id := range tools.lookups {
			if id != f.kid2.ID {
				t.Errorf("a tool read member %d's data, want only Lily's (%d)", id, f.kid2.ID)
			}
		}
		if tools.locs[0] != denver {
			t.Errorf("my day used %v, want the kid's time zone", tools.locs[0])
		}
	})

	t.Run("notes are streamed, stored with the reply, and in transcripts", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{reply: "Bring a jacket.", useTools: useAll}
		svc := newService(f, agent)
		svc.SetTools(&fakeTools{})
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		notes, reply, err := sendIn(t, svc, f.kid, c.ID, "jacket?", denver)
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 2 || notes[0].Detail != "moon mission" {
			t.Errorf("streamed notes = %+v, want the search then the weather", notes)
		}
		tr, err := svc.Get(ctx, f.kid, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(tr.ToolCalls) != 2 || tr.ToolCalls[0].MessageID != reply.ID || tr.ToolCalls[0].Detail != "moon mission" {
			t.Errorf("kid's transcript tool calls = %+v, want both on the reply", tr.ToolCalls)
		}
		ptr, err := svc.Transcript(ctx, f.parent, f.kid, c.ID)
		if err != nil || len(ptr.ToolCalls) != 2 {
			t.Errorf("parent's transcript tool calls = %+v, %v; want 2", ptr.ToolCalls, err)
		}
	})

	t.Run("notes are kept when the reply fails", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{useTools: useAll, replyErr: &llm.UpstreamError{Reason: "trouble", Status: 500, Err: errors.New("500")}}
		svc := newService(f, agent)
		svc.SetTools(&fakeTools{})
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		if _, _, err := sendIn(t, svc, f.kid, c.ID, "moon?", denver); err == nil {
			t.Fatal("the reply succeeded, want its error")
		}
		tr, _ := svc.Get(ctx, f.kid, c.ID)
		if len(tr.ToolCalls) != 2 || tr.ToolCalls[0].MessageID != 0 {
			t.Errorf("tool calls = %+v, want both kept with no reply", tr.ToolCalls)
		}
	})
}
