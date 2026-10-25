package main

// End-to-end tests of kid chat's routes: kids chatting (with the reply
// streamed as server-sent events), parents reviewing, and who may do what.
// The agent is a fake; internal/chat and internal/aikit test the rest.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/sqlite3store"

	"porchlight/internal/aikit"
	"porchlight/internal/auth"
	"porchlight/internal/chat"
	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// scriptedAgent is a chat.Agent with a settable verdict and reply.
type scriptedAgent struct {
	mu       sync.Mutex
	verdict  aikit.Verdict
	reply    []string // streamed chunk by chunk
	replyErr error
	// checkDay makes the reply look at the kid's day first, as a model
	// calling the my_day tool would.
	checkDay bool
}

func (a *scriptedAgent) set(verdict aikit.Verdict, replyErr error, chunks ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.verdict, a.replyErr, a.reply = verdict, replyErr, chunks
}

func (a *scriptedAgent) Check(context.Context, []aikit.Turn, string, bool) (aikit.Verdict, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.verdict, nil
}

func (a *scriptedAgent) Reply(ctx context.Context, in aikit.ReplyInput, onChunk func(string)) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.checkDay && in.Tools != nil {
		_, _ = in.Tools.MyDay(ctx)
		in.Tools.OnTool(aikit.ToolNote{Tool: aikit.ToolMyDay})
	}
	for _, c := range a.reply {
		onChunk(c)
	}
	if a.replyErr != nil {
		return "", a.replyErr
	}
	return strings.Join(a.reply, ""), nil
}

// chatFamily is alice (a parent) and her kid, each logged in, on a server
// whose kid chat uses agent (nil: chat off), plus a parent and kid of
// another family on the same server.
type chatFamily struct {
	srv                         *httptest.Server
	st                          *store.Store
	alice, kid, other, otherKid *client
	aliceID, kidID, otherKidID  int64
}

// dayTools is chat.Tools whose my-day lookup records the time zone it was
// asked for.
type dayTools struct {
	mu   sync.Mutex
	locs []string
}

func (d *dayTools) Weather(context.Context, store.User) (string, error) { return "Sunny.", nil }
func (d *dayTools) MyPicks(context.Context, store.User) (string, error) { return "", nil }
func (d *dayTools) Search(context.Context, store.User, string) ([]aikit.SearchResult, error) {
	return nil, nil
}
func (d *dayTools) MyDay(_ context.Context, _ store.User, loc *time.Location) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.locs = append(d.locs, loc.String())
	return "Soccer at 4.", nil
}

func newChatFamily(t *testing.T, agent chat.Agent) chatFamily {
	return newChatFamilyWithTools(t, agent, nil)
}

func newChatFamilyWithTools(t *testing.T, agent chat.Agent, tools chat.Tools) chatFamily {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewSessionManager(sqlite3store.NewWithCleanupInterval(st.DB(), 0), logger)
	d := deps{
		Logger: logger, Store: st, Sessions: sessions,
		Planner: fakePlanner{}, Curator: fakeCurator{}, Briefer: &countingBriefer{},
		Sources:   sources.NewRegistry(fakeNews{}, fakeSource{}, fakeWeather{}),
		Calendars: fakeCalendars{},
	}
	if agent != nil {
		d.Chat = chat.NewService(agent, st, logger)
		if tools != nil {
			d.Chat.SetTools(tools)
		}
	}
	srv := httptest.NewServer(routes(d))
	t.Cleanup(func() { st.Close() })
	t.Cleanup(srv.Close)

	f := chatFamily{srv: srv, st: st, alice: newClient(t, srv), kid: newClient(t, srv), other: newClient(t, srv), otherKid: newClient(t, srv)}
	aliceMe := f.alice.signup("alice@example.com")
	f.alice.setUpFamily()
	f.aliceID = aliceMe.User.ID
	f.kidID = f.kid.accept(tokenOf(f.alice.invite(aliceMe.Family.ID, "kid@example.com", "kid", http.StatusCreated))).User.ID
	otherMe := f.other.signup("other@example.com")
	f.other.setUpFamily()
	f.otherKidID = f.otherKid.accept(tokenOf(f.other.invite(otherMe.Family.ID, "otherkid@example.com", "kid", http.StatusCreated))).User.ID
	return f
}

