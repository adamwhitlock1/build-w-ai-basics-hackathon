package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"kinlight/internal/calendar"
	"kinlight/internal/store"
)

// Agenda works out the Today card: a member's events for today and
// tomorrow, from the calendar link they connected. TodayHandler serves it
// over HTTP; anything else in Kinlight can call it with a context and a
// member.
//
// A day's events are remembered and given again without fetching; after
// TodayFreshFor they're given marked stale. A new day, time zone or link
// fetches again, and so does a refresh (see DayOptions).
//
// A calendar link is a secret. It's read from the store, handed to the
// fetcher, and hashed into the remembered result's inputs; it is never
// returned or logged.
type Agenda struct {
	fetcher CalendarFetcher
	store   CalendarStore
	logger  *slog.Logger
}

// NewAgenda returns an Agenda that fetches calendars with fetcher and keeps
// links and remembered events in st.
func NewAgenda(fetcher CalendarFetcher, st CalendarStore, logger *slog.Logger) *Agenda {
	return &Agenda{fetcher: fetcher, store: st, logger: logger}
}

// DayOptions are the choices a caller of Agenda.Day can make.
type DayOptions struct {
	// Refresh fetches the calendar again even when the day's events are
	// remembered, unless they were fetched under a minute ago (see
	// refreshGuard).
	Refresh bool
}

// Day returns member's events for today and tomorrow as calendar days in
// loc. No calendar, or a link that stopped working, is a TodayWidget with an
// EmptyReason, not an error. A failed fetch is a *StepError for
// StepCalendar.
func (a *Agenda) Day(ctx context.Context, member store.User, loc *time.Location, opts DayOptions) (TodayWidget, error) {
	now := timeNow()
	day := todayDay{Date: now.In(loc).Format(time.DateOnly), TimeZone: loc.String(), Today: []TodayEvent{}, Tomorrow: []TodayEvent{}}

	link, err := a.store.GetCalendarLink(ctx, member.ID)
	if err != nil {
		if !clientGone(ctx, err) {
			a.logger.ErrorContext(ctx, "load calendar link failed", "err", err)
		}
		return TodayWidget{}, fmt.Errorf("load calendar link: %w", err)
	}
	if link == "" {
		day.EmptyReason = EmptyNoCalendar
		return TodayWidget{todayDay: day, Freshness: justUpdated(now)}, nil
	}
	day.Provider = calendar.Provider(link)
	inputs := linkHash(link) + "\n" + day.TimeZone + "\n" + day.Date
	if remembered, ok := a.remembered(ctx, member.ID, inputs, opts.Refresh, now); ok {
		return remembered, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, todayBudget)
	defer cancel()
	feed, err := a.fetcher.Fetch(fetchCtx, link)
	if errors.Is(err, calendar.ErrLinkGone) {
		// The link was revoked or deleted: say so, and forget its events.
		a.logger.InfoContext(fetchCtx, "calendar link stopped working", "user_id", member.ID, "provider", day.Provider)
		if err := a.store.DeleteCuratedResult(context.WithoutCancel(fetchCtx), member.ID, kindToday); err != nil {
			a.logger.WarnContext(fetchCtx, "couldn't forget the remembered events", "user_id", member.ID, "err", err)
		}
		day.EmptyReason = EmptyCalendarGone
		return TodayWidget{todayDay: day, Freshness: justUpdated(now)}, nil
	}
	if err != nil {
		return TodayWidget{}, stepError(StepCalendar, err, "user_id", member.ID, "provider", day.Provider)
	}

	// Today and tomorrow, as calendar days in the member's time zone.
	y, m, d := now.In(loc).Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	events, err := calendar.Between(feed, todayStart, tomorrowStart.AddDate(0, 0, 1), loc)
	if err != nil {
		return TodayWidget{}, stepError(StepCalendar, err, "user_id", member.ID, "provider", day.Provider)
	}
	day.Today, day.MoreToday = eventsOn(events, todayStart, tomorrowStart)
	day.Tomorrow, day.MoreTomorrow = eventsOn(events, tomorrowStart, tomorrowStart.AddDate(0, 0, 1))

	a.remember(ctx, member.ID, inputs, day, now)
	a.logger.InfoContext(fetchCtx, "today's events fetched", "user_id", member.ID, "provider", day.Provider, "today", len(day.Today)+day.MoreToday, "tomorrow", len(day.Tomorrow)+day.MoreTomorrow)
	return TodayWidget{todayDay: day, Freshness: justUpdated(now)}, nil
}

// remembered returns the member's remembered events, and true, when they
// were fetched for inputs (this link, time zone and day) and the caller
// doesn't need new ones. As for the curated cards, a refresh needs new ones
// unless the remembered ones are under a minute old.
func (a *Agenda) remembered(ctx context.Context, userID int64, inputs string, refresh bool, now time.Time) (TodayWidget, bool) {
	remembered, err := a.store.GetCuratedResult(ctx, userID, kindToday)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return TodayWidget{}, false
	case err != nil:
		a.logger.WarnContext(ctx, "couldn't load the remembered events; fetching again", "user_id", userID, "err", err)
		return TodayWidget{}, false
	case remembered.Inputs != inputs:
		return TodayWidget{}, false
	case refresh && !recentlyUpdated(remembered.UpdatedAt, now):
		return TodayWidget{}, false
	}
	var day todayDay
	if err := json.Unmarshal(remembered.Payload, &day); err != nil {
		a.logger.WarnContext(ctx, "couldn't read the remembered events; fetching again", "user_id", userID, "err", err)
		return TodayWidget{}, false
	}
	if day.Today == nil {
		day.Today = []TodayEvent{}
	}
	if day.Tomorrow == nil {
		day.Tomorrow = []TodayEvent{}
	}
	return TodayWidget{todayDay: day, Freshness: freshness(remembered.UpdatedAt, now, TodayFreshFor)}, true
}

// remember saves day as the member's remembered events. Like the other
// cards' remember, it outlives the caller's cancellation, and a failure is
// only logged.
func (a *Agenda) remember(ctx context.Context, userID int64, inputs string, day todayDay, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	payload, err := json.Marshal(day)
	if err == nil {
		err = a.store.PutCuratedResult(ctx, store.CuratedResult{UserID: userID, Kind: kindToday, Inputs: inputs, Payload: payload, UpdatedAt: now})
	}
	if err != nil {
		a.logger.WarnContext(ctx, "couldn't remember today's events", "user_id", userID, "err", err)
	}
}
