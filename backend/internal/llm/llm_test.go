package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
)

// reply is one canned HTTP response from the fake OpenRouter.
type reply struct {
	status int
	body   string
}

// ok wraps content in a successful chat completion.
func ok(content string) reply { return completion(content, "stop") }

func completion(content, finishReason string) reply {
	body, _ := json.Marshal(map[string]any{
		"id":     "gen-1",
		"object": "chat.completion",
		"model":  "test/model",
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": content},
			"finish_reason": finishReason,
		}},
	})
	return reply{status: http.StatusOK, body: string(body)}
}

// chatRequest is the part of a chat completion request the tests inspect.
type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ResponseFormat struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string `json:"name"`
			Strict bool   `json:"strict"`
			Schema struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

// fakeOpenRouter serves canned replies to POST /chat/completions, in order,
// and records each request.
type fakeOpenRouter struct {
	t       *testing.T
	mu      sync.Mutex
	replies []reply
	reqs    []chatRequest
	raw     []string // request bodies
	headers []http.Header
}

func newFake(t *testing.T, replies ...reply) (*fakeOpenRouter, *llm.Client) {
	f := &fakeOpenRouter{t: t, replies: replies}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	client := llm.NewClient("test-key", "test/model", srv.URL)
	llm.SetRetryDelay(client, time.Millisecond)
	return f, client
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
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Errorf("decode request: %v", err)
	}
	f.reqs = append(f.reqs, req)
	f.raw = append(f.raw, string(body))
	f.headers = append(f.headers, r.Header.Clone())

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

// catalog is a fixed llm.Catalog.
type catalog []string

func (c catalog) Names() []string  { return c }
func (c catalog) Describe() string { return "- " + strings.Join(c, "\n- ") + "\n" }

var testCatalog = catalog{"news_search", "events_search", "web_search"}

const validSpec = `{"title":"SpaceX Launches","source":"events_search","query":"upcoming SpaceX launches","archetype":"list"}`

// rateLimited is OpenRouter's HTTP 429 reply.
var rateLimited = reply{status: http.StatusTooManyRequests, body: `{"error":{"message":"Rate limit exceeded: free-models-per-min","code":429}}`}

func TestPickWidgetSpec(t *testing.T) {
	tests := []struct {
		name         string
		replies      []reply
		wantErr      error  // matched with errors.Is
		wantUpstream string // substring of UpstreamError.Reason; "" = no UpstreamError expected
		wantStatus   int    // UpstreamError.Status: OpenRouter's HTTP error status, or 0
		wantCalls    int
	}{
		{
			name:      "valid structured JSON",
			replies:   []reply{ok(validSpec)},
			wantCalls: 1,
		},
		{
			name:      "malformed JSON then valid on retry",
			replies:   []reply{ok(`{"title": "SpaceX`), ok(validSpec)},
			wantCalls: 2,
		},
		{
			name:      "code-fenced JSON is accepted",
			replies:   []reply{ok("```json\n" + validSpec + "\n```")},
			wantCalls: 1,
		},
		{
			name:      "empty reply then valid on retry",
			replies:   []reply{ok(""), ok(validSpec)},
			wantCalls: 2,
		},
		{
			name:      "rate limited once, then succeeds",
			replies:   []reply{rateLimited, ok(validSpec)},
			wantCalls: 2,
		},
		{
			name:         "rate limited twice",
			replies:      []reply{rateLimited, rateLimited},
			wantUpstream: "rate limiting",
			wantStatus:   http.StatusTooManyRequests,
			wantCalls:    2,
		},
		{
			name: "unknown source twice",
			replies: []reply{
				ok(`{"title":"Weather","source":"weather_api","query":"weather","archetype":"list"}`),
				ok(`{"title":"Weather","source":"weather_api","query":"weather","archetype":"list"}`),
			},
			wantErr:   llm.ErrInvalidSpec,
			wantCalls: 2,
		},
		{
			name:         "HTTP 200 with an error and no choices",
			replies:      []reply{{status: 200, body: `{"error":{"message":"provider returned error","code":502}}`}},
			wantUpstream: "empty answer",
			wantCalls:    1,
		},
		{
			name:         "truncated answer",
			replies:      []reply{completion(`{"title":"Spa`, "length")},
			wantUpstream: "cut off",
			wantCalls:    1,
		},
		{
			name:         "rejected key",
			replies:      []reply{{status: 401, body: `{"error":{"message":"No auth credentials found","code":401}}`}},
			wantUpstream: "API key",
			wantStatus:   http.StatusUnauthorized,
			wantCalls:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newFake(t, tt.replies...)

			spec, err := client.PickWidgetSpec(context.Background(), "show me upcoming SpaceX launches", testCatalog, sources.Everyone, "")

			if got := len(fake.reqs); got != tt.wantCalls {
				t.Errorf("made %d calls, want %d", got, tt.wantCalls)
			}
			var upstream *llm.UpstreamError
			switch {
			case tt.wantUpstream != "":
				if !errors.As(err, &upstream) || !strings.Contains(upstream.Reason, tt.wantUpstream) {
					t.Fatalf("err = %v, want UpstreamError mentioning %q", err, tt.wantUpstream)
				}
				if upstream.Status != tt.wantStatus {
					t.Errorf("UpstreamError.Status = %d, want %d", upstream.Status, tt.wantStatus)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := llm.WidgetSpec{Title: "SpaceX Launches", Source: "events_search", Query: "upcoming SpaceX launches", Archetype: llm.ArchetypeList}
				if spec != want {
					t.Errorf("spec = %+v, want %+v", spec, want)
				}
			}
		})
	}
}