type chatList struct {
	Available     bool             `json:"available"`
	Conversations []map[string]any `json:"conversations"`
}

type transcript struct {
	Conversation map[string]any   `json:"conversation"`
	Messages     []map[string]any `json:"messages"`
	Flags        []map[string]any `json:"flags"`
}

// sseEvent is one server-sent event.
type sseEvent struct {
	name string
	data map[string]any
}

// sendMessage posts text to the kid's chat id and returns the status and,
// for a stream, its events.
func sendMessage(t *testing.T, c *client, chatID int64, text string) (int, []sseEvent, string) {
	t.Helper()
	return sendMessageIn(t, c, chatID, text, "")
}

// sendMessageIn is sendMessage with the kid's time zone.
func sendMessageIn(t *testing.T, c *client, chatID int64, text, timeZone string) (int, []sseEvent, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"text": text, "timeZone": timeZone})
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/me/chats/%d/messages", c.base, chatID), strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, nil, string(raw)
	}
	var events []sseEvent
	var ev sseEvent
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			ev = sseEvent{name: strings.TrimPrefix(line, "event: ")}
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.data); err != nil {
				t.Fatalf("event %s data: %v", ev.name, err)
			}
		case line == "":
			events = append(events, ev)
		}
	}
	return resp.StatusCode, events, ""
}

func eventNames(events []sseEvent) string {
	var names []string
	for _, e := range events {
		names = append(names, e.name)
	}
	return strings.Join(names, ",")
}

func TestKidChatRoutes(t *testing.T) {
	agent := &scriptedAgent{}
	f := newChatFamily(t, agent)

	var list chatList
	f.kid.do("GET", "/api/me/chats", nil, http.StatusOK, &list)
	if !list.Available || len(list.Conversations) != 0 {
		t.Fatalf("list = %+v, want available and empty", list)
	}

	var conv map[string]any
	f.kid.do("POST", "/api/me/chats", map[string]string{"mode": "homework"}, http.StatusCreated, &conv)
	chatID := int64(conv["id"].(float64))
	if conv["mode"] != "homework" {
		t.Errorf("conversation = %v, want homework", conv)
	}
	f.kid.wantError("POST", "/api/me/chats", map[string]string{"mode": "gossip"}, http.StatusBadRequest, `Send {"mode": "homework"} or {"mode": "explore"}.`)

	// A message streams: the kid's message, the reply in pieces, done.
	agent.set(aikit.Verdict{Title: "Solving for x"}, nil, "What have ", "you tried?")
	status, events, _ := sendMessage(t, f.kid, chatID, "solve 3x + 5 = 20")
	if status != http.StatusOK || eventNames(events) != "message,delta,delta,done" {
		t.Fatalf("status %d, events %q; want 200 and message,delta,delta,done", status, eventNames(events))
	}
	if events[0].data["text"] != "solve 3x + 5 = 20" || events[0].data["role"] != "kid" {
		t.Errorf("message event = %v", events[0].data)
	}
	if events[1].data["text"] != "What have " || events[3].data["text"] != "What have you tried?" || events[3].data["role"] != "agent" {
		t.Errorf("delta and done = %v, %v", events[1].data, events[3].data)
	}

	var tr transcript
	f.kid.do("GET", fmt.Sprintf("/api/me/chats/%d", chatID), nil, http.StatusOK, &tr)
	if len(tr.Messages) != 2 || tr.Conversation["title"] != "Solving for x" {
		t.Errorf("transcript = %+v, want 2 messages titled Solving for x", tr)
	}
	// Kids never see flags or flag counts.
	if _, has := tr.Conversation["unreviewedFlags"]; has || tr.Flags != nil {
		t.Errorf("kid's transcript shows flags: %+v", tr)
	}

	// Refused before anything is stored: ordinary JSON errors.
	if status, _, body := sendMessage(t, f.kid, chatID, "   "); status != http.StatusBadRequest || !strings.Contains(body, "Type a message first.") {
		t.Errorf("empty message: status %d, body %s", status, body)
	}
	if status, _, _ := sendMessage(t, f.kid, chatID, strings.Repeat("a", chat.MaxMessageChars+1)); status != http.StatusBadRequest {
		t.Errorf("long message: status %d, want 400", status)
	}

	// A reply that fails partway: what arrived, then a kind error.
	agent.set(aikit.Verdict{}, &llm.UpstreamError{Reason: "rate limiting", Status: 429, Err: fmt.Errorf("429")}, "Let's ")
	status, events, _ = sendMessage(t, f.kid, chatID, "again?")
	if status != http.StatusOK || eventNames(events) != "message,delta,error" {
		t.Fatalf("status %d, events %q; want message,delta,error", status, eventNames(events))
	}
	if msg, _ := events[2].data["error"].(string); !strings.Contains(msg, "something went wrong") {
		t.Errorf("error event = %v, want a kind message", events[2].data)
	}
	f.kid.do("GET", fmt.Sprintf("/api/me/chats/%d", chatID), nil, http.StatusOK, &tr)
	if len(tr.Messages) != 3 || tr.Messages[2]["role"] != "kid" {
		t.Errorf("after a failed reply: %d messages, want the kid's message stored and no partial reply", len(tr.Messages))
	}

	// Another family's kid can't see or use it.
	f.otherKid.wantError("GET", fmt.Sprintf("/api/me/chats/%d", chatID), nil, http.StatusNotFound, "That conversation doesn't exist.")
	if status, _, _ := sendMessage(t, f.otherKid, chatID, "hi"); status != http.StatusNotFound {
		t.Errorf("another kid's send: status %d, want 404", status)
	}

	// A parent can't chat as themselves.
	f.alice.wantError("POST", "/api/me/chats", map[string]string{"mode": "explore"}, http.StatusForbidden, "The parents' assistant isn't available yet.")
	f.alice.wantError("GET", "/api/me/chats", nil, http.StatusForbidden, "The parents' assistant isn't available yet.")

	// A forged cross-site message is refused before it reaches the handler.
	headers := map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"}
	if status := sendRaw(t, f.kid, "POST", fmt.Sprintf("/api/me/chats/%d/messages", chatID), `{"text":"hi"}`, headers); status != http.StatusForbidden {
		t.Errorf("cross-site send: status %d, want 403", status)
	}
}

