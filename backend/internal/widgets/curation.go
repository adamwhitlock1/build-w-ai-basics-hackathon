package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
)

// Curation works out the Articles and Local Events widgets, which pick
// their own searches from a member's interests and location. CuratedHandler
// serves it over HTTP; anything else in Kinlight can call it with a
// context and a member.
//
// Each member's latest result is remembered and given again, without
// searching, until the member's interests or location change or the caller
// asks for a refresh (see CurationOptions). A remembered result past its
// freshness window is still given, marked stale.
type Curation struct {
	curator  Curator
	news     sources.Source
	events   sources.Source
	profiles ProfileStore
	results  ResultStore
	logger   *slog.Logger
}

// NewCuration returns a Curation that plans with curator, reads profiles
// from profiles, remembers results in results, and searches the news_search
// and events_search sources in srcs. It panics if either source is missing:
// that's a wiring mistake, and panicking catches it at startup.
func NewCuration(curator Curator, srcs SourceFinder, profiles ProfileStore, results ResultStore, logger *slog.Logger) *Curation {
	return &Curation{
		curator:  curator,
		news:     mustGetSource(srcs, newsSourceName),
		events:   mustGetSource(srcs, eventsSourceName),
		profiles: profiles,
		results:  results,
		logger:   logger,
	}
}

// CurationOptions are the choices a caller of Curation's methods can make.
type CurationOptions struct {
	// Refresh works the widget out again even when a result is remembered,
	// unless it was worked out under a minute ago (see refreshGuard).
	Refresh bool
}

// Articles returns 3-5 recent news articles for member's interests. The LLM
// writes one news query per interest (or, if it fails, the interests are
// split into queries), every query is searched at once, and each query's
// best result is shown. viewer is who asked: member, or a parent looking at
// member's page.
//
// No interests is a CuratedWidget with an EmptyReason, not an error. When
// every search fails, the error is a *StepError for StepSearch.
func (c *Curation) Articles(ctx context.Context, viewer, member store.User, opts CurationOptions) (CuratedWidget, error) {
	ctx = attribute(ctx, FeatureArticles, viewer, member)
	profile, err := loadProfile(ctx, c.profiles, c.logger, viewer, member)
	if err != nil {
		return CuratedWidget{}, err
	}
	// The member whose card this is decides what's suitable, even when a
	// parent is the one looking.
	aud := audienceFor(member)
	now := timeNow()
	widget := CuratedWidget{Widget: articlesWidget(aud), Freshness: justUpdated(now)}

	// Without interests there is nothing to search for; asking the LLM
	// anyway would only spend credit.
	if profile.Interests == "" {
		widget.EmptyReason = EmptyNoInterests
		return widget, nil
	}

	// Articles only depend on the interests, so a new location keeps them.
	inputs := articlesInputs(profile)
	if remembered, ok := c.remembered(ctx, member.ID, kindArticles, inputs, aud, ArticlesFreshFor, opts.Refresh, now); ok {
		return remembered, nil
	}

	// work bounds the AI calls and searches. ctx is still the caller's own,
	// and says whether they've gone.
	work, cancel := context.WithTimeout(ctx, RequestBudget)
	defer cancel()

	// 1. Plan the searches. The widget still works without the LLM.
	planCtx, cancelPlan := context.WithTimeout(work, planTimeout)
	queries, err := c.curator.CurateArticles(planCtx, profile.Interests, aud)
	cancelPlan() // release the timer now rather than at the end of the call
	fromLLM := err == nil
	if err != nil {
		if ctx.Err() != nil { // the caller has gone: don't search for nobody
			return CuratedWidget{}, &StepError{Step: StepArticlePlanning, Err: err}
		}
		c.logger.WarnContext(work, "couldn't curate article queries; searching the interests as written", "err", err)
		queries = llm.FallbackArticleQueries(profile.Interests)
	}
	if len(queries) == 0 { // interests like "and, and" leave nothing to search
		widget.EmptyReason = EmptyNoInterests
		return widget, nil
	}
	widget.Query = strings.Join(queries, querySeparator)

	// 2. Search, then 3. pick the best result per query.
	perQuery, err := c.search(work, ctx, c.news, queries, aud)
	if err != nil {
		return CuratedWidget{}, err
	}
	widget.Items = pickArticles(perQuery)
	c.remember(ctx, member.ID, kindArticles, inputs, widget.Widget, now)

	c.logger.InfoContext(work, "articles curated",
		"user_id", member.ID, // whose card this is
		"viewer_id", viewer.ID, // who asked: a parent, if not the member
		"queries", widget.Query,
		"from_llm", fromLLM,
		"kid_safe", widget.KidSafe,
		"results", countResults(perQuery),
		"items", len(widget.Items),
	)
	return widget, nil
}

