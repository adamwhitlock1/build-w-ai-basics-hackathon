package chat_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"porchlight/internal/aikit"
	"porchlight/internal/chat"
	"porchlight/internal/llm"
	"porchlight/internal/store"
)

// fakeAgent answers with canned verdicts and replies, and records what it
// was asked.
type fakeAgent struct {
	mu       sync.Mutex
	verdict  aikit.Verdict
	checkErr error
	reply    string
	replyErr error
	chunks   []string // streamed before replyErr, if set; else reply in one chunk
	checks   []checkCall
	replies  []aikit.ReplyInput
	attrs    []llm.Attribution
}

type checkCall struct {
	recent    []aikit.Turn
	msg       string
	wantTitle bool
}

func (f *fakeAgent) Check(ctx context.Context, recent []aikit.Turn, msg string, wantTitle bool) (aikit.Verdict, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks = append(f.checks, checkCall{recent, msg, wantTitle})
	if a, ok := llm.AttributionFrom(ctx); ok {
		f.attrs = append(f.attrs, a)
	}
	return f.verdict, f.checkErr
}

func (f *fakeAgent) Reply(ctx context.Context, in aikit.ReplyInput, onChunk func(string)) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, in)
	if f.replyErr != nil {
		for _, c := range f.chunks {
			onChunk(c)
		}
		return "", f.replyErr
	}
	onChunk(f.reply)
	return f.reply, nil
}

