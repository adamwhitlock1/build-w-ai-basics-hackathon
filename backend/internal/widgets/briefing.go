package widgets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// Limits on how often a member's briefing is written. Each write is one AI
// call, so these cap what a member's briefing costs in a day.
const (
	maxBriefingWrites  = 4
	briefingRewriteGap = 10 * time.Minute
)

const (
	// The briefing's kinds in the ResultStore: its text, and the day's
	// forecast it was written from.
	kindBriefing        = "briefing"
	kindBriefingWeather = "briefing_weather"
	// briefingBudget bounds writing a briefing: inside RequestBudget, with
	// room for callJSON's one retry on a fast model.
	briefingBudget = 25 * time.Second
	// briefingWeatherTimeout bounds asking for the day's forecast. The
	// briefing is still written without it.
	briefingWeatherTimeout = 6 * time.Second
	// How many headlines and nearby events a briefing may draw on.
	maxBriefingArticles = 3
	maxBriefingNearby   = 3
	// maxBriefingHeadline is about one line of text.
	maxBriefingHeadline = 120
	// eveningHour is when the evening starts: the briefing adds tomorrow's
	// events, as the Today card does.
	eveningHour = 18
)

// EmptyNothingYet means the member has no weather, calendar, articles or
// nearby events for a briefing to talk about.
const EmptyNothingYet EmptyReason = "nothing_yet"

// The facts a briefing can be written from, as listed in its Uses.
const (
	usesWeather  = "weather"
	usesCalendar = "calendar"
	usesArticles = "articles"
	usesNearby   = "nearby"
)

// Briefer writes a morning briefing from facts. *llm.Client satisfies it;
// tests use a fake.
type Briefer interface {
	WriteBriefing(ctx context.Context, facts llm.BriefingFacts, aud sources.Audience) (string, error)
}

// BriefingHandler serves the morning briefing: a few sentences the AI
// writes about a member's day from what their other cards already hold,
// plus the day's forecast. It writes one on the member's first visit of the
// day, then sends it again until the facts behind it change, writing at most
// maxBriefingWrites a day, at least briefingRewriteGap apart. It never runs
// a paid search.
//
// Like the other cards, it acts on the request's subject: the logged-in
// user, or the family member a parent is looking at.
type BriefingHandler struct {
	briefer  Briefer
	weather  sources.Source
	profiles ProfileStore
	results  ResultStore
	locks    memberLocks
	logger   *slog.Logger
	now      func() time.Time // timeNow; tests set another clock
}

// NewBriefingHandler returns a BriefingHandler that writes with briefer,
// asks for forecasts from the weather_forecast source in srcs, reads
// profiles from profiles, and reads and remembers results in results. It
// panics if the weather source is missing, as NewCuratedHandler does.
func NewBriefingHandler(briefer Briefer, srcs SourceFinder, profiles ProfileStore, results ResultStore, logger *slog.Logger) *BriefingHandler {
	return &BriefingHandler{
		briefer:  briefer,
		weather:  mustGetSource(srcs, sources.WeatherForecast),
		profiles: profiles,
		results:  results,
		logger:   logger,
		now:      timeNow,
	}
}

// BriefingWidget is the response body of GET /api/widgets/briefing (and of
// GET /api/members/{id}/widgets/briefing). It's never stale: the briefing is
// rewritten when its facts change, not when it grows old.
type BriefingWidget struct {
	Freshness               // UpdatedAt is when the briefing was written
	EmptyReason EmptyReason `json:"emptyReason"` // "" or EmptyNothingYet
	Text        string      `json:"text"`
	Date        string      `json:"date"` // today in the request's time zone, YYYY-MM-DD
	KidSafe     bool        `json:"kidSafe"`
	Uses        []string    `json:"uses"` // which facts it drew on; never null
}

// briefingDay is a member's remembered briefing. The fingerprint of the
// facts it was written from is the remembered result's Inputs.
type briefingDay struct {
	Text     string   `json:"text"`
	Date     string   `json:"date"`
	TimeZone string   `json:"timeZone"`
	KidSafe  bool     `json:"kidSafe"`
	Uses     []string `json:"uses"`
	Writes   int      `json:"writes"` // briefings written for this day, this one included
}