// Events returns up to 5 upcoming events near member, matched to their
// interests when they have some. The LLM writes 2-3 location- and
// month-anchored web queries (with a fixed fallback, unless OpenRouter
// itself is turning calls away), every query is searched at once, and the
// LLM picks dated events out of the merged results. viewer is as for
// Articles.
//
// No location is a CuratedWidget with an EmptyReason, not an error. A failed
// AI step is a *StepError for StepEventPlanning or StepEventExtraction, and
// failed searches one for StepSearch.
func (c *Curation) Events(ctx context.Context, viewer, member store.User, opts CurationOptions) (CuratedWidget, error) {
	ctx = attribute(ctx, FeatureEvents, viewer, member)
	profile, err := loadProfile(ctx, c.profiles, c.logger, viewer, member)
	if err != nil {
		return CuratedWidget{}, err
	}
	aud := audienceFor(member) // as in Articles
	now := timeNow()
	widget := CuratedWidget{Widget: eventsWidget(aud), Freshness: justUpdated(now)}
	if profile.Location == "" {
		widget.EmptyReason = EmptyNoLocation
		return widget, nil
	}

	inputs := eventsInputs(profile)
	if remembered, ok := c.remembered(ctx, member.ID, kindEvents, inputs, aud, EventsFreshFor, opts.Refresh, now); ok {
		return remembered, nil
	}

	work, cancel := context.WithTimeout(ctx, RequestBudget) // as in Articles
	defer cancel()

	// 1. Plan the searches.
	planCtx, cancelPlan := context.WithTimeout(work, planTimeout)
	queries, err := c.curator.CurateEvents(planCtx, profile.Interests, profile.Location, now, aud)
	cancelPlan()
	fromLLM := err == nil
	if err != nil {
		// OpenRouter turning the call away (a bad key, no credits, an
		// outage) means step 3 would fail the same way, so don't spend
		// searches on a card that can't be filled. This is a deliberate
		// exception to falling back when the LLM fails, to save credit.
		if ctx.Err() != nil || openRouterRefused(err) {
			return CuratedWidget{}, &StepError{Step: StepEventPlanning, Err: err}
		}
		c.logger.WarnContext(work, "couldn't curate event queries; using general ones", "err", err)
		queries = llm.FallbackEventQueries(profile.Location, now)
	}
	widget.Query = strings.Join(queries, querySeparator)

	// 2. Search, and mix the queries' results together, taking one from
	// each query in turn, so every query is represented under the cap.
	perQuery, err := c.search(work, ctx, c.events, queries, aud)
	if err != nil {
		return CuratedWidget{}, err
	}
	results := newRoundRobin(perQuery).fill(nil, maxEventResults)

	// 3. Find the events. A list of web pages isn't a list of events, so
	// unlike the custom widgets there's no raw-results fallback.
	var events []llm.Event
	if len(results) > 0 {
		events, err = c.curator.ExtractEvents(work, profile.Interests, profile.Location, results, now, aud)
		if err != nil {
			return CuratedWidget{}, &StepError{Step: StepEventExtraction, Err: err}
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
	c.remember(ctx, member.ID, kindEvents, inputs, widget.Widget, now)

	c.logger.InfoContext(work, "events curated",
		"user_id", member.ID,
		"viewer_id", viewer.ID,
		"queries", widget.Query,
		"from_llm", fromLLM,
		"kid_safe", widget.KidSafe,
		"results", len(results),
		"items", len(widget.Items),
	)
	return widget, nil
}

// loadProfile returns member's profile, trimmed. No profile yet reads as an
// empty one. viewer, who asked for member's card, is for the log line of a
// failure. The curated widgets and the briefing share it.
func loadProfile(ctx context.Context, profiles ProfileStore, logger *slog.Logger, viewer, member store.User) (store.MemberProfile, error) {
	profile, err := profiles.GetMemberProfile(ctx, member.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		if !clientGone(ctx, err) {
			logger.ErrorContext(ctx, "load profile failed", "user_id", member.ID, "viewer_id", viewer.ID, "err", err)
		}
		return store.MemberProfile{}, fmt.Errorf("load profile of user %d: %w", member.ID, err)
	}
	profile.Interests = strings.TrimSpace(profile.Interests)
	profile.Location = strings.TrimSpace(profile.Location)
	return profile, nil
}

// remembered returns userID's remembered widget of kind, as of now, and
// true, when there is one picked for inputs and aud and the caller doesn't
// need a new one. A refresh needs a new one, unless the remembered one is
// under a minute old (see refreshGuard). If nothing usable is remembered it
// returns false, and the caller works the widget out.
func (c *Curation) remembered(ctx context.Context, userID int64, kind, inputs string, aud sources.Audience, window time.Duration, refresh bool, now time.Time) (CuratedWidget, bool) {
	remembered, err := c.results.GetCuratedResult(ctx, userID, kind)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return CuratedWidget{}, false
	case err != nil:
		// Not worth failing the card over: working it out again still works.
		c.logger.WarnContext(ctx, "couldn't load the remembered widget; picking again", "kind", kind, "user_id", userID, "err", err)
		return CuratedWidget{}, false
	case remembered.Inputs != inputs: // the member's profile has changed since
		return CuratedWidget{}, false
	case refresh && !recentlyUpdated(remembered.UpdatedAt, now):
		return CuratedWidget{}, false
	}

	var widget Widget
	if err := json.Unmarshal(remembered.Payload, &widget); err != nil {
		c.logger.WarnContext(ctx, "couldn't read the remembered widget; picking again", "kind", kind, "user_id", userID, "err", err)
		return CuratedWidget{}, false
	}
	// A kid must never be sent a result picked without kid-safe filtering,
	// such as one remembered before kid-safe picks existed. (Results picked
	// for a kid are filtered more than a parent needs, so they're picked
	// again too.)
	if widget.KidSafe != (aud == sources.Kids) {
		return CuratedWidget{}, false
	}
	if widget.Items == nil {
		widget.Items = []llm.Item{}
	}
	return CuratedWidget{Widget: widget, Freshness: freshness(remembered.UpdatedAt, now, window)}, true
}

// remember saves widget, worked out at now from inputs, as userID's latest
// widget of kind. The searches are already paid for, so like saving a
// described widget it outlives the caller's cancellation. A failure is only
// logged: the member still gets their widget, and it's worked out again
// next time.
func (c *Curation) remember(ctx context.Context, userID int64, kind, inputs string, widget Widget, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	payload, err := json.Marshal(widget)
	if err == nil {
		err = c.results.PutCuratedResult(ctx, store.CuratedResult{
			UserID: userID, Kind: kind, Inputs: inputs, Payload: payload, UpdatedAt: now,
		})
	}
	if err != nil {
		c.logger.WarnContext(ctx, "couldn't remember the widget", "kind", kind, "user_id", userID, "err", err)
	}
}

// search runs every query on src at once, within ctx and filtered for aud
// (see searchAll). caller is the caller's own context, which tells a caller
// who has gone from a search that ran out of time. A failed query is logged
// and skipped; if every one fails, the error is a *StepError for
// StepSearch.
func (c *Curation) search(ctx, caller context.Context, src sources.Source, queries []string, aud sources.Audience) ([][]sources.Result, error) {
	perQuery, errs := searchAll(ctx, src, queries, aud)
	// If the caller has gone, every search was cancelled: that's not worth
	// a warning each, and writeServiceError logs it once.
	callerGone := caller.Err() != nil
	failed := 0
	for i, err := range errs {
		if err != nil {
			failed++
			if !callerGone {
				c.logger.WarnContext(ctx, "search failed; skipping query", "source", src.Name(), "query", queries[i], "err", err)
			}
		}
	}
	if failed > 0 && failed == len(queries) {
		// errors.Join keeps every error, so errors.Is inside writeSearchError
		// still spots a rate limit among them.
		return nil, &StepError{Step: StepSearch, Err: errors.Join(errs...)}
	}
	return perQuery, nil
}