func TestParentReviewRoutes(t *testing.T) {
	agent := &scriptedAgent{}
	f := newChatFamily(t, agent)
	var conv map[string]any
	f.kid.do("POST", "/api/me/chats", map[string]string{"mode": "explore"}, http.StatusCreated, &conv)
	chatID := int64(conv["id"].(float64))

	agent.set(aikit.Verdict{Flag: true, Category: aikit.CategorySelfHarm, Severity: "high", Crisis: true, Reason: "Said they want to disappear."}, nil, "I'm really glad you told me.")
	_, events, _ := sendMessage(t, f.kid, chatID, "i want to disappear")
	if done := events[len(events)-1]; done.name != "done" || !strings.HasSuffix(done.data["text"].(string), chat.CrisisResources) {
		t.Fatalf("last event = %+v, want done ending with the crisis resources", done)
	}

	// The kid can't use the parents' routes.
	f.kid.wantError("GET", "/api/family/reviews", nil, http.StatusForbidden, "Only parents can review kids' chats.")
	f.kid.wantError("GET", "/api/family/alerts", nil, http.StatusForbidden, "Only parents can review kids' chats.")
	f.kid.do("GET", memberPath(f.kidID, "/chats"), nil, http.StatusForbidden, nil)

	// Alerts and the queue.
	var alerts struct {
		Unreviewed int              `json:"unreviewed"`
		Crises     []map[string]any `json:"crises"`
	}
	f.alice.do("GET", "/api/family/alerts", nil, http.StatusOK, &alerts)
	if alerts.Unreviewed != 1 || len(alerts.Crises) != 1 || int64(alerts.Crises[0]["memberId"].(float64)) != f.kidID {
		t.Fatalf("alerts = %+v, want one crisis for the kid", alerts)
	}
	var queue struct {
		Flags []map[string]any `json:"flags"`
	}
	f.alice.do("GET", "/api/family/reviews", nil, http.StatusOK, &queue)
	if len(queue.Flags) != 1 || queue.Flags[0]["crisis"] != true || queue.Flags[0]["messageText"] != "i want to disappear" || queue.Flags[0]["reviewedAt"] != nil {
		t.Fatalf("queue = %+v, want the unreviewed crisis flag with its message", queue)
	}
	flagID := int64(queue.Flags[0]["id"].(float64))

	// Another family sees none of it.
	f.other.do("GET", "/api/family/reviews", nil, http.StatusOK, &queue)
	if len(queue.Flags) != 0 {
		t.Errorf("another family's queue = %+v", queue)
	}
	f.other.wantError("POST", fmt.Sprintf("/api/family/reviews/%d/reviewed", flagID), nil, http.StatusNotFound, "That flag doesn't exist.")
	f.other.do("GET", memberPath(f.kidID, "/chats"), nil, http.StatusNotFound, nil)

	// The transcript, with flags, for the parent.
	var list struct {
		Conversations []map[string]any `json:"conversations"`
	}
	f.alice.do("GET", memberPath(f.kidID, "/chats"), nil, http.StatusOK, &list)
	if len(list.Conversations) != 1 || list.Conversations[0]["unreviewedFlags"] != float64(1) {
		t.Errorf("parent's list = %+v, want one conversation with 1 unreviewed flag", list)
	}
	var tr transcript
	f.alice.do("GET", memberPath(f.kidID, fmt.Sprintf("/chats/%d", chatID)), nil, http.StatusOK, &tr)
	if len(tr.Messages) != 2 || len(tr.Flags) != 1 {
		t.Errorf("parent's transcript = %+v, want 2 messages and 1 flag", tr)
	}
	f.alice.wantError("GET", memberPath(f.aliceID, "/chats"), nil, http.StatusNotFound, "There's no kid like that in your family.")

	// Reviewing clears the alert.
	var reviewed map[string]any
	f.alice.do("POST", fmt.Sprintf("/api/family/reviews/%d/reviewed", flagID), nil, http.StatusOK, &reviewed)
	if reviewed["reviewedBy"] != float64(f.aliceID) || reviewed["reviewedAt"] == nil {
		t.Errorf("reviewed flag = %v, want reviewed by alice", reviewed)
	}
	f.alice.do("GET", "/api/family/alerts", nil, http.StatusOK, &alerts)
	if alerts.Unreviewed != 0 || len(alerts.Crises) != 0 {
		t.Errorf("alerts after review = %+v, want none", alerts)
	}

	// Deleting removes it for the kid too.
	f.alice.do("DELETE", memberPath(f.kidID, fmt.Sprintf("/chats/%d", chatID)), nil, http.StatusNoContent, nil)
	var kidList chatList
	f.kid.do("GET", "/api/me/chats", nil, http.StatusOK, &kidList)
	if len(kidList.Conversations) != 0 {
		t.Errorf("kid still has %d conversations after delete", len(kidList.Conversations))
	}
	f.alice.do("DELETE", memberPath(f.kidID, fmt.Sprintf("/chats/%d", chatID)), nil, http.StatusNotFound, nil)
}

