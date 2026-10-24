package widgets

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// Briefings writes the morning briefing: a few sentences the AI writes
// about a member's day from what their other cards already hold, plus the
// day's forecast. BriefingHandler serves it over HTTP; anything else in
// Porchlight can call it with a context and a member.
//
// It writes one on the member's first visit of the day, then gives it again
// until the facts behind it change, writing at most maxBriefingWrites a
// day, at least briefingRewriteGap apart. It never runs a paid search.
type Briefings struct {
	briefer  Briefer
	weather  sources.Source
	profiles ProfileStore
	results  ResultStore
	locks    memberLocks
	logger   *slog.Logger
	now      func() time.Time // timeNow; tests set another clock
}

// NewBriefings returns a Briefings that writes with briefer, asks for
// forecasts from the weather_forecast source in srcs, reads profiles from
// profiles, and reads and remembers results in results. It panics if the
// weather source is missing, as NewCuration does.
func NewBriefings(briefer Briefer, srcs SourceFinder, profiles ProfileStore, results ResultStore, logger *slog.Logger) *Briefings {
	return &Briefings{
		briefer:  briefer,
		weather:  mustGetSource(srcs, sources.WeatherForecast),
		profiles: profiles,
		results:  results,
		logger:   logger,
		now:      timeNow,
	}
}

// Today returns member's briefing for today in loc. viewer is who asked:
// member, or a parent looking at member's page. Nothing to write about is a
// BriefingWidget with an EmptyReason, not an error. When the AI can't write
// one, the briefing from earlier today is given instead if there is one;
// otherwise the error is a *StepError for StepBriefing.
func (b *Briefings) Today(ctx context.Context, viewer, member store.User, loc *time.Location) (BriefingWidget, error) {
	ctx = attribute(ctx, FeatureBriefing, viewer, member)
	aud := audienceFor(member)
	now := b.now()
	date := now.In(loc).Format(time.DateOnly)

	profile, err := loadProfile(ctx, b.profiles, b.logger, viewer, member)
	if err != nil {
		return BriefingWidget{}, err
	}

	// One briefing at a time per member, so two tabs (or a parent and the
	// member) opening at once pay for one AI call, not two.
	unlock := b.locks.lock(member.ID)
	defer unlock()

	facts := b.gatherFacts(ctx, member.ID, profile, aud, loc, now)
	widget := BriefingWidget{Freshness: justUpdated(now), Date: date, KidSafe: aud == sources.Kids, Uses: []string{}}
	if len(facts.uses) == 0 {
		widget.EmptyReason = EmptyNothingYet
		return widget, nil
	}

	today := b.rememberedToday(ctx, member.ID, date, loc.String(), aud)
	if !shouldWrite(today, facts.fingerprint, now) { // so today isn't nil
		b.logger.InfoContext(ctx, "briefing remembered", "user_id", member.ID, "viewer_id", viewer.ID, "writes", today.day.Writes)
		return today.widget(), nil
	}

	writeCtx, cancel := context.WithTimeout(ctx, briefingBudget)
	defer cancel()
	text, err := b.briefer.WriteBriefing(writeCtx, facts.prompt, aud)
	if err != nil {
		failed := stepError(StepBriefing, err, "user_id", member.ID)
		if ctx.Err() != nil { // the caller has gone: nobody to send one to
			return BriefingWidget{}, failed
		}
		if today != nil {
			// The briefing from earlier today still says something true.
			b.logger.WarnContext(ctx, "couldn't rewrite the briefing; sending the earlier one", "user_id", member.ID, "err", err)
			return today.widget(), nil
		}
		return BriefingWidget{}, failed
	}

	day := briefingDay{Text: text, Date: date, TimeZone: loc.String(), KidSafe: aud == sources.Kids, Uses: facts.uses, Writes: 1}
	if today != nil {
		day.Writes = today.day.Writes + 1
	}
	b.remember(ctx, member.ID, day, facts.fingerprint, now)
	b.logger.InfoContext(ctx, "briefing written",
		"user_id", member.ID,
		"viewer_id", viewer.ID,
		"uses", strings.Join(facts.uses, ","),
		"writes", day.Writes,
		"kid_safe", day.KidSafe,
	)
	return rememberedBriefing{day: day, writtenAt: now}.widget(), nil
}

