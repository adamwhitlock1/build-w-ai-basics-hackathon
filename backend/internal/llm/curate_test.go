package llm_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
)

func TestCurateArticles(t *testing.T) {
	tests := []struct {
		name         string
		replies      []reply
		want         []string
		wantErr      error // matched with errors.Is
		wantUpstream bool  // an *llm.UpstreamError is expected
		wantCalls    int
	}{
		{
			name: "trims, dedupes, and caps the model's queries",
			replies: []reply{ok(`{"queries":["  space exploration   news ", "Soccer", "soccer", "", "Mars rover",
				"NASA Artemis", "Premier League", "chess openings"]}`)},
			want:      []string{"space exploration news", "Soccer", "Mars rover", "NASA Artemis", "Premier League"},
			wantCalls: 1,
		},
		{
			name:      "no usable queries, then fixed on retry",
			replies:   []reply{ok(`{"queries":["  ", ""]}`), ok(`{"queries":["soccer"]}`)},
			want:      []string{"soccer"},
			wantCalls: 2,
		},
		{
			name:      "no usable queries twice",
			replies:   []reply{ok(`{"queries":[]}`), ok(`{"queries":[" "]}`)},
			wantErr:   llm.ErrNoQueries,
			wantCalls: 2,
		},
		{
			name:         "OpenRouter rejects the key",
			replies:      []reply{{status: 401, body: `{"error":{"message":"No auth credentials found","code":401}}`}},
			wantUpstream: true,
			wantCalls:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newFake(t, tt.replies...)

			got, err := client.CurateArticles(context.Background(), "space exploration, soccer", sources.Everyone)

			if n := len(fake.reqs); n != tt.wantCalls {
				t.Errorf("made %d calls, want %d", n, tt.wantCalls)
			}
			var upstream *llm.UpstreamError
			switch {
			case tt.wantUpstream:
				if !errors.As(err, &upstream) {
					t.Fatalf("err = %v, want an UpstreamError", err)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("queries = %q, want %q", got, tt.want)
			}

			req := fake.reqs[0]
			if rf := req.ResponseFormat; rf.JSONSchema.Name != "article_queries" || !rf.JSONSchema.Strict {
				t.Errorf("response_format = %+v, want strict json_schema named article_queries", rf)
			}
			if len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "space exploration, soccer") {
				t.Errorf("user message should carry the interests; messages = %+v", req.Messages)
			}
		})
	}
}

func TestCurateEvents(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.Local)
	fake, client := newFake(t, ok(`{"queries":["Denver indie concerts October 2026","Denver soccer games October 2026",
		"Denver science museum events October 2026","Denver farmers markets October 2026"]}`))

	got, err := client.CurateEvents(context.Background(), "", "Denver, CO", now, sources.Everyone)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != llm.MaxEventQueries {
		t.Errorf("got %d queries, want them capped at %d: %q", len(got), llm.MaxEventQueries, got)
	}

	req := fake.reqs[0]
	if name := req.ResponseFormat.JSONSchema.Name; name != "event_queries" {
		t.Errorf("schema name = %q, want event_queries", name)
	}
	if !strings.Contains(req.Messages[0].Content, "October 2026") {
		t.Error("system prompt should anchor queries to the current month and year")
	}
	if user := req.Messages[1].Content; !strings.Contains(user, "Denver, CO") || !strings.Contains(user, "(none given)") {
		t.Errorf("user message = %q, want the location and a marker for blank interests", user)
	}
}