// rememberedBriefing is today's remembered briefing, with the fingerprint
// of its facts and when it was written.
type rememberedBriefing struct {
	day         briefingDay
	fingerprint string
	writtenAt   time.Time
}

// Briefing handles GET /api/widgets/briefing?tz=<IANA zone> (and GET
// /api/members/{id}/widgets/briefing): the member's briefing for today in
// time zone tz (UTC if tz is missing or unknown). See BriefingHandler for
// when it's written.
func (h *BriefingHandler) Briefing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	viewer, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	member, _ := auth.SubjectFrom(ctx) // falls back to viewer
	aud := audienceFor(member)
	loc := locationFrom(r, h.logger)
	now := h.now()
	date := now.In(loc).Format(time.DateOnly)

	profile, err := h.profiles.GetMemberProfile(ctx, member.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.logger.ErrorContext(ctx, "load profile failed", "user_id", member.ID, "viewer_id", viewer.ID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
		return
	}
	profile.Interests = strings.TrimSpace(profile.Interests)
	profile.Location = strings.TrimSpace(profile.Location)

	// One briefing request at a time per member, so two tabs (or a parent
	// and the member) opening at once pay for one AI call, not two.
	unlock := h.locks.lock(member.ID)
	defer unlock()

	facts := h.gatherFacts(ctx, member.ID, profile, aud, loc, now)
	widget := BriefingWidget{Freshness: justUpdated(now), Date: date, KidSafe: aud == sources.Kids, Uses: []string{}}
	if len(facts.uses) == 0 {
		widget.EmptyReason = EmptyNothingYet
		respond.JSON(w, http.StatusOK, widget)
		return
	}

	today := h.rememberedToday(ctx, member.ID, date, loc.String(), aud)
	if !shouldWrite(today, facts.fingerprint, now) {
		h.logger.InfoContext(ctx, "briefing remembered", "user_id", member.ID, "viewer_id", viewer.ID, "writes", today.day.Writes)
		respond.JSON(w, http.StatusOK, today.widget())
		return
	}

	writeCtx, cancel := context.WithTimeout(ctx, briefingBudget)
	defer cancel()
	text, err := h.briefer.WriteBriefing(writeCtx, facts.prompt, aud)
	if err != nil {
		if writeIfCancelled(w, r, h.logger, err) {
			return
		}
		if today != nil {
			// The briefing from earlier today still says something true.
			h.logger.WarnContext(ctx, "couldn't rewrite the briefing; sending the earlier one", "user_id", member.ID, "err", err)
			respond.JSON(w, http.StatusOK, today.widget())
			return
		}
		h.writeError(w, r, member.ID, err)
		return
	}

	day := briefingDay{Text: text, Date: date, TimeZone: loc.String(), KidSafe: aud == sources.Kids, Uses: facts.uses, Writes: 1}
	if today != nil {
		day.Writes = today.day.Writes + 1
	}
	h.remember(ctx, member.ID, day, facts.fingerprint, now)
	h.logger.InfoContext(ctx, "briefing written",
		"user_id", member.ID,
		"viewer_id", viewer.ID,
		"uses", strings.Join(facts.uses, ","),
		"writes", day.Writes,
		"kid_safe", day.KidSafe,
	)
	respond.JSON(w, http.StatusOK, rememberedBriefing{day: day, writtenAt: now}.widget())
}

// widget is the response for a remembered briefing.
func (b rememberedBriefing) widget() BriefingWidget {
	uses := b.day.Uses
	if uses == nil {
		uses = []string{}
	}
	return BriefingWidget{
		Freshness: justUpdated(b.writtenAt), // written, never stale
		Text:      b.day.Text,
		Date:      b.day.Date,
		KidSafe:   b.day.KidSafe,
		Uses:      uses,
	}
}

