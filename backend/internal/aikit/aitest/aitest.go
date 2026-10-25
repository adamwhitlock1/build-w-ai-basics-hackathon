// Package aitest builds an aikit.Kit on a real Genkit instance whose only
// model is scripted: it answers from a queue of replies and records what it
// was sent. Tests get Genkit's real pipeline (schema instructions,
// middleware, output parsing) with no network.
package aitest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/genkit"
	"github.com/openai/openai-go"

	"kinlight/internal/aikit"
)

// ModelName is the scripted model's Genkit name, and the Kit's default model.
const ModelName = "test/scripted"

// Reply is one scripted model answer. Build one with Text, JSON, ToolCall,
// ToolCalls or Fail.
type Reply struct {
	text   string
	tools  []*ai.ToolRequest
	status int // non-zero: the call fails with this HTTP status
	// failAfter makes a streamed call fail with status after streaming
	// text, the way a connection drops partway through a reply.
	failAfter bool
	finish    ai.FinishReason
	usage     *ai.GenerationUsage
}

// Text answers with text.
func Text(text string) Reply { return Reply{text: text} }

// JSON answers with v encoded as JSON text, the way a model answers a
// request for structured output.
func JSON(v any) Reply {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("aitest.JSON: %v", err))
	}
	return Reply{text: string(b)}
}

// ToolCall answers with a request to call the tool name with input.
func ToolCall(name string, input any) Reply {
	return ToolCalls(Call{Name: name, Input: input})
}

// Call is one tool request in ToolCalls.
type Call struct {
	Name  string
	Input any
}

// ToolCalls answers with several tool requests in one turn, which Genkit
// runs at the same time.
func ToolCalls(calls ...Call) Reply {
	var r Reply
	for _, c := range calls {
		r.tools = append(r.tools, &ai.ToolRequest{Name: c.Name, Input: c.Input})
	}
	return r
}

// Fail makes the call fail the way the OpenRouter plugin does when
// OpenRouter answers with an HTTP error status: a Genkit status.Error
// wrapping the SDK's *openai.Error.
func Fail(httpStatus int) Reply { return Reply{status: httpStatus} }

// StreamThenFail answers a streaming call with text, in chunks, and then
// fails with httpStatus, as when a connection drops partway through.
func StreamThenFail(text string, httpStatus int) Reply {
	return Reply{text: text, status: httpStatus, failAfter: true}
}

// WithUsage makes the reply report token usage.
func (r Reply) WithUsage(input, output int) Reply {
	r.usage = &ai.GenerationUsage{InputTokens: input, OutputTokens: output, TotalTokens: input + output}
	return r
}

// WithFinish makes the reply finish for reason instead of
// ai.FinishReasonStop.
func (r Reply) WithFinish(reason ai.FinishReason) Reply {
	r.finish = reason
	return r
}

// Model is the scripted model. It is safe for concurrent use.
type Model struct {
	t        testing.TB
	mu       sync.Mutex
	replies  []Reply
	requests []*ai.ModelRequest
}

// Requests returns every request the model has been sent, in order.
func (m *Model) Requests() []*ai.ModelRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*ai.ModelRequest(nil), m.requests...)
}

// New returns a Kit whose default model answers with replies, in order, and
// the Model, for inspecting requests. A request beyond the last reply fails
// the test.
func New(t testing.TB, replies ...Reply) (*aikit.Kit, *Model) {
	t.Helper()
	m := &Model{t: t, replies: replies}
	g := genkit.Init(context.Background())
	// The same capabilities the OpenRouter plugin declares: notably no
	// constrained output, so Genkit puts the schema in the prompt.
	genkit.DefineModelAction(g, ModelName, &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, Tools: true, ToolChoice: true, SystemRole: true, Media: true},
	}, m.generate)
	return aikit.FromGenkit(g, ModelName), m
}

// Chunks is how many pieces a streamed text reply arrives in (fewer if it
// has fewer words).
const Chunks = 3

func (m *Model) generate(ctx context.Context, req *ai.ModelRequest, _ map[string]any, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	if len(m.replies) == 0 {
		m.t.Errorf("aitest: request %d has no scripted reply", len(m.requests))
		return nil, status.Errorf(status.ErrInternal, "aitest: no scripted reply left")
	}
	r := m.replies[0]
	m.replies = m.replies[1:]

	if r.status != 0 && !r.failAfter {
		return nil, apiError(r.status)
	}
	// A streaming call gets text replies in chunks, as a provider sends them.
	if cb != nil && len(r.tools) == 0 {
		for _, chunk := range chunks(r.text, Chunks) {
			if err := cb(ctx, &ai.ModelResponseChunk{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart(chunk)}}); err != nil {
				return nil, err
			}
		}
	}
	if r.failAfter {
		return nil, apiError(r.status)
	}
	msg := ai.NewModelTextMessage(r.text)
	if len(r.tools) > 0 {
		parts := make([]*ai.Part, len(r.tools))
		for i, tr := range r.tools {
			parts[i] = ai.NewToolRequestPart(tr)
		}
		msg = ai.NewModelMessage(parts...)
	}
	finish := r.finish
	if finish == "" {
		finish = ai.FinishReasonStop
	}
	return &ai.ModelResponse{Message: msg, FinishReason: finish, Usage: r.usage, Request: req}, nil
}

// apiError builds the error the OpenRouter plugin returns for an HTTP error
// status. The SDK error's Error method reads Request and Response, so both
// are filled in.
func apiError(code int) error {
	req, _ := http.NewRequest(http.MethodPost, "https://openrouter.test/api/v1/chat/completions", nil)
	apiErr := &openai.Error{StatusCode: code, Request: req, Response: &http.Response{StatusCode: code}}
	return status.Errorf(status.Base(status.FromHTTPCode(code)), "%w", apiErr)
}

// chunks splits text into at most n pieces at word boundaries; joined,
// they give text back exactly.
func chunks(text string, n int) []string {
	words := strings.SplitAfter(text, " ")
	if len(words) < n {
		n = len(words)
	}
	var out []string
	per := (len(words) + n - 1) / n
	for i := 0; i < len(words); i += per {
		out = append(out, strings.Join(words[i:min(i+per, len(words))], ""))
	}
	return out
}