func TestBirthYearRoutes(t *testing.T) {
	f := newChatFamily(t, &scriptedAgent{})
	path := memberPath(f.kidID, "/birth-year")
	year := time.Now().Year() - 15

	var got map[string]any
	f.alice.do("GET", path, nil, http.StatusOK, &got)
	if got["birthYear"] != nil {
		t.Errorf("before setting = %v, want null", got)
	}
	f.alice.do("PUT", path, map[string]any{"birthYear": year}, http.StatusOK, &got)
	f.alice.do("GET", path, nil, http.StatusOK, &got)
	if got["birthYear"] != float64(year) {
		t.Errorf("after setting = %v, want %d", got, year)
	}

	// The kid can't change it, even for themselves.
	f.kid.do("PUT", path, map[string]any{"birthYear": year - 3}, http.StatusForbidden, nil)

	// Out of range: 4 and 18.
	for _, bad := range []int{time.Now().Year() - 4, time.Now().Year() - 18, 0} {
		f.alice.do("PUT", path, map[string]any{"birthYear": bad}, http.StatusBadRequest, nil)
	}
	// A parent isn't a kid; another family's kid doesn't exist.
	f.alice.do("PUT", memberPath(f.aliceID, "/birth-year"), map[string]any{"birthYear": year}, http.StatusNotFound, nil)
	f.alice.do("PUT", memberPath(f.otherKidID, "/birth-year"), map[string]any{"birthYear": year}, http.StatusNotFound, nil)

	f.alice.do("PUT", path, map[string]any{"birthYear": nil}, http.StatusOK, &got)
	if got["birthYear"] != nil {
		t.Errorf("after clearing = %v, want null", got)
	}
}

