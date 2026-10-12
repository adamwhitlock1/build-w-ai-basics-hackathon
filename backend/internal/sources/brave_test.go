package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

const webBody = `{
  "web": {"results": [{
    "title": "SpaceX <strong>Launch</strong> Schedule &amp; Updates",
    "url": "https://example.com/launches",
    "description": "Next up: <strong>Starlink</strong> 10-12 on Jun&nbsp;12.",
    "age": "2 days ago",
    "thumbnail": {"src": "https://img.example.com/t.jpg"},
    "meta_url": {"scheme": "https", "netloc": "example.com", "hostname": "www.example.com", "path": "› launches"},
    "profile": {"name": "Example", "long_name": "example.com"},
    "extra_snippets": ["Falcon 9 <em>lifts off</em> at 9pm", "  "]
  }]}
}`

const newsBody = `{
  "results": [{
    "title": "Crew-11 docks",
    "url": "https://news.example.com/crew-11",
    "description": "The capsule arrived &quot;on time&quot;.",
    "page_age": "2025-06-10T08:00:00",
    "meta_url": {"scheme": "https", "netloc": "news.example.com", "hostname": "news.example.com"}
  }]
}`

func TestBraveSearch(t *testing.T) {
	tests := []struct {
		name       string
		source     func(*BraveClient) *BraveSource
		wantPath   string
		wantParams map[string]string // query params that must be present ("" = must be absent)
		want       Result
	}{
		{
			name:       "web strips html and keeps extras",
			source:     (*BraveClient).Web, // a method expression: func(*BraveClient) *BraveSource
			wantPath:   "/web/search",
			wantParams: map[string]string{"q": "spacex launches", "count": "10", "extra_snippets": "true", "freshness": ""},
			want: Result{
				Title:         "SpaceX Launch Schedule & Updates",
				URL:           "https://example.com/launches",
				Snippet:       "Next up: Starlink 10-12 on Jun 12.",
				Date:          "2 days ago",
				Thumbnail:     "https://img.example.com/t.jpg",
				Site:          "example.com",
				ExtraSnippets: []string{"Falcon 9 lifts off at 9pm"},
			},
		},
		{
			name:       "events is web search with freshness",
			source:     (*BraveClient).Events,
			wantPath:   "/web/search",
			wantParams: map[string]string{"freshness": "pm"},
			want: Result{
				Title:         "SpaceX Launch Schedule & Updates",
				URL:           "https://example.com/launches",
				Snippet:       "Next up: Starlink 10-12 on Jun 12.",
				Date:          "2 days ago",
				Thumbnail:     "https://img.example.com/t.jpg",
				Site:          "example.com",
				ExtraSnippets: []string{"Falcon 9 lifts off at 9pm"},
			},
		},
		{
			name:       "news reads top-level results and falls back to page_age",
			source:     (*BraveClient).News,
			wantPath:   "/news/search",
			wantParams: map[string]string{"q": "spacex launches", "count": "10"},
			want: Result{
				Title:         "Crew-11 docks",
				URL:           "https://news.example.com/crew-11",
				Snippet:       `The capsule arrived "on time".`,
				Date:          "2025-06-10",
				Site:          "news.example.com",
				ExtraSnippets: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("X-Subscription-Token"); got != "test-key" {
					t.Errorf("X-Subscription-Token = %q, want test-key", got)
				}
				if got := r.Header.Get("Accept"); got != "application/json" {
					t.Errorf("Accept = %q, want application/json", got)
				}
				if r.URL.Path != tt.wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, tt.wantPath)
				}
				for k, want := range tt.wantParams {
					if got := r.URL.Query().Get(k); got != want {
						t.Errorf("param %s = %q, want %q", k, got, want)
					}
				}
				if r.URL.Path == "/news/search" {
					w.Write([]byte(newsBody))
				} else {
					w.Write([]byte(webBody))
				}
			}))
			defer srv.Close()

			src := tt.source(NewBraveClient("test-key", srv.URL))
			got, err := src.Search(context.Background(), "spacex launches", Everyone)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d results, want 1", len(got))
			}
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Errorf("result mismatch\n got: %+v\nwant: %+v", got[0], tt.want)
			}
		})
	}
}

func TestBraveSafeSearch(t *testing.T) {
	sources := []struct {
		name   string
		source func(*BraveClient) *BraveSource
	}{
		{"news", (*BraveClient).News},
		{"web", (*BraveClient).Web},
		{"events", (*BraveClient).Events},
	}
	audiences := []struct {
		name string
		aud  Audience
		want string // "" = the param must be absent
	}{
		{"kids get strict", Kids, "strict"},
		{"everyone gets Brave's default", Everyone, ""},
	}
	for _, src := range sources {
		for _, a := range audiences {
			t.Run(src.name+"/"+a.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					if got := q.Get("safesearch"); got != a.want || q.Has("safesearch") != (a.want != "") {
						t.Errorf("safesearch = %q (present: %v), want %q", got, q.Has("safesearch"), a.want)
					}
					if r.URL.Path == "/news/search" {
						w.Write([]byte(newsBody))
					} else {
						w.Write([]byte(webBody))
					}
				}))
				defer srv.Close()

				if _, err := src.source(NewBraveClient("test-key", srv.URL)).Search(context.Background(), "q", a.aud); err != nil {
					t.Fatalf("Search: %v", err)
				}
			})
		}
	}
}

func TestBraveResultSite(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		profile  string
		want     string
	}{
		{"host without www", "www.space.com", "Space", "space.com"},
		{"host as is", "news.example.com", "", "news.example.com"},
		{"no host falls back to the profile name", "", "Wikipedia", "Wikipedia"},
		{"neither", "", "", ""},
	}
	for _, tt := range tests {
		var r braveResult
		r.MetaURL.Hostname = tt.hostname
		r.Profile.Name = tt.profile
		if got := r.normalize().Site; got != tt.want {
			t.Errorf("%s: Site = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestBraveRetriesRateLimitOnce(t *testing.T) {
	tests := []struct {
		name      string
		failures  int32 // how many 429s the server sends before succeeding
		wantErr   error
		wantCalls int32
	}{
		{name: "recovers after one 429", failures: 1, wantCalls: 2},
		{name: "gives up after second 429", failures: 2, wantErr: ErrRateLimited, wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32 // handlers run on their own goroutines
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) <= tt.failures {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.Write([]byte(newsBody))
			}))
			defer srv.Close()

			c := NewBraveClient("test-key", srv.URL)
			c.retryDelay = time.Millisecond
			_, err := c.News().Search(context.Background(), "q", Everyone)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("server called %d times, want %d", got, tt.wantCalls)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	c := NewBraveClient("k", "")
	r := NewRegistry(c.News(), c.Events(), c.Web())

	if got, want := r.Names(), []string{"news_search", "events_search", "web_search"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	if _, ok := r.Get("web_search"); !ok {
		t.Error(`Get("web_search") not found`)
	}
	if _, ok := r.Get("weather"); ok {
		t.Error(`Get("weather") found, want missing`)
	}
}
