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
	CurateArticles(ctx context.Context, interests string) ([]string, error)
	CurateEvents(ctx context.Context, interests, location string, now time.Time) ([]string, error)
	ExtractEvents(ctx context.Context, interests, location string, results []sources.Result, now time.Time) ([]llm.Event, error)
}

// ProfileStore loads a member's interests and location. *store.Store
// satisfies it.
type ProfileStore interface {
	GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
}

// EmptyReason says why a curated widget has no items, when it's something
// the member can fix.
type EmptyReason string

// The reasons a curated widget can be empty.
const (
	EmptyNoInterests EmptyReason = "no_interests" // Articles needs interests
	EmptyNoLocation  EmptyReason = "no_location"  // Local Events needs a location
)

// CuratedWidget is the response body of GET /api/widgets/articles and GET
// /api/widgets/events. EmptyReason is "" unless the member has to fill
// something in first; a widget that found nothing new has no reason.
type CuratedWidget struct {
	// Widget is embedded, so its fields are flattened into this object,
	// as in SavedWidget.
	Widget
	EmptyReason EmptyReason `json:"emptyReason"`
}

// CuratedHandler serves the Articles and Local Events widgets. They pick
// their own searches from the member's interests and location, and are
// worked out fresh on every request: nothing is cached or saved.
type CuratedHandler struct {
	curator  Curator
	news     sources.Source
	events   sources.Source
	profiles ProfileStore
	logger   *slog.Logger
}

// NewCuratedHandler returns a CuratedHandler that plans with curator, reads
// profiles from profiles, and searches the news_search and events_search
// sources in srcs. It panics if either source is missing: that's a wiring
// mistake, and panicking catches it at startup.
func NewCuratedHandler(curator Curator, srcs SourceFinder, profiles ProfileStore, logger *slog.Logger) *CuratedHandler {
	return &CuratedHandler{
		curator:  curator,
		news:     mustGetSource(srcs, newsSourceName),
		events:   mustGetSource(srcs, eventsSourceName),
		profiles: profiles,
		logger:   logger,
	}
}

func mustGetSource(srcs SourceFinder, name string) sources.Source {
	src, ok := srcs.Get(name)
	if !ok {
		panic(fmt.Sprintf("widgets: no %q source registered", name))
	}
	return src
}

// Articles handles GET /api/widgets/articles: 3-5 recent news articles for
// the logged-in member's interests. The LLM writes one news query per
// interest (or, if it fails, the interests are split into queries), every
// query is searched at once, and each query's best result is shown. It must
// sit behind auth.RequireAuth.
func (h *CuratedHandler) Articles(w http.ResponseWriter, r *http.Request) {
	user, profile, ok := h.loadProfile(w, r)
	if !ok {
		return
	}
	widget := CuratedWidget{Widget: Widget{
		Title:     "Articles for you",
		Source:    newsSourceName,
		Archetype: llm.ArchetypeCards,
		Items:     []llm.Item{},
	}}

	// Without interests there is nothing to search for; asking the LLM
	// anyway would only spend credit.
	if profile.Interests == "" {
		widget.EmptyReason = EmptyNoInterests
		respond.JSON(w, http.StatusOK, widget)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestBudget)
	defer cancel()

	// 1. Plan the searches. The widget still works without the LLM.
	planCtx, cancelPlan := context.WithTimeout(ctx, planTimeout)
	queries, err := h.curator.CurateArticles(planCtx, profile.Interests)
	cancelPlan() // release the timer now rather than at the end of the request
	fromLLM := err == nil
	if err != nil {
		if writeIfCancelled(w, r, h.logger, err) {
			return
		}
		h.logger.WarnContext(ctx, "couldn't curate article queries; searching the interests as written", "err", err)
		queries = llm.FallbackArticleQueries(profile.Interests)
	}
	if len(queries) == 0 { // interests like "and, and" leave nothing to search
		widget.EmptyReason = EmptyNoInterests
		respond.JSON(w, http.StatusOK, widget)
		return
	}
	widget.Query = strings.Join(queries, querySeparator)

	// 2. Search, then 3. pick the best result per query.
	perQuery, ok := h.search(ctx, w, r, h.news, queries)
	if !ok {
		return
	}
	widget.Items = pickArticles(perQuery)

	h.logger.InfoContext(ctx, "articles curated",
		"user_id", user.ID,
		"queries", widget.Query,
		"from_llm", fromLLM,
		"results", countResults(perQuery),
		"items", len(widget.Items),
	)
	respond.JSON(w, http.StatusOK, widget)
}