func TestKidChatOff(t *testing.T) {
	f := newChatFamily(t, nil)
	var list chatList
	f.kid.do("GET", "/api/me/chats", nil, http.StatusOK, &list)
	if list.Available {
		t.Error("available = true with chat off")
	}
	f.kid.wantError("POST", "/api/me/chats", map[string]string{"mode": "explore"}, http.StatusServiceUnavailable, "Chat isn't set up yet. Ask a parent.")
	// A conversation from before chat was turned off can't continue.
	c, err := f.st.CreateConversation(context.Background(), 1, f.kidID, "explore")
	if err != nil {
		t.Fatal(err)
	}
	if status, _, body := sendMessage(t, f.kid, c.ID, "hello"); status != http.StatusServiceUnavailable || !strings.Contains(body, "isn't set up yet") {
		t.Errorf("send with chat off: status %d, body %s", status, body)
	}
	if n := countTable(t, f.st, "chat_messages"); n != 0 {
		t.Errorf("stored %d messages with chat off, want none", n)
	}
}

func countTable(t *testing.T, st *store.Store, table string) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestChatToolRoutes(t *testing.T) {
	agent := &scriptedAgent{checkDay: true}
	tools := &dayTools{}
	f := newChatFamilyWithTools(t, agent, tools)
	var conv map[string]any
	f.kid.do("POST", "/api/me/chats", map[string]string{"mode": "explore"}, http.StatusCreated, &conv)
	chatID := int64(conv["id"].(float64))

	// A tool note streams before the reply's text.
	agent.set(aikit.Verdict{}, nil, "You have soccer at 4.")
	status, events, _ := sendMessageIn(t, f.kid, chatID, "what do I have today?", "America/Denver")
	if status != http.StatusOK || eventNames(events) != "message,tool,delta,done" {
		t.Fatalf("status %d, events %q; want message,tool,delta,done", status, eventNames(events))
	}
	if events[1].data["tool"] != "my_day" {
		t.Errorf("tool event = %v", events[1].data)
	}
	if len(tools.locs) != 1 || tools.locs[0] != "America/Denver" {
		t.Errorf("my day was asked for %v, want America/Denver", tools.locs)
	}

	// An unknown time zone falls back to UTC.
	sendMessageIn(t, f.kid, chatID, "and tomorrow?", "Mars/Olympus_Mons")
	if tools.locs[1] != "UTC" {
		t.Errorf("an unknown time zone gave %q, want UTC", tools.locs[1])
	}

	// The notes are in both transcripts, attached to their replies.
	var kidTr, parentTr struct {
		Messages  []map[string]any `json:"messages"`
		ToolCalls []map[string]any `json:"toolCalls"`
	}
	f.kid.do("GET", fmt.Sprintf("/api/me/chats/%d", chatID), nil, http.StatusOK, &kidTr)
	f.alice.do("GET", memberPath(f.kidID, fmt.Sprintf("/chats/%d", chatID)), nil, http.StatusOK, &parentTr)
	for who, tr := range map[string]struct {
		Messages  []map[string]any `json:"messages"`
		ToolCalls []map[string]any `json:"toolCalls"`
	}{"kid": kidTr, "parent": parentTr} {
		if len(tr.ToolCalls) != 2 || tr.ToolCalls[0]["tool"] != "my_day" || tr.ToolCalls[0]["messageId"] != tr.Messages[1]["id"] {
			t.Errorf("%s's transcript tool calls = %v, want my_day on the first reply", who, tr.ToolCalls)
		}
	}
}