// shouldWrite decides whether to write a new briefing, given today's
// remembered one (nil if there isn't one) and the fingerprint of the facts
// as they are now. It writes when there's nothing from today, or when the
// facts have changed, unless the last one is under briefingRewriteGap old
// or maxBriefingWrites have been written today.
func shouldWrite(today *rememberedBriefing, fingerprint string, now time.Time) bool {
	switch {
	case today == nil:
		return true
	case today.fingerprint == fingerprint:
		return false
	case today.day.Writes >= maxBriefingWrites:
		return false
	case now.Sub(today.writtenAt) < briefingRewriteGap:
		return false
	}
	return true
}

// rememberedToday returns the member's remembered briefing if it was
// written for date in timeZone, for aud; otherwise nil. A briefing written
// for the other audience counts as missing, so a kid is never sent one
// written without the kid guidance.
func (h *BriefingHandler) rememberedToday(ctx context.Context, userID int64, date, timeZone string, aud sources.Audience) *rememberedBriefing {
	r, err := h.results.GetCuratedResult(ctx, userID, kindBriefing)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.WarnContext(ctx, "couldn't load the remembered briefing; writing again", "user_id", userID, "err", err)
		}
		return nil
	}
	var day briefingDay
	if err := json.Unmarshal(r.Payload, &day); err != nil {
		h.logger.WarnContext(ctx, "couldn't read the remembered briefing; writing again", "user_id", userID, "err", err)
		return nil
	}
	if day.Date != date || day.TimeZone != timeZone || day.KidSafe != (aud == sources.Kids) || day.Text == "" {
		return nil
	}
	return &rememberedBriefing{day: day, fingerprint: r.Inputs, writtenAt: r.UpdatedAt}
}

// remember saves day as the member's briefing, written at now from facts
// with fingerprint. Like the other cards' remember, it outlives the
// request, and a failure is only logged.
func (h *BriefingHandler) remember(ctx context.Context, userID int64, day briefingDay, fingerprint string, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	payload, err := json.Marshal(day)
	if err == nil {
		err = h.results.PutCuratedResult(ctx, store.CuratedResult{UserID: userID, Kind: kindBriefing, Inputs: fingerprint, Payload: payload, UpdatedAt: now})
	}
	if err != nil {
		h.logger.WarnContext(ctx, "couldn't remember the briefing", "user_id", userID, "err", err)
	}
}

// writeError answers a briefing that couldn't be written, when there's no
// earlier one from today to send instead.
func (h *BriefingHandler) writeError(w http.ResponseWriter, r *http.Request, userID int64, err error) {
	h.logger.ErrorContext(r.Context(), "briefing writing failed", "user_id", userID, "err", err)
	var upstream *llm.UpstreamError
	if errors.As(err, &upstream) && upstream.Status != 0 {
		// OpenRouter itself said no (a bad key, no credits, a rate limit):
		// its reason and hint are what to fix.
		respond.Error(w, http.StatusBadGateway, "The AI service had a problem: "+upstream.Reason+".", upstream.Hint)
		return
	}
	respond.Error(w, http.StatusBadGateway, "Couldn't write your briefing right now.", "Try again in a moment.")
}

// briefingFacts is what a briefing would be written from right now.
type briefingFacts struct {
	prompt      llm.BriefingFacts // what the AI is given
	fingerprint string            // changes when the facts the briefing depends on change
	uses        []string          // which kinds of fact there are; empty means none
}

// factsPrint is what the fingerprint covers. It holds the whole day's
// events rather than only those still to come, so an event ending doesn't
// count as a change; the part of the day stands in for the time.
type factsPrint struct {
	Part     string               `json:"part"`
	Weather  string               `json:"weather"`
	Today    []TodayEvent         `json:"today"`
	Tomorrow []TodayEvent         `json:"tomorrow"` // only in the evening
	Articles []string             `json:"articles"`
	Nearby   []llm.BriefingNearby `json:"nearby"`
}