// Events handles GET /api/widgets/events: up to 5 upcoming events near the
// logged-in member, matched to their interests when they have some. The LLM
// writes 2-3 location- and month-anchored web queries (with a fixed
// fallback, unless OpenRouter itself is turning calls away), every query is
// searched at once, and the LLM picks dated events out of the merged
// results. It must sit behind auth.RequireAuth.
func (h *CuratedHandler) Events(w http.ResponseWriter, r *http.Request) {
	user, profile, ok := h.loadProfile(w, r)
	if !ok {
		return
	}
	widget := CuratedWidget{Widget: Widget{
		Title:     "Local events",
		Source:    eventsSourceName,
		Archetype: llm.ArchetypeList,
		Items:     []llm.Item{},
	}}
	if profile.Location == "" {
		widget.EmptyReason = EmptyNoLocation
		respond.JSON(w, http.StatusOK, widget)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestBudget)
	defer cancel()
	now := time.Now()

	// 1. Plan the searches.
	planCtx, cancelPlan := context.WithTimeout(ctx, planTimeout)
	queries, err := h.curator.CurateEvents(planCtx, profile.Interests, profile.Location, now)
	cancelPlan()
	fromLLM := err == nil
	if err != nil {
		if writeIfCancelled(w, r, h.logger, err) {
			return
		}
		// OpenRouter turning the call away (a bad key, no credits, an
		// outage) means step 3 would fail the same way, so don't spend
		// searches on a card that can't be filled. This is a deliberate
		// exception to falling back when the LLM fails, to save credit.
		if openRouterRefused(err) {
			h.writeLLMError(w, r, "event planning", err)
			return
		}
		h.logger.WarnContext(ctx, "couldn't curate event queries; using general ones", "err", err)
		queries = llm.FallbackEventQueries(profile.Location, now)
	}
	widget.Query = strings.Join(queries, querySeparator)

	// 2. Search, and mix the queries' results together, taking one from
	// each query in turn, so every query is represented under the cap.
	perQuery, ok := h.search(ctx, w, r, h.events, queries)
	if !ok {
		return
	}
	results := newRoundRobin(perQuery).fill(nil, maxEventResults)

	// 3. Find the events. A list of web pages isn't a list of events, so
	// unlike the custom widgets there's no raw-results fallback.
	var events []llm.Event
	if len(results) > 0 {
		events, err = h.curator.ExtractEvents(ctx, profile.Interests, profile.Location, results, now)
		if err != nil {
			h.writeLLMError(w, r, "event extraction", err)
			return
		}
	}
	for _, e := range events {
		widget.Items = append(widget.Items, llm.Item{
			Title:     e.Name,
			URL:       e.URL,
			Snippet:   shorten(e.Description, maxSnippetRunes),
			Date:      e.Date,
			Label:     e.Venue,
			Thumbnail: e.Thumbnail,
		})
	}

	h.logger.InfoContext(ctx, "events curated",
		"user_id", user.ID,
		"queries", widget.Query,
		"from_llm", fromLLM,
		"results", len(results),
		"items", len(widget.Items),
	)
	respond.JSON(w, http.StatusOK, widget)
}

// loadProfile returns the logged-in user and their profile, trimmed. No
// profile yet reads as an empty one. On failure it writes the response and
// returns ok == false.
func (h *CuratedHandler) loadProfile(w http.ResponseWriter, r *http.Request) (user store.User, profile store.MemberProfile, ok bool) {
	ctx := r.Context()
	user, ok = auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return user, profile, false
	}
	profile, err := h.profiles.GetMemberProfile(ctx, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.logger.ErrorContext(ctx, "load profile failed", "user_id", user.ID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
		return user, profile, false
	}
	profile.Interests = strings.TrimSpace(profile.Interests)
	profile.Location = strings.TrimSpace(profile.Location)
	return user, profile, true
}

// search runs every query on src at once (see searchAll). A failed query is
// logged and skipped; if every one fails, search writes the error response
// and returns ok == false.
func (h *CuratedHandler) search(ctx context.Context, w http.ResponseWriter, r *http.Request, src sources.Source, queries []string) (perQuery [][]sources.Result, ok bool) {
	perQuery, errs := searchAll(ctx, src, queries)
	// If the client has gone, every search was cancelled: that's not worth a
	// warning each, and writeSearchError logs it once.
	clientGone := r.Context().Err() != nil
	failed := 0
	for i, err := range errs {
		if err != nil {
			failed++
			if !clientGone {
				h.logger.WarnContext(ctx, "search failed; skipping query", "source", src.Name(), "query", queries[i], "err", err)
			}
		}
	}
	if failed > 0 && failed == len(queries) {
		// errors.Join keeps every error, so errors.Is inside writeSearchError
		// still spots a rate limit among them.
		writeSearchError(w, r, h.logger, errors.Join(errs...))
		return nil, false
	}
	return perQuery, true
}

// searchAll searches src for every query at the same time and waits for all
// of them. results[i] and errs[i] belong to queries[i], whatever order the
// searches finish in.
func searchAll(ctx context.Context, src sources.Source, queries []string) (results [][]sources.Result, errs []error) {
	results = make([][]sources.Result, len(queries))
	errs = make([]error, len(queries))

	// A WaitGroup counts goroutines still running: Add(1) before starting
	// each one, Done when it finishes, and Wait blocks until none are left.
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		// "go" runs the function on a new goroutine and moves straight on to
		// the next query. Since Go 1.22 every loop iteration has its own i
		// and q, so each goroutine sees the values from its own iteration.
		go func() {
			defer wg.Done()
			// Each goroutine writes only its own slot of the slices, so no two
			// goroutines touch the same memory and no mutex is needed.
			results[i], errs[i] = src.Search(ctx, q)
		}()
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

// writeLLMError maps a failed LLM step of the Events widget (step, such as
// "event extraction", is for the log) to a response.
func (h *CuratedHandler) writeLLMError(w http.ResponseWriter, r *http.Request, step string, err error) {
	if writeIfCancelled(w, r, h.logger, err) {
		return
	}
	h.logger.ErrorContext(r.Context(), step+" failed", "err", err)
	var upstream *llm.UpstreamError
	if errors.As(err, &upstream) {
		// Hints about OpenRouter itself (its key, credits, rate limit) fit
		// here; others, such as "Try describing the widget differently",
		// were written for describe-a-widget, and the member described
		// nothing.
		hint := "Try refreshing in a moment."
		if upstream.Status != 0 {
			hint = upstream.Hint
		}
		respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", hint)
		return
	}
	respond.Error(w, http.StatusBadGateway, "Couldn't pick out events from the search results.", "Try again in a moment.")
}
