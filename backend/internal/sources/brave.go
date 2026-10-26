package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultBraveBaseURL is the Brave Search API root.
const DefaultBraveBaseURL = "https://api.search.brave.com/res/v1"

// ErrRateLimited is returned when Brave keeps answering HTTP 429 after one retry.
var ErrRateLimited = errors.New("brave search rate limit exceeded")

const resultCount = 10 // Brave allows up to 20 per web request

// BraveClient calls the Brave Search API. One client backs all three Brave
// sources; use News, Web and Events to get them.
type BraveClient struct {
	apiKey     string
	baseURL    string
	http       *http.Client
	retryDelay time.Duration // pause before retrying a 429
}

// NewBraveClient returns a client for the Brave Search API. An empty baseURL
// means DefaultBraveBaseURL.
func NewBraveClient(apiKey, baseURL string) *BraveClient {
	if baseURL == "" {
		baseURL = DefaultBraveBaseURL
	}
	return &BraveClient{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		http:       &http.Client{Timeout: 15 * time.Second},
		retryDelay: time.Second, // the free plan allows one request per second
	}
}

// News returns the "news_search" source: Brave's news vertical.
func (c *BraveClient) News() *BraveSource {
	return &BraveSource{
		client:   c,
		name:     "news_search",
		vertical: newsVertical,
		shape:    "recent news articles (headline, link, short summary, how long ago). Best for headlines, current events, sports results, announcements.",
	}
}

// Web returns the "web_search" source: Brave's general web search.
func (c *BraveClient) Web() *BraveSource {
	return &BraveSource{
		client:   c,
		name:     "web_search",
		vertical: webVertical,
		shape:    "general web pages (title, link, summary, extra excerpts). Best for evergreen facts, quotes, recipes, prices, stats, how-tos.",
	}
}

// Events returns the "events_search" source. Brave has no events vertical, so
// this is web search limited to pages updated in the past month, which
// favors schedules and upcoming-event listings over stale pages.
func (c *BraveClient) Events() *BraveSource {
	return &BraveSource{
		client:    c,
		name:      "events_search",
		vertical:  webVertical,
		freshness: "pm",
		shape:     "web pages about upcoming or recent events, updated in the past month (title, link, summary, extra excerpts). Best for launches, concerts, games, schedules, local happenings.",
	}
}

// vertical selects which Brave endpoint a BraveSource calls.
type vertical int

const (
	webVertical vertical = iota
	newsVertical
)

// BraveSource is a Source backed by one Brave Search endpoint.
type BraveSource struct {
	client    *BraveClient
	name      string
	vertical  vertical
	freshness string // optional Brave freshness filter, e.g. "pm" = past month
	shape     string
}

// Name implements Source.
func (s *BraveSource) Name() string { return s.name }

// Shape implements Source.
func (s *BraveSource) Shape() string { return s.shape }

// Search implements Source.
func (s *BraveSource) Search(ctx context.Context, query string, aud Audience) ([]Result, error) {
	var (
		raw []braveResult
		err error
	)
	switch s.vertical {
	case newsVertical:
		raw, err = s.client.news(ctx, query, aud)
	default:
		raw, err = s.client.web(ctx, query, s.freshness, aud)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.name, err)
	}

	results := make([]Result, 0, len(raw))
	for _, r := range raw {
		results = append(results, r.normalize())
	}
	return results, nil
}

// setSafeSearch asks Brave to filter adult content strictly for kids. For
// everyone else it leaves the parameter out, so Brave uses its default.
func setSafeSearch(params url.Values, aud Audience) {
	if aud == Kids {
		params.Set("safesearch", "strict")
	}
}

// news calls GET /news/search. News results sit at the top level of the body.
func (c *BraveClient) news(ctx context.Context, query string, aud Audience) ([]braveResult, error) {
	params := url.Values{"q": {query}, "count": {strconv.Itoa(resultCount)}}
	setSafeSearch(params, aud)
	var body struct {
		Results []braveResult `json:"results"`
	}
	if err := c.get(ctx, "/news/search", params, &body); err != nil {
		return nil, err
	}
	return body.Results, nil
}

// web calls GET /web/search. Web results are nested under "web".
func (c *BraveClient) web(ctx context.Context, query, freshness string, aud Audience) ([]braveResult, error) {
	params := url.Values{
		"q":              {query},
		"count":          {strconv.Itoa(resultCount)},
		"extra_snippets": {"true"},
	}
	if freshness != "" {
		params.Set("freshness", freshness)
	}
	setSafeSearch(params, aud)
	var body struct {
		Web struct {
			Results []braveResult `json:"results"`
		} `json:"web"`
	}
	if err := c.get(ctx, "/web/search", params, &body); err != nil {
		return nil, err
	}
	return body.Web.Results, nil
}

// braveResult is the subset of a Brave web or news result we use.
type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age"`      // e.g. "2 days ago" or "May 1, 2025"
	PageAge     string `json:"page_age"` // e.g. "2025-05-01T12:00:00"
	Thumbnail   struct {
		Src string `json:"src"`
	} `json:"thumbnail"`
	MetaURL struct {
		Hostname string `json:"hostname"` // e.g. "www.space.com"
	} `json:"meta_url"`
	Profile struct {
		Name string `json:"name"` // the site's name, e.g. "Wikipedia"; web results only
	} `json:"profile"`
	ExtraSnippets []string `json:"extra_snippets"`
}

func (r braveResult) normalize() Result {
	date := r.Age
	if date == "" && len(r.PageAge) >= len("2006-01-02") {
		date = r.PageAge[:len("2006-01-02")]
	}
	extras := make([]string, 0, len(r.ExtraSnippets))
	for _, s := range r.ExtraSnippets {
		if s = cleanText(s); s != "" {
			extras = append(extras, s)
		}
	}
	return Result{
		Title:         cleanText(r.Title),
		URL:           r.URL,
		Snippet:       cleanText(r.Description),
		Date:          date,
		Thumbnail:     r.Thumbnail.Src,
		Site:          r.site(),
		ExtraSnippets: extras,
	}
}

// site names who published the result: the page's host without a leading
// "www.", or else the site name Brave knows it by.
func (r braveResult) site() string {
	if host := strings.TrimPrefix(r.MetaURL.Hostname, "www."); host != "" {
		return host
	}
	return cleanText(r.Profile.Name)
}

// get calls path with params and decodes the JSON body into out. On HTTP 429
// it waits once and retries before giving up with ErrRateLimited.
func (c *BraveClient) get(ctx context.Context, path string, params url.Values, out any) error {
	err := c.getOnce(ctx, path, params, out)
	if !errors.Is(err, ErrRateLimited) {
		return err
	}
	// select lets the wait end early if the request is cancelled.
	select {
	case <-time.After(c.retryDelay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return c.getOnce(ctx, path, params, out)
}

func (c *BraveClient) getOnce(ctx context.Context, path string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.apiKey)
	// Deliberately no Accept-Encoding header: when we leave it unset, Go asks
	// for gzip itself and transparently decompresses the response.

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call brave: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("brave returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode brave response: %w", err)
	}
	return nil
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// cleanText strips HTML tags (Brave wraps matches in <strong>), decodes
// entities like &amp;, and collapses runs of whitespace.
func cleanText(s string) string {
	s = html.UnescapeString(htmlTag.ReplaceAllString(s, ""))
	return strings.Join(strings.Fields(s), " ")
}

// WrapTransport puts the paid-search dispatch boundary around the client's transport.
func (c *BraveClient) WrapTransport(wrap func(http.RoundTripper) http.RoundTripper) {
	c.http.Transport = wrap(c.http.Transport)
}
