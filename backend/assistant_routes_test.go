package main

// End-to-end tests of the parents' Assistant routes, with fake agents.

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/sqlite3store"

	"porchlight/internal/aikit"
	"porchlight/internal/assistant"
	"porchlight/internal/auth"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// parentAgent replies after using the family's-day tool once.
type parentAgent struct{ reply string }

func (a *parentAgent) Reply(ctx context.Context, in aikit.ParentReplyInput, onChunk func(string)) (string, error) {
	if in.Tools != nil {
		in.Tools.OnTool(aikit.ToolNote{Tool: aikit.ToolFamilyDay, Detail: "everyone"})
	}
	onChunk(a.reply)
	return a.reply, nil
}

// noTools is assistant.Tools that finds nothing.
type noTools struct{}

func (noTools) Search(context.Context, string) ([]aikit.SearchResult, error) { return nil, nil }
func (noTools) FamilyDay(context.Context, store.User, string, *time.Location) (string, error) {
	return "", nil
}
func (noTools) Weather(context.Context, store.User) (string, error) { return "", nil }
func (noTools) MyPicks(context.Context, store.User) (string, error) { return "", nil }
func (noTools) KidsChats(context.Context, store.User, aikit.KidsChatsQuery) (string, error) {
	return "", nil
}

type assistantFamily struct {
	srv            *httptest.Server
	maya, sam, kid *client
}

// newAssistantFamily serves the API with the Assistant on fake agents (kids
// nil: "About the kids" off), and logs in Maya and Sam (parents) and a kid.
func newAssistantFamily(t *testing.T, kidsAgent assistant.Agent) assistantFamily {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewSessionManager(sqlite3store.NewWithCleanupInterval(st.DB(), 0), logger)
	svc := assistant.NewService(&parentAgent{reply: "Jordan has soccer at 4."}, kidsAgent, st, logger)
	svc.SetTools(noTools{})
	srv := httptest.NewServer(routes(deps{
		Logger: logger, Store: st, Sessions: sessions,
		Planner: fakePlanner{}, Curator: fakeCurator{}, Briefer: &countingBriefer{},
		Sources:   sources.NewRegistry(fakeNews{}, fakeSource{}, fakeWeather{}),
		Calendars: fakeCalendars{},
		Assistant: svc,
	}))
	t.Cleanup(func() { st.Close() })
	t.Cleanup(srv.Close)

	f := assistantFamily{srv: srv, maya: newClient(t, srv), sam: newClient(t, srv), kid: newClient(t, srv)}
	me := f.maya.signup("maya@example.com")
	f.maya.setUpFamily()
	f.sam.accept(tokenOf(f.maya.invite(me.Family.ID, "sam@example.com", "parent", http.StatusCreated)))
	f.kid.accept(tokenOf(f.maya.invite(me.Family.ID, "kid@example.com", "kid", http.StatusCreated)))
	return f
}

func TestAssistantRoutes(t *testing.T) {
	f := newAssistantFamily(t, nil)

	var list struct {
		Conversations []map[string]any `json:"conversations"`
		KidsAvailable bool             `json:"kidsAvailable"`
	}
	f.maya.do("GET", "/api/me/assistant", nil, http.StatusOK, &list)
	if len(list.Conversations) != 0 || list.KidsAvailable {
		t.Fatalf("list = %+v, want empty with About the kids off", list)
	}
	f.maya.wantError("POST", "/api/me/assistant", map[string]string{"kind": "kids"}, http.StatusConflict, `"About the kids" needs kid chat to be set up.`)

	var conv map[string]any
	f.maya.do("POST", "/api/me/assistant", map[string]string{"kind": "anything"}, http.StatusCreated, &conv)
	id := int64(conv["id"].(float64))
	path := fmt.Sprintf("/api/me/assistant/%d", id)

	// The reply streams like kid chat's, with the tool note first.
	status, events, _ := postStream(t, f.maya, path+"/messages", "what does everyone have tomorrow?")
	if status != http.StatusOK || eventNames(events) != "message,tool,delta,done" {
		t.Fatalf("status %d, events %q; want message,tool,delta,done", status, eventNames(events))
	}
	var tr struct {
		Conversation map[string]any   `json:"conversation"`
		Messages     []map[string]any `json:"messages"`
		ToolCalls    []map[string]any `json:"toolCalls"`
	}
	f.maya.do("GET", path, nil, http.StatusOK, &tr)
	if len(tr.Messages) != 2 || tr.Messages[0]["role"] != "parent" || len(tr.ToolCalls) != 1 || tr.Conversation["title"] != "what does everyone have tomorrow?" {
		t.Errorf("transcript = %+v", tr)
	}

	// Private: Sam and the kid can't see it; the kid can't use the Assistant.
	f.sam.wantError("GET", path, nil, http.StatusNotFound, "That conversation doesn't exist.")
	f.sam.do("DELETE", path, nil, http.StatusNotFound, nil)
	f.kid.wantError("GET", "/api/me/assistant", nil, http.StatusForbidden, "The Assistant is for parents.")
	f.kid.wantError("POST", "/api/me/assistant", map[string]string{"kind": "anything"}, http.StatusForbidden, "The Assistant is for parents.")

	// Cross-site posts are refused.
	headers := map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"}
	if status := sendRaw(t, f.maya, "POST", path+"/messages", `{"text":"hi"}`, headers); status != http.StatusForbidden {
		t.Errorf("cross-site send: status %d, want 403", status)
	}

	// Maya deletes her own.
	f.maya.do("DELETE", path, nil, http.StatusNoContent, nil)
	f.maya.do("GET", path, nil, http.StatusNotFound, nil)
}

func TestAssistantAboutTheKids(t *testing.T) {
	f := newAssistantFamily(t, &parentAgent{reply: "Lily has one flag."})
	var conv map[string]any
	f.maya.do("POST", "/api/me/assistant", map[string]string{"kind": "kids"}, http.StatusCreated, &conv)
	if conv["kind"] != "kids" {
		t.Errorf("conversation = %v", conv)
	}
	_, events, _ := postStream(t, f.maya, fmt.Sprintf("/api/me/assistant/%d/messages", int64(conv["id"].(float64))), "anything about Lily?")
	if done := events[len(events)-1]; done.name != "done" || !strings.Contains(done.data["text"].(string), "Lily has one flag.") {
		t.Errorf("last event = %+v, want the kids' agent's reply", done)
	}
}
