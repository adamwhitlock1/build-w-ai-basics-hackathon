package widgets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// The sources the curated widgets search.
const (
	newsSourceName   = "news_search"
	eventsSourceName = "events_search"
)

// Limits for the curated widgets.
const (
	minArticles = 3
	maxArticles = 5
	// maxEventResults caps the search results sent to the model to find
	// events in; more would make a slower, costlier prompt.
	maxEventResults = 20
	// maxSnippetRunes is about one line of a card.
	maxSnippetRunes = 160
	// querySeparator joins a widget's queries into its Query field.
	querySeparator = " · "
	// planTimeout caps the LLM call that plans a curated widget's searches,
	// well inside RequestBudget. Planning has a free fallback, so a slow
	// model shouldn't hold the card up for the LLM client's full 40 seconds.
	planTimeout = 20 * time.Second
)

// Curator is the LLM half of the curated widgets. *llm.Client satisfies it;
// tests use a fake.
type Curator interface {
	CurateArticles(ctx context.Context, interests string, aud sources.Audience) ([]string, error)
	CurateEvents(ctx context.Context, interests, location string, now time.Time, aud sources.Audience) ([]string, error)
	ExtractEvents(ctx context.Context, interests, location string, results []sources.Result, now time.Time, aud sources.Audience) ([]llm.Event, error)
}

// ProfileStore loads a member's interests and location. *store.Store
// satisfies it.
type ProfileStore interface {
	GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
}

// ResultStore remembers each member's latest curated widgets. *store.Store
// satisfies it.
type ResultStore interface {
	GetCuratedResult(ctx context.Context, userID int64, kind string) (store.CuratedResult, error)
	PutCuratedResult(ctx context.Context, r store.CuratedResult) error
}

// The kinds of curated widget, as remembered in the ResultStore.
const (
	kindArticles = "articles"
	kindEvents   = "events"
)

// EmptyReason says why a curated widget has no items, when it's something
// the member can fix.
type EmptyReason string

// The reasons a curated widget can be empty.
const (
	EmptyNoInterests EmptyReason = "no_interests" // Articles needs interests
	EmptyNoLocation  EmptyReason = "no_location"  // Local Events needs a location
)

// CuratedWidget is the response body of GET /api/widgets/articles and GET
// /api/widgets/events (and of their /api/members/{id}/widgets/...
// versions). EmptyReason is "" unless the member has to fill something in
// first; a widget that found nothing new has no reason.
type CuratedWidget struct {
	// Widget and Freshness are embedded, so their fields are flattened into
	// this object, as in SavedWidget.
	Widget
	Freshness
	EmptyReason EmptyReason `json:"emptyReason"`
}

// CuratedHandler serves the Articles and Local Events widgets, which a
// Curation works out. The member is the request's subject (see
// auth.SubjectFrom): the logged-in user on /api/widgets/..., or, on
// /api/members/{id}/widgets/..., the family member a parent is looking at.
type CuratedHandler struct {
	curation *Curation
	logger   *slog.Logger
}

// NewCuratedHandler returns a CuratedHandler whose Curation plans with
// curator, reads profiles from profiles, remembers results in results, and
// searches the news_search and events_search sources in srcs. It panics if
// either source is missing (see NewCuration).
func NewCuratedHandler(curator Curator, srcs SourceFinder, profiles ProfileStore, results ResultStore, logger *slog.Logger) *CuratedHandler {
	return &CuratedHandler{curation: NewCuration(curator, srcs, profiles, results, logger), logger: logger}
}

// articlesWidget is an empty Articles widget for aud.
func articlesWidget(aud sources.Audience) Widget {
	return Widget{
		Title:     "Articles for you",
		Source:    newsSourceName,
		Archetype: llm.ArchetypeCards,
		Items:     []llm.Item{},
		KidSafe:   aud == sources.Kids,
	}
}

// eventsWidget is an empty "Happening near you" widget for aud.
func eventsWidget(aud sources.Audience) Widget {
	return Widget{
		Title:     "Happening near you",
		Source:    eventsSourceName,
		Archetype: llm.ArchetypeList,
		Items:     []llm.Item{},
		KidSafe:   aud == sources.Kids,
	}
}

// articlesInputs is what a remembered Articles result is matched on: the
// member's interests.
func articlesInputs(p store.MemberProfile) string {
	return normalizeInput(p.Interests)
}

// eventsInputs is what a remembered "Happening near you" result is matched
// on. Events are picked for the location and the interests, so a change to
// either picks them again.
func eventsInputs(p store.MemberProfile) string {
	return normalizeInput(p.Interests) + "\n" + normalizeInput(p.Location)
}

func mustGetSource(srcs SourceFinder, name string) sources.Source {
	src, ok := srcs.Get(name)
	if !ok {
		panic(fmt.Sprintf("widgets: no %q source registered", name))
	}
	return src
}

// Articles handles GET /api/widgets/articles[?refresh=1] (and GET
// /api/members/{id}/widgets/articles): 3-5 recent news articles for the
// member's interests (see Curation.Articles). It must sit behind
// auth.RequireAuth.
func (h *CuratedHandler) Articles(w http.ResponseWriter, r *http.Request) {
	// h.curation.Articles is a method value: the method with its receiver
	// already bound, so it can be passed around like any other function.
	h.serve(w, r, h.curation.Articles)
}