func TestFallbackArticleQueries(t *testing.T) {
	tests := []struct {
		interests string
		want      []string
	}{
		{"space exploration, soccer", []string{"space exploration", "soccer"}},
		{"space exploration, soccer and cooking", []string{"space exploration", "soccer", "cooking"}},
		{"Rockets AND gardening; baking\nchess & go.", []string{"Rockets", "gardening", "baking", "chess", "go"}},
		{"soccer, Soccer,  soccer ", []string{"soccer"}},
		{"android apps, sandwiches", []string{"android apps", "sandwiches"}}, // "and" only as a whole word
		{"a, b, c, d, e, f, g", []string{"a", "b", "c", "d", "e"}},
		{", and ,", []string{}},
		{"", []string{}},
	}
	for _, tt := range tests {
		got := llm.FallbackArticleQueries(tt.interests)
		if !slices.Equal(got, tt.want) {
			t.Errorf("FallbackArticleQueries(%q) = %q, want %q", tt.interests, got, tt.want)
		}
	}
}

func TestFallbackEventQueries(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.Local)
	got := llm.FallbackEventQueries("Denver, CO", now)
	want := []string{"Denver, CO events October 2026", "things to do in Denver, CO this weekend"}
	if !slices.Equal(got, want) {
		t.Errorf("FallbackEventQueries = %q, want %q", got, want)
	}
}

func TestExtractEvents(t *testing.T) {
	results := []sources.Result{
		{Title: "Red Rocks concerts", URL: "https://example.com/red-rocks", Thumbnail: "https://img.example.com/rr.jpg"},
		{Title: "Denver calendar", URL: "https://example.com/calendar"},
	}
	// Dates relative to today, so the test means the same thing on any day.
	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format(time.DateOnly) }

	// event writes one event as the model would.
	event := func(name, sortDate, url string) string {
		return `{"name":"` + name + `","date":"` + name + ` date","sort_date":"` + sortDate +
			`","venue":"Venue","description":"About ` + name + `","url":"` + url + `"}`
	}
	// want is the Event that event(name, sortDate, "") becomes.
	want := func(name, sortDate string) llm.Event {
		return llm.Event{Name: name, Date: name + " date", SortDate: sortDate, Venue: "Venue", Description: "About " + name}
	}

	tests := []struct {
		name         string
		reply        reply
		want         []llm.Event
		wantUpstream bool
	}{
		{
			name: "keeps dated upcoming events, soonest first, at most 5",
			reply: ok(`{"events":[` + strings.Join([]string{
				event("Yesterday", day(-1), ""),
				event("Undated", "", ""),
				event("Vague", "next week", ""),
				event("In ten days", day(10), ""),
				event("In three days", day(3), ""),
				event("Today", day(0), ""),
				event("In five days", day(5), ""),
				event("Tomorrow", day(1), ""),
				event("In two days", day(2), ""),
			}, ",") + `]}`),
			want: []llm.Event{
				want("Today", day(0)),
				want("Tomorrow", day(1)),
				want("In two days", day(2)),
				want("In three days", day(3)),
				want("In five days", day(5)),
			},
		},
		{
			name: "grounds URLs, copies thumbnails, drops nameless and repeated events",
			reply: ok(`{"events":[` + strings.Join([]string{
				event("Concert", day(2), "https://example.com/red-rocks"),
				event("Made-up link", day(3), "https://invented.example/x"),
				event("", day(4), ""),
				event("concert", day(2), "https://example.com/calendar"), // same event, other page
				`{"name":"No readable date","date":"","sort_date":"` + day(30) + `","venue":"","description":"","url":""}`,
			}, ",") + `]}`),
			want: []llm.Event{
				{Name: "Concert", Date: "Concert date", SortDate: day(2), Venue: "Venue", Description: "About Concert",
					URL: "https://example.com/red-rocks", Thumbnail: "https://img.example.com/rr.jpg"},
				want("Made-up link", day(3)),
				{Name: "No readable date", Date: time.Now().AddDate(0, 0, 30).Format("Mon, Jan 2"), SortDate: day(30)},
			},
		},
		{
			// Nothing upcoming is an honest empty card, not an error.
			name:  "everything already happened",
			reply: ok(`{"events":[` + event("Yesterday", day(-1), "") + `]}`),
			want:  []llm.Event{},
		},
		{
			name:  "no events found",
			reply: ok(`{"events":[]}`),
			want:  []llm.Event{},
		},
		{
			name:         "OpenRouter is down",
			reply:        reply{status: 503, body: `{"error":{"message":"overloaded","code":503}}`},
			wantUpstream: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newFake(t, tt.reply)

			events, err := client.ExtractEvents(context.Background(), "indie music", "Denver, CO", results, time.Now(), sources.Everyone)

			var upstream *llm.UpstreamError
			if tt.wantUpstream {
				if !errors.As(err, &upstream) {
					t.Fatalf("err = %v, want an UpstreamError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if events == nil {
				t.Error("events is nil, want an empty slice (encodes as [] not null)")
			}
			if !slices.Equal(events, tt.want) {
				t.Errorf("events =\n%+v\nwant\n%+v", events, tt.want)
			}
			if name := fake.reqs[0].ResponseFormat.JSONSchema.Name; name != "local_events" {
				t.Errorf("schema name = %q, want local_events", name)
			}
			if strings.Contains(fake.raw[0], "$ref") {
				t.Error("events schema should be inlined, but contains $ref")
			}
			if user := fake.reqs[0].Messages[1].Content; !strings.Contains(user, "https://example.com/red-rocks") {
				t.Error("user message should include the search results")
			}
		})
	}
}