// rememberedToday returns the member's remembered briefing if it was
// written for date in timeZone, for aud; otherwise nil. A briefing written
// for the other audience counts as missing, so a kid is never sent one
// written without the kid guidance.
func (b *Briefings) rememberedToday(ctx context.Context, userID int64, date, timeZone string, aud sources.Audience) *rememberedBriefing {
	r, err := b.results.GetCuratedResult(ctx, userID, kindBriefing)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			b.logger.WarnContext(ctx, "couldn't load the remembered briefing; writing again", "user_id", userID, "err", err)
		}
		return nil
	}
	var day briefingDay
	if err := json.Unmarshal(r.Payload, &day); err != nil {
		b.logger.WarnContext(ctx, "couldn't read the remembered briefing; writing again", "user_id", userID, "err", err)
		return nil
	}
	if day.Date != date || day.TimeZone != timeZone || day.KidSafe != (aud == sources.Kids) || day.Text == "" {
		return nil
	}
	return &rememberedBriefing{day: day, fingerprint: r.Inputs, writtenAt: r.UpdatedAt}
}

// remember saves day as the member's briefing, written at now from facts
// with fingerprint. Like the other cards' remember, it outlives the
// caller's cancellation, and a failure is only logged.
func (b *Briefings) remember(ctx context.Context, userID int64, day briefingDay, fingerprint string, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	payload, err := json.Marshal(day)
	if err == nil {
		err = b.results.PutCuratedResult(ctx, store.CuratedResult{UserID: userID, Kind: kindBriefing, Inputs: fingerprint, Payload: payload, UpdatedAt: now})
	}
	if err != nil {
		b.logger.WarnContext(ctx, "couldn't remember the briefing", "user_id", userID, "err", err)
	}
}

// gatherFacts collects what the member's briefing can say at now, in loc:
// the day's forecast, today's (and in the evening tomorrow's) events from
// their Today card, and headlines and nearby events from their remembered
// Articles and Local Events. Each is optional. Nothing here runs a paid
// search: the forecast is free, and the rest is read from what's
// remembered.
func (b *Briefings) gatherFacts(ctx context.Context, userID int64, profile store.MemberProfile, aud sources.Audience, loc *time.Location, now time.Time) briefingFacts {
	local := now.In(loc)
	date := local.Format(time.DateOnly)
	evening := local.Hour() >= eveningHour
	fp := factsPrint{Part: partOfDay(local)}
	f := briefingFacts{prompt: llm.BriefingFacts{Now: local}}

	if profile.Location != "" {
		// In the evening the briefing looks ahead, so it gets tomorrow's
		// forecast rather than the rest of today's.
		today, tomorrow := b.weatherFact(ctx, userID, profile.Location, aud, date, now)
		if evening {
			f.prompt.WeatherTomorrow, fp.Weather = tomorrow, tomorrow
		} else {
			f.prompt.Weather, fp.Weather = today, today
		}
		if fp.Weather != "" {
			f.uses = append(f.uses, usesWeather)
		}
	}

	if day, ok := b.todaysEvents(ctx, userID, date, loc.String()); ok {
		fp.Today = day.Today
		for _, e := range day.Today {
			// Timed events that are over are left out of what the AI is
			// given; the fingerprint keeps them.
			if e.AllDay || e.End.After(now) || e.End.Equal(e.Start) && e.Start.After(now) {
				f.prompt.Today = append(f.prompt.Today, briefingEvent(e, loc))
			}
		}
		if evening {
			fp.Tomorrow = day.Tomorrow
			for _, e := range day.Tomorrow {
				f.prompt.Tomorrow = append(f.prompt.Tomorrow, briefingEvent(e, loc))
			}
		}
		if len(f.prompt.Today)+len(f.prompt.Tomorrow) > 0 {
			f.uses = append(f.uses, usesCalendar)
		}
	}

	if profile.Interests != "" {
		if items, ok := b.rememberedItems(ctx, userID, kindArticles, normalizeInput(profile.Interests), aud); ok {
			for _, it := range items {
				if len(f.prompt.Articles) == maxBriefingArticles {
					break
				}
				if title := shorten(it.Title, maxBriefingHeadline); title != "" {
					f.prompt.Articles = append(f.prompt.Articles, title)
				}
			}
			if len(f.prompt.Articles) > 0 {
				fp.Articles = f.prompt.Articles
				f.uses = append(f.uses, usesArticles)
			}
		}
	}

	if profile.Location != "" {
		inputs := normalizeInput(profile.Interests) + "\n" + normalizeInput(profile.Location)
		if items, ok := b.rememberedItems(ctx, userID, kindEvents, inputs, aud); ok {
			for _, it := range items {
				if len(f.prompt.Nearby) == maxBriefingNearby {
					break
				}
				if it.Title == "" {
					continue
				}
				f.prompt.Nearby = append(f.prompt.Nearby, llm.BriefingNearby{
					Name: shorten(it.Title, maxBriefingHeadline), Date: it.Date, Venue: it.Label,
				})
			}
			if len(f.prompt.Nearby) > 0 {
				fp.Nearby = f.prompt.Nearby
				f.uses = append(f.uses, usesNearby)
			}
		}
	}

	f.fingerprint = fingerprint(fp)
	return f
}