// Events handles GET /api/widgets/events[?refresh=1] (and GET
// /api/members/{id}/widgets/events): up to 5 upcoming events near the
// member (see Curation.Events). It must sit behind auth.RequireAuth.
func (h *CuratedHandler) Events(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, h.curation.Events)
}

// serve answers a curated widget request with what curate, Curation.Articles
// or Curation.Events, works out for the request's subject.
func (h *CuratedHandler) serve(w http.ResponseWriter, r *http.Request, curate func(ctx context.Context, viewer, member store.User, opts CurationOptions) (CuratedWidget, error)) {
	viewer, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	member, _ := auth.SubjectFrom(r.Context()) // falls back to viewer, so always ok here
	widget, err := curate(r.Context(), viewer, member, CurationOptions{Refresh: wantsRefresh(r)})
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	respond.JSON(w, http.StatusOK, widget)
}

// normalizeInput puts a profile field in the form remembered widgets are
// matched on: lower case, with runs of spaces and line breaks made single
// spaces, so retyping the same interests doesn't count as a change.
func normalizeInput(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// searchAll searches src (filtered for aud) for every query at the same time and waits for all
// of them. results[i] and errs[i] belong to queries[i], whatever order the
// searches finish in.
func searchAll(ctx context.Context, src sources.Source, queries []string, aud sources.Audience) (results [][]sources.Result, errs []error) {
	results = make([][]sources.Result, len(queries))
	errs = make([]error, len(queries))

	// A WaitGroup counts goroutines still running, and Wait blocks until
	// none are left. wg.Go (Go 1.25) starts f on a new goroutine and does
	// the counting for us: it replaces the older wg.Add(1), go func() {
	// defer wg.Done(); ... }() pattern.
	var wg sync.WaitGroup
	for i, q := range queries {
		// wg.Go runs the function on a new goroutine and moves straight on to
		// the next query. Since Go 1.22 every loop iteration has its own i
		// and q, so each goroutine sees the values from its own iteration.
		wg.Go(func() {
			// Each goroutine writes only its own slot of the slices, so no two
			// goroutines touch the same memory and no mutex is needed.
			results[i], errs[i] = src.Search(ctx, q, aud)
		})
	}
	wg.Wait()
	return results, errs
}

// pickArticles chooses which articles to show: every query's best result,
// never the same URL twice, then (if that makes fewer than minArticles) the
// next-best results, one per query in turn, up to minArticles. It never
// returns more than maxArticles.
func pickArticles(perQuery [][]sources.Result) []llm.Item {
	rr := newRoundRobin(perQuery)
	picked := rr.round()
	if len(picked) > maxArticles {
		picked = picked[:maxArticles]
	}
	picked = rr.fill(picked, minArticles)

	items := make([]llm.Item, 0, len(picked))
	for _, r := range picked {
		items = append(items, llm.Item{
			Title:     r.Title,
			URL:       r.URL,
			Snippet:   shorten(r.Snippet, maxSnippetRunes),
			Date:      r.Date,
			Label:     r.Site,
			Thumbnail: r.Thumbnail,
		})
	}
	return items
}

// roundRobin hands out results from several lists in turn, best first,
// skipping any URL it has already handed out and any result without a
// title or URL.
type roundRobin struct {
	lists [][]sources.Result
	next  []int           // next[i] is the next result to look at in lists[i]
	seen  map[string]bool // URLs already handed out
}

func newRoundRobin(lists [][]sources.Result) *roundRobin {
	return &roundRobin{lists: lists, next: make([]int, len(lists)), seen: make(map[string]bool)}
}

// take returns the best result of list i not handed out yet. ok is false
// when list i has none left.
func (rr *roundRobin) take(i int) (r sources.Result, ok bool) {
	for rr.next[i] < len(rr.lists[i]) {
		r = rr.lists[i][rr.next[i]]
		rr.next[i]++
		if r.URL != "" && r.Title != "" && !rr.seen[r.URL] {
			rr.seen[r.URL] = true
			return r, true
		}
	}
	return sources.Result{}, false
}

// round takes one result from each list, in list order.
func (rr *roundRobin) round() []sources.Result {
	var out []sources.Result
	for i := range rr.lists {
		if r, ok := rr.take(i); ok {
			out = append(out, r)
		}
	}
	return out
}

// fill appends results to out, one from each list in turn, until out holds
// n results or every list is used up.
func (rr *roundRobin) fill(out []sources.Result, n int) []sources.Result {
	for len(out) < n {
		took := false
		for i := range rr.lists {
			if len(out) == n {
				break
			}
			if r, ok := rr.take(i); ok {
				out = append(out, r)
				took = true
			}
		}
		if !took {
			break
		}
	}
	return out
}

func countResults(perQuery [][]sources.Result) int {
	n := 0
	for _, results := range perQuery {
		n += len(results)
	}
	return n
}

// shorten returns s on one line, cut to at most limit characters at a word
// boundary and ending in "…" if it was too long.
func shorten(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	cut := string([]rune(s)[:limit-1]) // leave room for the "…"
	// Back up to the last space, unless that would drop most of the text
	// (one very long word).
	if i := strings.LastIndexByte(cut, ' '); i > len(cut)/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:-–—") + "…"
}

// openRouterRefused reports whether err says OpenRouter answered with an
// HTTP error status, as opposed to a slow or unusable answer.
func openRouterRefused(err error) bool {
	var upstream *llm.UpstreamError
	return errors.As(err, &upstream) && upstream.Status != 0
}