// TestKidGuidance checks that every model call adds the under-18 guidance
// to its system prompt for kids, and only for kids.
func TestKidGuidance(t *testing.T) {
	results := []sources.Result{{Title: "Launch schedule", URL: "https://example.com/schedule"}}
	spec := llm.WidgetSpec{Title: "SpaceX Launches", Source: "events_search", Query: "q", Archetype: llm.ArchetypeList}
	shaped := `{"future_only":false,"items":[{"title":"Launch","url":"","snippet":"","date":"","label":"","value":"","sort_date":""}]}`

	calls := []struct {
		name  string
		reply string
		call  func(*llm.Client, sources.Audience) error
	}{
		{"curate articles", `{"queries":["space news"]}`, func(c *llm.Client, aud sources.Audience) error {
			_, err := c.CurateArticles(context.Background(), "space", aud)
			return err
		}},
		{"curate events", `{"queries":["Denver events"]}`, func(c *llm.Client, aud sources.Audience) error {
			_, err := c.CurateEvents(context.Background(), "", "Denver, CO", time.Now(), aud)
			return err
		}},
		{"extract events", `{"events":[]}`, func(c *llm.Client, aud sources.Audience) error {
			_, err := c.ExtractEvents(context.Background(), "", "Denver, CO", results, time.Now(), aud)
			return err
		}},
		{"pick widget spec", validSpec, func(c *llm.Client, aud sources.Audience) error {
			_, err := c.PickWidgetSpec(context.Background(), "upcoming SpaceX launches", testCatalog, aud)
			return err
		}},
		{"shape items", shaped, func(c *llm.Client, aud sources.Audience) error {
			_, err := c.ShapeItems(context.Background(), "upcoming SpaceX launches", spec, results, aud)
			return err
		}},
	}
	audiences := []struct {
		name string
		aud  sources.Audience
		want bool
	}{
		{"kids", sources.Kids, true},
		{"everyone", sources.Everyone, false},
	}
	for _, c := range calls {
		for _, a := range audiences {
			t.Run(c.name+"/"+a.name, func(t *testing.T) {
				fake, client := newFake(t, ok(c.reply))
				if err := c.call(client, a.aud); err != nil {
					t.Fatalf("call: %v", err)
				}
				if len(fake.reqs) != 1 || len(fake.reqs[0].Messages) == 0 {
					t.Fatalf("got %d requests, want 1 with messages", len(fake.reqs))
				}
				system := fake.reqs[0].Messages[0]
				if system.Role != "system" {
					t.Fatalf("first message role = %q, want system", system.Role)
				}
				if got := strings.Contains(system.Content, llm.KidGuidance); got != a.want {
					t.Errorf("system prompt has kid guidance = %v, want %v", got, a.want)
				}
			})
		}
	}
}
