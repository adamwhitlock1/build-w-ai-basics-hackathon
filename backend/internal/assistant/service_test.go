package assistant_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kinlight/internal/aikit"
	"kinlight/internal/assistant"
	"kinlight/internal/llm"
	"kinlight/internal/store"
)

// fakeAgent records what it was asked, and can call tools first.
type fakeAgent struct {
	name     string
	reply    string
	replyErr error
	inputs   []aikit.ParentReplyInput
	attrs    []llm.Attribution
	useTools func(ctx context.Context, t *aikit.ParentTools)
}

func (f *fakeAgent) Reply(ctx context.Context, in aikit.ParentReplyInput, onChunk func(string)) (string, error) {
	f.inputs = append(f.inputs, in)
	if a, ok := llm.AttributionFrom(ctx); ok {
		f.attrs = append(f.attrs, a)
	}
	if f.useTools != nil {
		f.useTools(ctx, in.Tools)
	}
	if f.replyErr != nil {
		return "", f.replyErr
	}
	onChunk(f.reply)
	return f.reply, nil
}

// family is the Riveras (two parents, two kids) and the Chens (one parent,
// one kid), in a real store.
type family struct {
	st                      *store.Store
	maya, sam, jordan, lily store.User
	alex, riley             store.User
}

func newFamily(t *testing.T) family {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	join := func(familyID int64, email string, admin bool) store.User {
		token := strings.ReplaceAll(email, "@", "-")
		if _, _, err := st.InviteToFamily(ctx, store.Invite{FamilyID: familyID, Email: email, Token: token, IsAdmin: admin, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		u, err := st.AcceptInvite(ctx, token, []byte("hash"))
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	maya, err := st.CreateFamilyWithAdmin(ctx, "maya@rivera.test", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	alex, err := st.CreateFamilyWithAdmin(ctx, "alex@chen.test", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	return family{
		st: st, maya: maya,
		sam:    join(maya.FamilyID, "sam@rivera.test", true),
		jordan: join(maya.FamilyID, "jordan@rivera.test", false),
		lily:   join(maya.FamilyID, "lily@rivera.test", false),
		alex:   alex,
		riley:  join(alex.FamilyID, "riley@chen.test", false),
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// recordingTools is assistant.Tools that records whose data was read.
type recordingTools struct{ calls []string }

func (r *recordingTools) Search(context.Context, string) ([]aikit.SearchResult, error) {
	r.calls = append(r.calls, "search")
	return nil, nil
}
func (r *recordingTools) FamilyDay(_ context.Context, p store.User, who string, _ *time.Location) (string, error) {
	r.calls = append(r.calls, "family_day:"+p.Email+":"+who)
	return "", nil
}
func (r *recordingTools) Weather(_ context.Context, p store.User) (string, error) {
	r.calls = append(r.calls, "weather:"+p.Email)
	return "", nil
}
func (r *recordingTools) MyPicks(_ context.Context, p store.User) (string, error) {
	r.calls = append(r.calls, "picks:"+p.Email)
	return "", nil
}
func (r *recordingTools) KidsChats(_ context.Context, p store.User, q aikit.KidsChatsQuery) (string, error) {
	r.calls = append(r.calls, "kids_chats:"+p.Email+":"+q.Kid)
	return "", nil
}

func send(svc *assistant.Service, parent store.User, id int64, text string) (store.AssistantMessage, []aikit.ToolNote, error) {
	var notes []aikit.ToolNote
	reply, err := svc.Send(context.Background(), parent, id, text, time.UTC, assistant.SendEvents{
		OnTool: func(n aikit.ToolNote) { notes = append(notes, n) },
	})
	return reply, notes, err
}

func TestAssistant(t *testing.T) {
	ctx := context.Background()

	t.Run("kids are refused", func(t *testing.T) {
		f := newFamily(t)
		svc := assistant.NewService(&fakeAgent{reply: "hi"}, nil, f.st, quiet())
		if _, err := svc.Start(ctx, f.jordan, aikit.KindAnything); !errors.Is(err, assistant.ErrNotParent) {
			t.Errorf("Start err = %v, want ErrNotParent", err)
		}
		if _, err := svc.List(ctx, f.jordan); !errors.Is(err, assistant.ErrNotParent) {
			t.Errorf("List err = %v, want ErrNotParent", err)
		}
	})

	t.Run("private to each parent", func(t *testing.T) {
		f := newFamily(t)
		svc := assistant.NewService(&fakeAgent{reply: "hi"}, nil, f.st, quiet())
		c, err := svc.Start(ctx, f.maya, aikit.KindAnything)
		if err != nil {
			t.Fatal(err)
		}
		for _, other := range []store.User{f.sam, f.alex} {
			if _, err := svc.Get(ctx, other, c.ID); !errors.Is(err, assistant.ErrNoSuchConversation) {
				t.Errorf("%s: Get err = %v, want ErrNoSuchConversation", other.Email, err)
			}
			if _, _, err := send(svc, other, c.ID, "hi"); !errors.Is(err, assistant.ErrNoSuchConversation) {
				t.Errorf("%s: Send err = %v, want ErrNoSuchConversation", other.Email, err)
			}
			if err := svc.Delete(ctx, other, c.ID); !errors.Is(err, assistant.ErrNoSuchConversation) {
				t.Errorf("%s: Delete err = %v, want ErrNoSuchConversation", other.Email, err)
			}
		}
		if err := svc.Delete(ctx, f.maya, c.ID); err != nil {
			t.Errorf("Maya's delete: %v", err)
		}
	})

	t.Run("About the kids needs kid chat", func(t *testing.T) {
		f := newFamily(t)
		svc := assistant.NewService(&fakeAgent{reply: "hi"}, nil, f.st, quiet())
		if svc.KidsAvailable() {
			t.Error("KidsAvailable with no kids' agent")
		}
		if _, err := svc.Start(ctx, f.maya, aikit.KindKids); !errors.Is(err, assistant.ErrKidsOff) {
			t.Errorf("Start err = %v, want ErrKidsOff", err)
		}
		if _, err := svc.Start(ctx, f.maya, "gossip"); !errors.Is(err, assistant.ErrInvalidKind) {
			t.Errorf("Start err = %v, want ErrInvalidKind", err)
		}
	})

	t.Run("the kind picks the agent and the tools", func(t *testing.T) {
		f := newFamily(t)
		anything := &fakeAgent{name: "main", reply: "Here you go.", useTools: func(ctx context.Context, t *aikit.ParentTools) {
			_, _ = t.FamilyDay(ctx, "everyone")
			t.OnTool(aikit.ToolNote{Tool: aikit.ToolFamilyDay, Detail: "everyone"})
		}}
		kids := &fakeAgent{name: "kids", reply: "Lily has one flag.", useTools: func(ctx context.Context, t *aikit.ParentTools) {
			_, _ = t.KidsChats(ctx, aikit.KidsChatsQuery{Kid: "Lily"})
		}}
		tools := &recordingTools{}
		svc := assistant.NewService(anything, kids, f.st, quiet())
		svc.SetTools(tools)

		c1, _ := svc.Start(ctx, f.maya, aikit.KindAnything)
		reply, notes, err := send(svc, f.maya, c1.ID, "what does everyone have tomorrow?")
		if err != nil || reply.Text != "Here you go." {
			t.Fatalf("reply = %+v, %v", reply, err)
		}
		if anything.inputs[0].Tools.KidsChats != nil {
			t.Error("an Ask anything conversation can read the kids' chats")
		}
		if len(kids.inputs) != 0 {
			t.Error("the kids' agent answered an Ask anything conversation")
		}
		if a := anything.attrs[0]; a.Feature != assistant.Feature || a.MemberID != f.maya.ID {
			t.Errorf("attribution = %+v", a)
		}
		tr, _ := svc.Get(ctx, f.maya, c1.ID)
		if tr.Conversation.Title != "what does everyone have tomorrow?" || len(tr.Messages) != 2 || len(tr.ToolCalls) != 1 || tr.ToolCalls[0].MessageID != reply.ID || len(notes) != 1 {
			t.Errorf("transcript = %+v, notes = %+v", tr, notes)
		}

		c2, _ := svc.Start(ctx, f.maya, aikit.KindKids)
		if _, _, err := send(svc, f.maya, c2.ID, "anything about Lily?"); err != nil {
			t.Fatal(err)
		}
		if len(kids.inputs) != 1 || kids.inputs[0].Tools.KidsChats == nil || kids.inputs[0].Kind != aikit.KindKids {
			t.Error("the About the kids conversation didn't use the kids' agent with kids_chats")
		}
		want := []string{"family_day:maya@rivera.test:everyone", "kids_chats:maya@rivera.test:Lily"}
		if strings.Join(tools.calls, ",") != strings.Join(want, ",") {
			t.Errorf("tool calls = %v, want %v (all for Maya)", tools.calls, want)
		}
	})

	t.Run("a failed reply keeps the parent's message and the notes", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{replyErr: &llm.UpstreamError{Reason: "trouble", Status: 500, Err: errors.New("500")}, useTools: func(ctx context.Context, t *aikit.ParentTools) {
			t.OnTool(aikit.ToolNote{Tool: aikit.ToolSearch, Detail: "robot kits"})
		}}
		svc := assistant.NewService(agent, nil, f.st, quiet())
		svc.SetTools(&recordingTools{})
		c, _ := svc.Start(ctx, f.maya, aikit.KindAnything)
		if _, _, err := send(svc, f.maya, c.ID, "find a robot kit"); err == nil {
			t.Fatal("no error")
		}
		tr, _ := svc.Get(ctx, f.maya, c.ID)
		if len(tr.Messages) != 1 || len(tr.ToolCalls) != 1 || tr.ToolCalls[0].MessageID != 0 {
			t.Errorf("transcript = %+v", tr)
		}
	})

	t.Run("empty and over-long messages", func(t *testing.T) {
		f := newFamily(t)
		svc := assistant.NewService(&fakeAgent{reply: "hi"}, nil, f.st, quiet())
		c, _ := svc.Start(ctx, f.maya, aikit.KindAnything)
		if _, _, err := send(svc, f.maya, c.ID, " "); !errors.Is(err, assistant.ErrEmptyMessage) {
			t.Errorf("err = %v", err)
		}
		if _, _, err := send(svc, f.maya, c.ID, strings.Repeat("a", assistant.MaxMessageChars+1)); !errors.Is(err, assistant.ErrMessageTooLong) {
			t.Errorf("err = %v", err)
		}
	})
}