// gatherFacts collects what the member's briefing can say at now, in loc:
// the day's forecast, today's (and in the evening tomorrow's) events from
// their Today card, and headlines and nearby events from their remembered
// Articles and Local Events. Each is optional. Nothing here runs a paid
// search: the forecast is free, and the rest is read from what's
// remembered.
func (h *BriefingHandler) gatherFacts(ctx context.Context, userID int64, profile store.MemberProfile, aud sources.Audience, loc *time.Location, now time.Time) briefingFacts {
	local := now.In(loc)
	date := local.Format(time.DateOnly)
	evening := local.Hour() >= eveningHour
	fp := factsPrint{Part: partOfDay(local)}
	f := briefingFacts{prompt: llm.BriefingFacts{Now: local}}

	if profile.Location != "" {
		// In the evening the briefing looks ahead, so it gets tomorrow's
		// forecast rather than the rest of today's.
		today, tomorrow := h.weatherFact(ctx, userID, profile.Location, aud, date, now)
		if evening {
			f.prompt.WeatherTomorrow, fp.Weather = tomorrow, tomorrow
		} else {
			f.prompt.Weather, fp.Weather = today, today
		}
		if fp.Weather != "" {
			f.uses = append(f.uses, usesWeather)
		}
	}

	if day, ok := h.todaysEvents(ctx, userID, date, loc.String()); ok {
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
		if items, ok := h.rememberedItems(ctx, userID, kindArticles, normalizeInput(profile.Interests), aud); ok {
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
		if items, ok := h.rememberedItems(ctx, userID, kindEvents, inputs, aud); ok {
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

// partOfDay names the part of the day at t: "morning" before noon,
// "afternoon" before eveningHour, then "evening".
func partOfDay(t time.Time) string {
	switch h := t.Hour(); {
	case h < 12:
		return "morning"
	case h < eveningHour:
		return "afternoon"
	default:
		return "evening"
	}
}

// fingerprint identifies a set of facts: the same facts always give the
// same fingerprint. json.Marshal writes a struct's fields in order, so the
// encoding is stable.
func fingerprint(p factsPrint) string {
	b, err := json.Marshal(p)
	if err != nil { // can't happen: every field is a plain value
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

// briefingEvent writes a Today card event as the AI is given it.
func briefingEvent(e TodayEvent, loc *time.Location) llm.BriefingEvent {
	at := "All day"
	if !e.AllDay {
		at = e.Start.In(loc).Format("3:04 PM")
	}
	return llm.BriefingEvent{Time: at, Title: e.Title, Location: e.Location}
}

// todaysEvents returns the member's remembered Today card events, if they
// were fetched for date in timeZone from a working calendar.
func (h *BriefingHandler) todaysEvents(ctx context.Context, userID int64, date, timeZone string) (todayDay, bool) {
	r, err := h.results.GetCuratedResult(ctx, userID, kindToday)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.WarnContext(ctx, "couldn't load the remembered events for the briefing", "user_id", userID, "err", err)
		}
		return todayDay{}, false
	}
	var day todayDay
	if err := json.Unmarshal(r.Payload, &day); err != nil {
		h.logger.WarnContext(ctx, "couldn't read the remembered events for the briefing", "user_id", userID, "err", err)
		return todayDay{}, false
	}
	if day.Date != date || day.TimeZone != timeZone || day.EmptyReason != "" {
		return todayDay{}, false
	}
	return day, true
}

// rememberedItems returns the items of the member's remembered curated
// widget of kind, if it was picked for inputs (their current profile) and
// for aud. These are the same checks CuratedHandler.sendRemembered makes, so
// the briefing only uses what the member's card would show.
func (h *BriefingHandler) rememberedItems(ctx context.Context, userID int64, kind, inputs string, aud sources.Audience) ([]llm.Item, bool) {
	r, err := h.results.GetCuratedResult(ctx, userID, kind)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.WarnContext(ctx, "couldn't load a remembered widget for the briefing", "kind", kind, "user_id", userID, "err", err)
		}
		return nil, false
	}
	if r.Inputs != inputs {
		return nil, false
	}
	var widget Widget
	if err := json.Unmarshal(r.Payload, &widget); err != nil {
		h.logger.WarnContext(ctx, "couldn't read a remembered widget for the briefing", "kind", kind, "user_id", userID, "err", err)
		return nil, false
	}
	if widget.KidSafe != (aud == sources.Kids) {
		return nil, false
	}
	return widget.Items, true
}

// briefingWeather is the remembered forecast a briefing uses: today's and
// tomorrow's lines. Its Inputs are the location and date it's for.
type briefingWeather struct {
	Forecast string `json:"forecast"` // today's
	Tomorrow string `json:"tomorrow"`
}

// weatherFact returns today's and tomorrow's forecasts for location, each as
// one line such as "Sunny. High 75°F, low 58°F. 10% chance of rain.", or ""
// where there isn't one. A forecast asked for under WeatherFreshFor ago is
// reused. If the forecast service fails, an older one from the same day is
// used, and otherwise there's no weather.
func (h *BriefingHandler) weatherFact(ctx context.Context, userID int64, location string, aud sources.Audience, date string, now time.Time) (today, tomorrow string) {
	inputs := normalizeInput(location) + "\n" + date
	var remembered briefingWeather
	if r, err := h.results.GetCuratedResult(ctx, userID, kindBriefingWeather); err == nil && r.Inputs == inputs {
		if json.Unmarshal(r.Payload, &remembered) == nil &&
			now.Sub(r.UpdatedAt) < WeatherFreshFor && remembered.Forecast != "" && remembered.Tomorrow != "" {
			return remembered.Forecast, remembered.Tomorrow
		}
	}

	searchCtx, cancel := context.WithTimeout(ctx, briefingWeatherTimeout)
	defer cancel()
	results, err := h.weather.Search(searchCtx, location, aud)
	today, tomorrow = forecastLines(results)
	if err != nil || today == "" {
		h.logger.WarnContext(ctx, "no forecast for the briefing", "user_id", userID, "err", err)
		return remembered.Forecast, remembered.Tomorrow
	}

	payload, _ := json.Marshal(briefingWeather{Forecast: today, Tomorrow: tomorrow})
	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancelSave()
	if err := h.results.PutCuratedResult(saveCtx, store.CuratedResult{UserID: userID, Kind: kindBriefingWeather, Inputs: inputs, Payload: payload, UpdatedAt: now}); err != nil {
		h.logger.WarnContext(ctx, "couldn't remember the forecast", "user_id", userID, "err", err)
	}
	return today, tomorrow
}

// forecastLines picks today's and tomorrow's lines out of the forecast
// source's results: the snippets of the day titled "Today (…)" and of the
// day after it. The "Right now" result is left out, since its temperature
// changes by the hour. The snippets name no place, so the member's location
// never reaches the AI.
func forecastLines(results []sources.Result) (today, tomorrow string) {
	for i, r := range results {
		if strings.HasPrefix(r.Title, "Today (") {
			today = strings.TrimSpace(r.Snippet)
			if i+1 < len(results) {
				tomorrow = strings.TrimSpace(results[i+1].Snippet)
			}
			return today, tomorrow
		}
	}
	return "", ""
}

// memberLocks hands out one lock per member, so each member's briefing is
// worked out by one request at a time while different members never wait on
// each other. Locks are never removed: one small mutex per member is
// nothing for a family app.
//
// A sync.Mutex is a lock only one goroutine can hold at a time; Lock waits
// until it's free. The zero value of memberLocks is ready to use.
type memberLocks struct {
	mu    sync.Mutex // guards locks
	locks map[int64]*sync.Mutex
}

// lock waits until no other request holds member id's lock, takes it, and
// returns the function that releases it.
func (l *memberLocks) lock(id int64) (unlock func()) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[int64]*sync.Mutex)
	}
	m, ok := l.locks[id]
	if !ok {
		m = &sync.Mutex{}
		l.locks[id] = m
	}
	// Release the map's lock before waiting on the member's, so a request
	// for someone else isn't held up behind this one.
	l.mu.Unlock()
	m.Lock()
	return m.Unlock
}
