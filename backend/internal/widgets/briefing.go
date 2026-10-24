package widgets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
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

// BriefingHandler serves the morning briefing, which Briefings writes. It
// acts on the request's subject: the logged-in user, or the family member a
// parent is looking at.
//
// It's declared as the same struct as Briefings, rather than as a struct
// holding a *Briefings. Converting between pointers to the two types costs
// nothing, so (*Briefings)(h) is the service with h's own per-member locks
// and clock, and this package's tests can still build a BriefingHandler
// from its fields.
type BriefingHandler Briefings

// NewBriefingHandler returns a BriefingHandler that writes with briefer,
// asks for forecasts from the weather_forecast source in srcs, reads
// profiles from profiles, and reads and remembers results in results. It
// panics if the weather source is missing, as NewCuratedHandler does.
func NewBriefingHandler(briefer Briefer, srcs SourceFinder, profiles ProfileStore, results ResultStore, logger *slog.Logger) *BriefingHandler {
	return (*BriefingHandler)(NewBriefings(briefer, srcs, profiles, results, logger))
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
// time zone tz (UTC if tz is missing or unknown). See Briefings for when
// it's written.
func (h *BriefingHandler) Briefing(w http.ResponseWriter, r *http.Request) {
	viewer, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	member, _ := auth.SubjectFrom(r.Context()) // falls back to viewer
	loc := locationFrom(r, h.logger)
	widget, err := (*Briefings)(h).Today(r.Context(), viewer, member, loc)
	if err != nil {
		writeServiceError(w, r, h.logger, err)
		return
	}
	respond.JSON(w, http.StatusOK, widget)
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

// briefingWeather is the remembered forecast a briefing uses: today's and
// tomorrow's lines. Its Inputs are the location and date it's for.
type briefingWeather struct {
	Forecast string `json:"forecast"` // today's
	Tomorrow string `json:"tomorrow"`
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