// family is a test family: two parents, two kids, and a kid in another
// family.
type family struct {
	st                         *store.Store
	parent, parent2, kid, kid2 store.User
	otherParent, otherKid      store.User
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
		inv := store.Invite{FamilyID: familyID, Email: email, Token: token, IsAdmin: admin, ExpiresAt: time.Now().Add(time.Hour)}
		if _, _, err := st.InviteToFamily(ctx, inv); err != nil {
			t.Fatal(err)
		}
		u, err := st.AcceptInvite(ctx, token, []byte("hash"))
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	parent, err := st.CreateFamilyWithAdmin(ctx, "maya@rivera.test", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateFamilyWithAdmin(ctx, "alex@chen.test", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	return family{
		st:          st,
		parent:      parent,
		parent2:     join(parent.FamilyID, "sam@rivera.test", true),
		kid:         join(parent.FamilyID, "jordan@rivera.test", false),
		kid2:        join(parent.FamilyID, "lily@rivera.test", false),
		otherParent: other,
		otherKid:    join(other.FamilyID, "riley@chen.test", false),
	}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newService(f family, agent *fakeAgent) *chat.Service {
	if agent == nil {
		return chat.NewService(nil, f.st, quietLogger())
	}
	return chat.NewService(agent, f.st, quietLogger())
}

// send sends text and returns the kid message, the streamed text, the
// stored reply and the error.
func send(t *testing.T, svc *chat.Service, kid store.User, id int64, text string) (store.ChatMessage, string, store.ChatMessage, error) {
	t.Helper()
	var kidMsg store.ChatMessage
	var streamed strings.Builder
	reply, err := svc.Send(context.Background(), kid, id, text,
		func(m store.ChatMessage) { kidMsg = m },
		func(s string) { streamed.WriteString(s) })
	return kidMsg, streamed.String(), reply, err
}

func TestKidChat(t *testing.T) {
	ctx := context.Background()

	t.Run("chat off: nothing starts or is stored", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, nil)
		if svc.Available() {
			t.Error("Available() = true with no agent")
		}
		if _, err := svc.Start(ctx, f.kid, aikit.ModeExplore); !errors.Is(err, chat.ErrChatOff) {
			t.Errorf("Start err = %v, want ErrChatOff", err)
		}
		// A conversation from before chat was turned off can't continue.
		c, err := f.st.CreateConversation(ctx, f.kid.FamilyID, f.kid.ID, aikit.ModeExplore)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := send(t, svc, f.kid, c.ID, "hello"); !errors.Is(err, chat.ErrChatOff) {
			t.Errorf("Send err = %v, want ErrChatOff", err)
		}
		if msgs, _ := f.st.ListChatMessages(ctx, c.ID); len(msgs) != 0 {
			t.Errorf("stored %d messages, want none", len(msgs))
		}
		// Reading still works.
		if list, err := svc.List(ctx, f.kid); err != nil || len(list) != 1 {
			t.Errorf("List = %d conversations, %v; want 1", len(list), err)
		}
	})

	t.Run("a parent can't chat as themselves", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, &fakeAgent{reply: "hi"})
		if _, err := svc.Start(ctx, f.parent, aikit.ModeExplore); !errors.Is(err, chat.ErrNotAKid) {
			t.Errorf("Start err = %v, want ErrNotAKid", err)
		}
		if _, err := svc.List(ctx, f.parent); !errors.Is(err, chat.ErrNotAKid) {
			t.Errorf("List err = %v, want ErrNotAKid", err)
		}
	})

	t.Run("an unknown mode is refused", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, &fakeAgent{reply: "hi"})
		if _, err := svc.Start(ctx, f.kid, "gossip"); !errors.Is(err, chat.ErrInvalidMode) {
			t.Errorf("Start err = %v, want ErrInvalidMode", err)
		}
	})

	t.Run("another kid's conversation isn't found", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, &fakeAgent{reply: "hi"})
		c, err := svc.Start(ctx, f.kid, aikit.ModeExplore)
		if err != nil {
			t.Fatal(err)
		}
		for _, other := range []store.User{f.kid2, f.otherKid} {
			if _, err := svc.Get(ctx, other, c.ID); !errors.Is(err, chat.ErrNoSuchChat) {
				t.Errorf("Get as %s err = %v, want ErrNoSuchChat", other.Email, err)
			}
			if _, _, _, err := send(t, svc, other, c.ID, "hi"); !errors.Is(err, chat.ErrNoSuchChat) {
				t.Errorf("Send as %s err = %v, want ErrNoSuchChat", other.Email, err)
			}
		}
	})

	t.Run("empty and over-long messages are refused", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, &fakeAgent{reply: "hi"})
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		if _, _, _, err := send(t, svc, f.kid, c.ID, "   "); !errors.Is(err, chat.ErrEmptyMessage) {
			t.Errorf("empty: err = %v, want ErrEmptyMessage", err)
		}
		if _, _, _, err := send(t, svc, f.kid, c.ID, strings.Repeat("a", chat.MaxMessageChars+1)); !errors.Is(err, chat.ErrMessageTooLong) {
			t.Errorf("long: err = %v, want ErrMessageTooLong", err)
		}
		if msgs, _ := f.st.ListChatMessages(ctx, c.ID); len(msgs) != 0 {
			t.Errorf("stored %d messages, want none", len(msgs))
		}
	})

	t.Run("a clean message", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{verdict: aikit.Verdict{Title: "How volcanoes form"}, reply: "Great question! What do you already know?"}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeHomework)
		kidMsg, streamed, reply, err := send(t, svc, f.kid, c.ID, "how do volcanoes form?")
		if err != nil {
			t.Fatal(err)
		}
		if kidMsg.Text != "how do volcanoes form?" || kidMsg.Role != store.RoleKid {
			t.Errorf("kid message = %+v", kidMsg)
		}
		if streamed != agent.reply || reply.Text != agent.reply || reply.Role != store.RoleAgent {
			t.Errorf("streamed %q, stored %+v; want the reply", streamed, reply)
		}
		tr, err := svc.Get(ctx, f.kid, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(tr.Messages) != 2 || tr.Conversation.Title != "How volcanoes form" || tr.Flags != nil {
			t.Errorf("transcript = %+v, want 2 messages, the title, and no flags for a kid", tr)
		}
		if n, _ := f.st.CountUnreviewedFlags(ctx, f.kid.FamilyID); n != 0 {
			t.Errorf("%d flags, want none", n)
		}
		if !agent.checks[0].wantTitle || len(agent.checks[0].recent) != 0 {
			t.Errorf("first check = %+v, want a title request and no earlier turns", agent.checks[0])
		}
		in := agent.replies[0]
		if in.Mode != aikit.ModeHomework || in.Crisis || len(in.History) != 1 || !in.History[0].FromKid {
			t.Errorf("reply input = %+v, want homework, no crisis, the kid's message", in)
		}
		if a := agent.attrs[0]; a.Feature != chat.Feature || a.MemberID != f.kid.ID || a.ViewerID != f.kid.ID {
			t.Errorf("attribution = %+v, want kid_chat for the kid", a)
		}

		// The second message sends the context and asks for no title.
		agent.verdict = aikit.Verdict{Title: "Something else"}
		if _, _, _, err := send(t, svc, f.kid, c.ID, "is it about magma?"); err != nil {
			t.Fatal(err)
		}
		if agent.checks[1].wantTitle || len(agent.checks[1].recent) != 2 {
			t.Errorf("second check = %+v, want no title request and the 2 earlier turns", agent.checks[1])
		}
		if tr, _ := svc.Get(ctx, f.kid, c.ID); tr.Conversation.Title != "How volcanoes form" {
			t.Errorf("title = %q, want it unchanged", tr.Conversation.Title)
		}
	})

	t.Run("a flagged message is still answered", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{verdict: aikit.Verdict{Flag: true, Category: aikit.CategoryBullying, Severity: "medium", Reason: "Being teased at school."}, reply: "That sounds hard."}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		_, _, reply, err := send(t, svc, f.kid, c.ID, "kids at school keep making fun of me")
		if err != nil || reply.Text != "That sounds hard." {
			t.Fatalf("reply = %+v, %v", reply, err)
		}
		queue, _ := f.st.ReviewQueue(ctx, f.kid.FamilyID, 10)
		if len(queue) != 1 || queue[0].Category != aikit.CategoryBullying || queue[0].Crisis || queue[0].MessageText != "kids at school keep making fun of me" {
			t.Errorf("queue = %+v, want one bullying flag on the message", queue)
		}
	})

	t.Run("a crisis ends with the resources", func(t *testing.T) {
		f := newFamily(t)
		// The model mentions 988 itself; the resources are still added.
		agent := &fakeAgent{verdict: aikit.Verdict{Flag: true, Category: aikit.CategorySelfHarm, Severity: "high", Crisis: true, Reason: "Said they don't want to be alive.", Title: "Feelings of hopelessness"},
			reply: "I'm really glad you told me. You could call 988."}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		_, streamed, reply, err := send(t, svc, f.kid, c.ID, "i don't want to be alive anymore")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(reply.Text, chat.CrisisResources) || !strings.HasSuffix(streamed, chat.CrisisResources) {
			t.Errorf("reply = %q, streamed %q; want both to end with the resources", reply.Text, streamed)
		}
		if !agent.replies[0].Crisis {
			t.Error("the reply wasn't told about the crisis")
		}
		crises, _ := f.st.UnreviewedCrises(ctx, f.kid.FamilyID)
		if len(crises) != 1 || crises[0].Severity != "high" {
			t.Errorf("crises = %+v, want one high-severity crisis flag", crises)
		}
		if tr, _ := svc.Get(ctx, f.kid, c.ID); tr.Conversation.Title != chat.NeutralTitle {
			t.Errorf("title = %q, want the neutral title", tr.Conversation.Title)
		}
	})

	t.Run("a failed check flags and still answers", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{checkErr: &llm.UpstreamError{Reason: "took too long", Err: context.DeadlineExceeded}, reply: "Sure!"}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		_, _, reply, err := send(t, svc, f.kid, c.ID, "tell me about sharks")
		if err != nil || reply.Text != "Sure!" {
			t.Fatalf("reply = %+v, %v", reply, err)
		}
		if agent.replies[0].Crisis {
			t.Error("a failed check led to crisis handling")
		}
		queue, _ := f.st.ReviewQueue(ctx, f.kid.FamilyID, 10)
		if len(queue) != 1 || queue[0].Category != aikit.CategoryCouldntCheck || queue[0].Severity != "low" {
			t.Errorf("queue = %+v, want one low couldnt_check flag", queue)
		}
		if tr, _ := svc.Get(ctx, f.kid, c.ID); tr.Conversation.Title != chat.NeutralTitle {
			t.Errorf("title = %q, want the neutral title for an unchecked message", tr.Conversation.Title)
		}
	})

	t.Run("a failed reply keeps the kid's message only", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{replyErr: &llm.UpstreamError{Reason: "rate limiting", Status: 429, Err: errors.New("429")}, chunks: []string{"Let's "}}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		kidMsg, streamed, _, err := send(t, svc, f.kid, c.ID, "hi")
		var ue *llm.UpstreamError
		if !errors.As(err, &ue) || ue.Status != 429 {
			t.Errorf("err = %v, want the UpstreamError", err)
		}
		if kidMsg.ID == 0 || streamed != "Let's " {
			t.Errorf("kid message %+v, streamed %q", kidMsg, streamed)
		}
		msgs, _ := f.st.ListChatMessages(ctx, c.ID)
		if len(msgs) != 1 || msgs[0].Role != store.RoleKid {
			t.Errorf("stored %+v, want only the kid's message", msgs)
		}
	})

	t.Run("a long first message gets a cut title", func(t *testing.T) {
		f := newFamily(t)
		svc := newService(f, &fakeAgent{reply: "ok"})
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		long := strings.Repeat("volcano ", 20)
		if _, _, _, err := send(t, svc, f.kid, c.ID, long); err != nil {
			t.Fatal(err)
		}
		tr, _ := svc.Get(ctx, f.kid, c.ID)
		if n := len([]rune(tr.Conversation.Title)); n > 60 || !strings.HasSuffix(tr.Conversation.Title, "volcano…") {
			t.Errorf("title = %q (%d characters), want at most 60, cut at a word", tr.Conversation.Title, n)
		}
	})

	t.Run("the persona follows the birth year", func(t *testing.T) {
		f := newFamily(t)
		agent := &fakeAgent{reply: "ok"}
		svc := newService(f, agent)
		c, _ := svc.Start(ctx, f.kid, aikit.ModeExplore)
		if _, _, _, err := send(t, svc, f.kid, c.ID, "hi"); err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetBirthYear(ctx, f.kid.ID, time.Now().Year()-15); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := send(t, svc, f.kid, c.ID, "hi again"); err != nil {
			t.Fatal(err)
		}
		if agent.replies[0].Persona != aikit.PersonaYounger || agent.replies[1].Persona != aikit.PersonaTeen {
			t.Errorf("personas = %s, %s; want younger (no birth year) then teen", agent.replies[0].Persona, agent.replies[1].Persona)
		}
	})
}

func TestPersonaFor(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		year int
		ok   bool
		want string
	}{
		{2014, true, aikit.PersonaYounger}, // 12
		{2013, true, aikit.PersonaTeen},    // 13
		{2011, true, aikit.PersonaTeen},    // 15
		{2017, true, aikit.PersonaYounger}, // 9
		{0, false, aikit.PersonaYounger},   // not set
	} {
		if got := chat.PersonaFor(tt.year, tt.ok, now); got != tt.want {
			t.Errorf("PersonaFor(%d, %v) = %s, want %s", tt.year, tt.ok, got, tt.want)
		}
	}
}