// An assistant message must have content, so a retry after an empty reply
// sends only the corrective user message.
func TestRetryAfterEmptyReply(t *testing.T) {
	fake, client := newFake(t, ok(""), ok(validSpec))
	if _, err := client.PickWidgetSpec(context.Background(), "show me upcoming SpaceX launches", testCatalog, sources.Everyone, ""); err != nil {
		t.Fatal(err)
	}
	if len(fake.reqs) != 2 {
		t.Fatalf("made %d calls, want 2", len(fake.reqs))
	}
	for _, m := range fake.reqs[1].Messages {
		if m.Content == "" {
			t.Errorf("retry sent a %q message with no content: %s", m.Role, fake.raw[1])
		}
	}
}

// TestPickWidgetSpecLocation checks that the member's saved location, and
// the rule for using it, reach the system prompt only when there is one.
func TestPickWidgetSpecLocation(t *testing.T) {
	const rule = "When the request needs a place and names none"
	tests := []struct {
		name     string
		location string
		want     []string // substrings the system prompt must have
		wantNot  []string // and must not have
	}{
		{"saved location", "Austin, TX", []string{`saved location is "Austin, TX"`, rule}, nil},
		{"no location", "", nil, []string{"saved location", rule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newFake(t, ok(validSpec))
			if _, err := client.PickWidgetSpec(context.Background(), "this week's weather", testCatalog, sources.Everyone, tt.location); err != nil {
				t.Fatal(err)
			}
			system := fake.reqs[0].Messages[0].Content
			for _, s := range tt.want {
				if !strings.Contains(system, s) {
					t.Errorf("system prompt is missing %q", s)
				}
			}
			for _, s := range tt.wantNot {
				if strings.Contains(system, s) {
					t.Errorf("system prompt has %q, want it absent", s)
				}
			}
		})
	}
}

func TestPickWidgetSpecRequest(t *testing.T) {
	fake, client := newFake(t, ok(validSpec))
	if _, err := client.PickWidgetSpec(context.Background(), "show me upcoming SpaceX launches", testCatalog, sources.Everyone, ""); err != nil {
		t.Fatal(err)
	}

	req, hdr := fake.reqs[0], fake.headers[0]
	rf := req.ResponseFormat
	if rf.Type != "json_schema" || rf.JSONSchema.Name != "widget_spec" || !rf.JSONSchema.Strict {
		t.Errorf("response_format = %+v, want strict json_schema named widget_spec", rf)
	}
	if got := rf.JSONSchema.Schema.Properties["source"].Enum; !slices.Equal(got, testCatalog) {
		t.Errorf("source enum = %v, want %v", got, testCatalog)
	}
	if got := rf.JSONSchema.Schema.Properties["archetype"].Enum; len(got) != len(llm.Archetypes) {
		t.Errorf("archetype enum = %v, want %d values", got, len(llm.Archetypes))
	}
	if got := hdr.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q", got)
	}
	if hdr.Get("X-Title") != "Porchlight" || hdr.Get("HTTP-Referer") != "http://localhost:5174" {
		t.Errorf("missing attribution headers: %v", hdr)
	}
	if len(req.Messages) != 2 || !strings.Contains(req.Messages[0].Content, "web_search") {
		t.Errorf("system prompt should list the sources; messages = %+v", req.Messages)
	}
}