// todaysEvents returns the member's remembered Today card events, if they
// were fetched for date in timeZone from a working calendar.
func (b *Briefings) todaysEvents(ctx context.Context, userID int64, date, timeZone string) (todayDay, bool) {
	r, err := b.results.GetCuratedResult(ctx, userID, kindToday)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			b.logger.WarnContext(ctx, "couldn't load the remembered events for the briefing", "user_id", userID, "err", err)
		}
		return todayDay{}, false
	}
	var day todayDay
	if err := json.Unmarshal(r.Payload, &day); err != nil {
		b.logger.WarnContext(ctx, "couldn't read the remembered events for the briefing", "user_id", userID, "err", err)
		return todayDay{}, false
	}
	if day.Date != date || day.TimeZone != timeZone || day.EmptyReason != "" {
		return todayDay{}, false
	}
	return day, true
}

// rememberedItems returns the items of the member's remembered curated
// widget of kind, if it was picked for inputs (their current profile) and
// for aud. These are the same checks Curation.remembered makes, so
// the briefing only uses what the member's card would show.
func (b *Briefings) rememberedItems(ctx context.Context, userID int64, kind, inputs string, aud sources.Audience) ([]llm.Item, bool) {
	r, err := b.results.GetCuratedResult(ctx, userID, kind)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			b.logger.WarnContext(ctx, "couldn't load a remembered widget for the briefing", "kind", kind, "user_id", userID, "err", err)
		}
		return nil, false
	}
	if r.Inputs != inputs {
		return nil, false
	}
	var widget Widget
	if err := json.Unmarshal(r.Payload, &widget); err != nil {
		b.logger.WarnContext(ctx, "couldn't read a remembered widget for the briefing", "kind", kind, "user_id", userID, "err", err)
		return nil, false
	}
	if widget.KidSafe != (aud == sources.Kids) {
		return nil, false
	}
	return widget.Items, true
}

// weatherFact returns today's and tomorrow's forecasts for location, each as
// one line such as "Sunny. High 75°F, low 58°F. 10% chance of rain.", or ""
// where there isn't one. A forecast asked for under WeatherFreshFor ago is
// reused. If the forecast service fails, an older one from the same day is
// used, and otherwise there's no weather.
func (b *Briefings) weatherFact(ctx context.Context, userID int64, location string, aud sources.Audience, date string, now time.Time) (today, tomorrow string) {
	inputs := normalizeInput(location) + "\n" + date
	var remembered briefingWeather
	if r, err := b.results.GetCuratedResult(ctx, userID, kindBriefingWeather); err == nil && r.Inputs == inputs {
		if json.Unmarshal(r.Payload, &remembered) == nil &&
			now.Sub(r.UpdatedAt) < WeatherFreshFor && remembered.Forecast != "" && remembered.Tomorrow != "" {
			return remembered.Forecast, remembered.Tomorrow
		}
	}

	searchCtx, cancel := context.WithTimeout(ctx, briefingWeatherTimeout)
	defer cancel()
	results, err := b.weather.Search(searchCtx, location, aud)
	today, tomorrow = forecastLines(results)
	if err != nil || today == "" {
		b.logger.WarnContext(ctx, "no forecast for the briefing", "user_id", userID, "err", err)
		return remembered.Forecast, remembered.Tomorrow
	}

	payload, _ := json.Marshal(briefingWeather{Forecast: today, Tomorrow: tomorrow})
	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancelSave()
	if err := b.results.PutCuratedResult(saveCtx, store.CuratedResult{UserID: userID, Kind: kindBriefingWeather, Inputs: inputs, Payload: payload, UpdatedAt: now}); err != nil {
		b.logger.WarnContext(ctx, "couldn't remember the forecast", "user_id", userID, "err", err)
	}
	return today, tomorrow
}
