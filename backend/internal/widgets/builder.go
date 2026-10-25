package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// WidgetBuilder builds described widgets: it turns a sentence into a live
// widget and saves it for a member, and refreshes saved ones. Handler serves
// it over HTTP; anything else in Porchlight can call it with a context and a
// member.
//
// Each call works within RequestBudget, and once the AI calls and searches
// are paid for, the save goes ahead even if the caller has gone.
type WidgetBuilder struct {
	planner  Planner
	sources  SourceFinder
	saved    WidgetStore
	profiles ProfileStore
	layouts  Layouts
	logger   *slog.Logger
}

// NewWidgetBuilder returns a WidgetBuilder that builds widgets with planner
// and srcs (telling the planner where the member lives, from profiles),
// keeps them in saved, and adds new ones to the member's layout.
func NewWidgetBuilder(planner Planner, srcs SourceFinder, saved WidgetStore, profiles ProfileStore, layouts Layouts, logger *slog.Logger) *WidgetBuilder {
	return &WidgetBuilder{planner: planner, sources: srcs, saved: saved, profiles: profiles, layouts: layouts, logger: logger}
}

// Build asks the LLM for a widget spec for prompt, searches the chosen
// source, shapes the results into items, and saves the widget for member at
// the end of their saved layout. prompt is a trimmed sentence of at most
// maxPromptRunes characters; Handler.FromPrompt checks that.
//
// A failed plan or search is a *StepError for StepPlanning or StepSearch.
// A plan naming no known source wraps ErrUnknownSource, and a widget that
// was built but couldn't be saved wraps ErrNotSaved.
func (b *WidgetBuilder) Build(ctx context.Context, member store.User, prompt string) (SavedWidget, error) {
	ctx = attribute(ctx, FeatureDescribeWidget, member, member) // a member describes widgets only for their own page
	// One deadline for the whole pipeline, so an HTTP reply is always
	// written before the server's WriteTimeout would cut the connection.
	ctx, cancel := context.WithTimeout(ctx, RequestBudget)
	defer cancel()
	// The widget goes on the member's own page, so they decide what's
	// suitable, and "weather" or "near me" means where they live.
	aud := audienceFor(member)
	location := b.savedLocation(ctx, member.ID)

	// 1. Plan: which source, what query, which archetype.
	spec, err := b.planner.PickWidgetSpec(ctx, prompt, b.sources, aud, location)
	if err != nil {
		return SavedWidget{}, &StepError{Step: StepPlanning, Err: err}
	}
	src, ok := b.sources.Get(spec.Source)
	if !ok { // the planner validates this, but don't trust a dependency blindly
		b.logger.WarnContext(ctx, "planner chose an unknown source", "source", spec.Source)
		return SavedWidget{}, fmt.Errorf("%w %q", ErrUnknownSource, spec.Source)
	}
	b.logger.InfoContext(ctx, "widget planned", "source", spec.Source, "query", spec.Query, "archetype", spec.Archetype, "kid_safe", aud == sources.Kids)

	// 2. Search.
	results, err := src.Search(ctx, spec.Query, aud)
	if err != nil {
		return SavedWidget{}, &StepError{Step: StepSearch, Err: err}
	}

	// 3. Shape results into items, falling back to the raw results.
	items := []llm.Item{}
	if len(results) > 0 {
		shaped, err := b.planner.ShapeItems(ctx, prompt, spec, results, aud)
		if err != nil {
			b.logger.WarnContext(ctx, "shaping failed; using raw search results", "err", err)
			items = itemsFromResults(results)
		} else {
			items = shaped
		}
	}

	// 4. Save it for the member.
	saved, err := b.save(ctx, member.ID, prompt, Widget{
		Title:     spec.Title,
		Source:    spec.Source,
		Query:     spec.Query,
		Archetype: spec.Archetype,
		Items:     items,
		KidSafe:   aud == sources.Kids,
	})
	if err != nil {
		b.logger.ErrorContext(ctx, "save widget", "user_id", member.ID, "err", err)
		return SavedWidget{}, fmt.Errorf("%w: %w", ErrNotSaved, err)
	}
	return saved, nil
}

// savedLocation returns the location in userID's profile, trimmed, or "" if
// they have none. Most widgets don't need a place, so a failed read is
// logged and treated as no location rather than failing the build.
func (b *WidgetBuilder) savedLocation(ctx context.Context, userID int64) string {
	profile, err := b.profiles.GetMemberProfile(ctx, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ""
	case err != nil:
		b.logger.WarnContext(ctx, "couldn't load the profile; planning without a location", "user_id", userID, "err", err)
		return ""
	}
	return strings.TrimSpace(profile.Location)
}