func TestShapeItems(t *testing.T) {
	results := []sources.Result{
		{Title: "Launch schedule", URL: "https://example.com/schedule", Thumbnail: "https://img.example.com/1.jpg"},
		{Title: "Other page", URL: "https://example.com/other"},
	}
	spec := llm.WidgetSpec{Title: "SpaceX Launches", Source: "events_search", Query: "q", Archetype: llm.ArchetypeList}
	yesterday := time.Now().AddDate(0, 0, -1).Format(time.DateOnly)
	nextWeek := time.Now().AddDate(0, 0, 7).Format(time.DateOnly)

	tests := []struct {
		name    string
		content string
		want    []llm.Item
		wantErr error
	}{
		{
			name: "future_only drops items dated before today",
			content: `{"future_only":true,"items":[
				{"title":"Already flew","url":"","snippet":"","date":"yesterday","label":"","value":"","sort_date":"` + yesterday + `"},
				{"title":"Next week","url":"","snippet":"","date":"next week","label":"","value":"","sort_date":"` + nextWeek + `"},
				{"title":"Date unknown","url":"","snippet":"","date":"","label":"","value":"","sort_date":""}
			]}`,
			want: []llm.Item{{Title: "Next week", Date: "next week"}, {Title: "Date unknown"}},
		},
		{
			name: "past items stay when the request isn't about the future",
			content: `{"future_only":false,"items":[
				{"title":"Yesterday's match","url":"","snippet":"","date":"yesterday","label":"","value":"","sort_date":"` + yesterday + `"}
			]}`,
			want: []llm.Item{{Title: "Yesterday's match", Date: "yesterday"}},
		},
		{
			// Everything found already happened: an honest empty widget, not
			// ErrNoItems (which would fall back to the stale raw results).
			name: "all in the past",
			content: `{"future_only":true,"items":[
				{"title":"Already flew","url":"","snippet":"","date":"","label":"","value":"","sort_date":"` + yesterday + `"}
			]}`,
			want: []llm.Item{},
		},
		{
			name: "grounds URLs and copies thumbnails",
			content: `{"items":[
				{"title":" Starlink 10-12 ","url":"https://example.com/schedule","snippet":"Falcon 9","date":"Jun 12","label":"","value":""},
				{"title":"Crew-11","url":"https://made-up.example/crew","snippet":"","date":"Jul 1","label":"","value":""},
				{"title":"","url":"","snippet":"orphan snippet","date":"","label":"","value":""}
			]}`,
			want: []llm.Item{
				{Title: "Starlink 10-12", URL: "https://example.com/schedule", Snippet: "Falcon 9", Date: "Jun 12", Thumbnail: "https://img.example.com/1.jpg"},
				{Title: "Crew-11", Date: "Jul 1"},
			},
		},
		{
			// The model sometimes repeats a hero's quote as its own context.
			name: "a snippet that repeats the title is dropped",
			content: `{"items":[
				{"title":"Success is liking yourself, liking what you do.","url":"","snippet":"“success is liking yourself — liking what you do”","date":"","label":"Maya Angelou","value":""},
				{"title":"Artemis II","url":"","snippet":"Artemis II crew named","date":"","label":"","value":""}
			]}`,
			want: []llm.Item{
				{Title: "Success is liking yourself, liking what you do.", Label: "Maya Angelou"},
				{Title: "Artemis II", Snippet: "Artemis II crew named"},
			},
		},
		{
			name:    "nothing usable",
			content: `{"items":[]}`,
			wantErr: llm.ErrNoItems,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newFake(t, ok(tt.content))

			items, err := client.ShapeItems(context.Background(), "upcoming SpaceX launches", spec, results, sources.Everyone)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(items, tt.want) {
				t.Errorf("items =\n%+v\nwant\n%+v", items, tt.want)
			}
			if tt.want != nil && items == nil {
				t.Error("items is nil, want an empty slice (encodes as [] not null)")
			}
			if name := fake.reqs[0].ResponseFormat.JSONSchema.Name; name != "widget_items" {
				t.Errorf("schema name = %q, want widget_items", name)
			}
			if strings.Contains(fake.raw[0], "$ref") {
				t.Error("items schema should be inlined, but contains $ref")
			}
		})
	}
}

func TestStartupChecks(t *testing.T) {
	const models = `{"data":[
		{"id":"good/model","supported_parameters":["response_format","structured_outputs"]},
		{"id":"plain/model","supported_parameters":["temperature"]}
	]}`

	tests := []struct {
		name         string
		model        string
		keyStatus    int
		wantModelErr bool // fatal (not ErrUnreachable) error expected
		wantKeyErr   bool
	}{
		{name: "all good", model: "good/model", keyStatus: 200},
		{name: "unknown model", model: "missing/model", keyStatus: 200, wantModelErr: true},
		{name: "no structured outputs", model: "plain/model", keyStatus: 200, wantModelErr: true},
		{name: "rejected key", model: "good/model", keyStatus: 401, wantKeyErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/models":
					w.Write([]byte(models))
				case "/key":
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Errorf("GET /key without bearer auth")
					}
					w.WriteHeader(tt.keyStatus)
					w.Write([]byte(`{"data":{}}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			client := llm.NewClient("test-key", tt.model, srv.URL)

			err := client.CheckModel(context.Background())
			if (err != nil) != tt.wantModelErr || errors.Is(err, llm.ErrUnreachable) {
				t.Errorf("CheckModel() = %v, want fatal error: %v", err, tt.wantModelErr)
			}
			err = client.CheckKey(context.Background())
			if (err != nil) != tt.wantKeyErr || errors.Is(err, llm.ErrUnreachable) {
				t.Errorf("CheckKey() = %v, want fatal error: %v", err, tt.wantKeyErr)
			}
		})
	}

	t.Run("unreachable server is a soft failure", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close() // nothing is listening now
		client := llm.NewClient("test-key", "good/model", srv.URL)
		if err := client.CheckModel(context.Background()); !errors.Is(err, llm.ErrUnreachable) {
			t.Errorf("CheckModel() = %v, want ErrUnreachable", err)
		}
	})
}
