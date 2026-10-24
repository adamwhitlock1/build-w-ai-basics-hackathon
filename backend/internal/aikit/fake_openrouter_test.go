package aikit_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

// reply is one canned HTTP response from the fake OpenRouter.
type reply struct {
	status int
	body   string
}

// completion is a successful chat completion answering content.
func completion(content string, prompt, completionTokens int) reply {
	body, _ := json.Marshal(map[string]any{
		"id":     "gen-1",
		"object": "chat.completion",
		"model":  "test/model",
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": prompt, "completion_tokens": completionTokens, "total_tokens": prompt + completionTokens},
	})
	return reply{status: http.StatusOK, body: string(body)}
}

// apiFailure is an OpenRouter error answer with an HTTP status.
func apiFailure(status int) reply {
	return reply{status: status, body: `{"error":{"message":"canned failure","code":` + strconv.Itoa(status) + `}}`}
}

// sentMessage is one chat message the fake received.
type sentMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // a string, or a list of content parts
}

// fakeOpenRouter serves canned replies to POST /chat/completions, in order,
// and records each request's messages. The legacy client and the Genkit
// plugin can both be pointed at it through their base URL.
type fakeOpenRouter struct {
	t       *testing.T
	mu      sync.Mutex
	replies []reply
	sent    [][]sentMessage
}

func newFakeOpenRouter(t *testing.T, replies ...reply) (*fakeOpenRouter, string) {
	f := &fakeOpenRouter{t: t, replies: replies}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func (f *fakeOpenRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/chat/completions" {
		f.t.Errorf("unexpected path %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Messages []sentMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Errorf("decode request: %v", err)
	}
	f.sent = append(f.sent, req.Messages)

	if len(f.replies) == 0 {
		f.t.Error("more requests than canned replies")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	next := f.replies[0]
	f.replies = f.replies[1:]
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(next.status)
	w.Write([]byte(next.body))
}

// requests returns the messages of every request received, in order.
func (f *fakeOpenRouter) requests() [][]sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]sentMessage(nil), f.sent...)
}