// save stores widget, described as prompt, for userID and puts it at the
// end of their saved layout. The prompt is kept so a refresh can shape new
// results the same way.
//
// By now the LLM and search calls are done and paid for, so the save uses
// neither the pipeline's deadline (which may have just run out) nor the
// caller's cancellation (the browser tab may have closed): the widget will
// be on the page next time. context.WithoutCancel keeps ctx's values but
// drops its cancellation and deadline, and saveTimeout bounds the writes
// instead.
func (b *WidgetBuilder) save(ctx context.Context, userID int64, prompt string, widget Widget) (SavedWidget, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()

	spec, err := json.Marshal(renderSpec{Title: widget.Title, Archetype: widget.Archetype, Items: widget.Items, KidSafe: widget.KidSafe})
	if err != nil {
		return SavedWidget{}, fmt.Errorf("encode render spec: %w", err)
	}
	cw, err := b.saved.CreateCustomWidget(ctx, store.CustomWidget{
		UserID:     userID,
		Name:       widget.Title,
		Source:     widget.Source,
		Query:      widget.Query,
		Prompt:     prompt,
		RenderSpec: spec,
	})
	if err != nil {
		return SavedWidget{}, err
	}

	// If this fails the widget is still saved, and GET /api/me/homepage adds
	// a card for any widget the layout is missing, so it's not worth failing
	// the build over: log it and carry on.
	if err := b.layouts.AddCustomWidget(ctx, userID, cw.ID); err != nil {
		b.logger.WarnContext(ctx, "couldn't add the new widget to the layout", "user_id", userID, "widget_id", cw.ID, "err", err)
	}
	return SavedWidget{ID: cw.ID, Widget: widget, CreatedAt: cw.CreatedAt.UTC(), Freshness: justUpdated(cw.CreatedAt)}, nil
}

// Refresh gets new items for member's saved widget widgetID by running its
// saved search again and shaping the results for the sentence it was
// described with, keeping its title and archetype.
//
// The saved items are only replaced by a successful refresh: a failed
// search leaves them as they were (and is a *StepError for StepSearch, as
// in Build), and a search that finds nothing at all returns them unchanged.
// A widget refreshed under a minute ago is returned as it is, so a double
// click or a second tab doesn't pay twice. Another member's widget wraps
// ErrNoSuchWidget, like a missing one, and one whose source is no longer
// registered wraps ErrSourceGone.
func (b *WidgetBuilder) Refresh(ctx context.Context, member store.User, widgetID int64) (SavedWidget, error) {
	ctx = attribute(ctx, FeatureRefreshWidget, member, member)
	cw, err := b.saved.GetCustomWidget(ctx, member.ID, widgetID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return SavedWidget{}, fmt.Errorf("widget %d: %w", widgetID, ErrNoSuchWidget)
	case err != nil:
		return SavedWidget{}, b.failed(ctx, "load widget", err)
	}
	now := timeNow()
	current, err := FromStore(cw, now)
	if err != nil {
		return SavedWidget{}, b.failed(ctx, "load widget", err)
	}
	if recentlyUpdated(current.UpdatedAt, now) {
		return current, nil
	}
	src, ok := b.sources.Get(cw.Source)
	if !ok { // saved before a source was renamed or removed
		b.logger.WarnContext(ctx, "saved widget has an unknown source", "widget_id", widgetID, "source", cw.Source)
		return SavedWidget{}, fmt.Errorf("%w: %q", ErrSourceGone, cw.Source)
	}

	ctx, cancel := context.WithTimeout(ctx, RequestBudget)
	defer cancel()
	// Only the owner refreshes their widget, so they decide what's suitable
	// (as in Build). A kid's widget saved before kid-safe picks becomes
	// kid-safe here.
	aud := audienceFor(member)

	// 1. Search again with the saved query.
	results, err := src.Search(ctx, cw.Query, aud)
	if err != nil {
		return SavedWidget{}, &StepError{Step: StepSearch, Err: err}
	}
	if len(results) == 0 {
		// More likely a hiccup than the topic vanishing: keep what's there.
		b.logger.InfoContext(ctx, "refresh found nothing; keeping the saved items", "widget_id", widgetID, "query", cw.Query)
		return current, nil
	}

	// 2. Shape the results the way the widget was first built. Widgets saved
	// before prompts were kept use their title, which names what they show.
	prompt := cw.Prompt
	if prompt == "" {
		prompt = current.Title
	}
	spec := llm.WidgetSpec{Title: current.Title, Source: cw.Source, Query: cw.Query, Archetype: current.Archetype}
	items, err := b.planner.ShapeItems(ctx, prompt, spec, results, aud)
	if err != nil {
		b.logger.WarnContext(ctx, "shaping failed; using raw search results", "widget_id", widgetID, "err", err)
		items = itemsFromResults(results)
	}

	// 3. Save, past the caller's cancellation and the deadline, like save
	// does: the calls are paid for.
	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancelSave()
	renderJSON, err := json.Marshal(renderSpec{Title: current.Title, Archetype: current.Archetype, Items: items, KidSafe: aud == sources.Kids})
	if err != nil {
		return SavedWidget{}, b.failed(ctx, "encode refreshed widget", err)
	}
	updated, err := b.saved.UpdateCustomWidgetItems(saveCtx, member.ID, widgetID, renderJSON, now)
	switch {
	case errors.Is(err, store.ErrNotFound): // deleted while it was refreshing
		return SavedWidget{}, fmt.Errorf("widget %d: %w", widgetID, ErrNoSuchWidget)
	case err != nil:
		return SavedWidget{}, b.failed(ctx, "save refreshed widget", err)
	}
	refreshed, err := FromStore(updated, now)
	if err != nil {
		return SavedWidget{}, b.failed(ctx, "load widget", err)
	}
	b.logger.InfoContext(ctx, "widget refreshed", "widget_id", widgetID, "user_id", member.ID, "kid_safe", refreshed.KidSafe, "items", len(refreshed.Items))
	return refreshed, nil
}

// failed logs a failure on our side, such as a database error, and returns
// it. what says what we were doing, e.g. "load widget". A request the
// browser gave up on isn't logged.
func (b *WidgetBuilder) failed(ctx context.Context, what string, err error) error {
	if !clientGone(ctx, err) {
		b.logger.ErrorContext(ctx, what+" failed", "err", err)
	}
	return fmt.Errorf("%s: %w", what, err)
}
