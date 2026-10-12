package widgets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"porchlight/internal/auth"
	"porchlight/internal/calendar"
	"porchlight/internal/respond"
	"porchlight/internal/store"
)

// TodayFreshFor is how long the Today card's events stay fresh. Fetching a
// calendar costs no credit, and plans change during the day.
const TodayFreshFor = 15 * time.Minute

const (
	// kindToday is the Today card's kind in the ResultStore.
	kindToday = "today"
	// todayBudget bounds fetching and reading a calendar for the Today card.
	todayBudget = 20 * time.Second
	// connectBudget bounds the check that a newly pasted link is a calendar.
	connectBudget = 15 * time.Second
	// maxDayEvents is how many events the card lists for one day; the rest
	// are counted in MoreToday or MoreTomorrow.
	maxDayEvents = 12
)

// The Today card's reasons for having no events, besides there being none.
const (
	EmptyNoCalendar   EmptyReason = "no_calendar"   // no calendar link connected
	EmptyCalendarGone EmptyReason = "calendar_gone" // the link stopped working
)

// CalendarFetcher gets a calendar feed. *calendar.Fetcher satisfies it.
type CalendarFetcher interface {
	Fetch(ctx context.Context, link string) ([]byte, error)
}

// CalendarStore keeps members' calendar links and the Today card's
// remembered events. *store.Store satisfies it.
type CalendarStore interface {
	ResultStore
	GetCalendarLink(ctx context.Context, userID int64) (string, error)
	SetCalendarLink(ctx context.Context, userID int64, link string) error
	DeleteCuratedResult(ctx context.Context, userID int64, kind string) error
}

// TodayHandler serves the Today card and the endpoints that connect and
// remove a member's calendar. Like CuratedHandler, it acts on the request's
// subject: the logged-in user, or the family member a parent is looking at.
//
// A calendar link is a secret. It's read from the store, handed to the
// fetcher, and hashed into the remembered result's inputs; it is never
// written to a response or a log line.
type TodayHandler struct {
	fetcher CalendarFetcher
	store   CalendarStore
	logger  *slog.Logger
}

// NewTodayHandler returns a TodayHandler that fetches calendars with
// fetcher and keeps links and remembered events in st.
func NewTodayHandler(fetcher CalendarFetcher, st CalendarStore, logger *slog.Logger) *TodayHandler {
	return &TodayHandler{fetcher: fetcher, store: st, logger: logger}
}

// TodayEvent is one event on the Today card.
type TodayEvent struct {
	Title    string    `json:"title"`
	Location string    `json:"location"`
	Start    time.Time `json:"start"` // RFC 3339, in the member's time zone
	End      time.Time `json:"end"`
	AllDay   bool      `json:"allDay"`
}

// TodayWidget is the response body of GET /api/widgets/today (and of GET
// /api/members/{id}/widgets/today). It always holds today's and tomorrow's
// events; the page decides when to show tomorrow's.
type TodayWidget struct {
	todayDay
	Freshness
}

// todayDay is a TodayWidget without its freshness: what's remembered.
type todayDay struct {
	EmptyReason  EmptyReason  `json:"emptyReason"`
	Provider     string       `json:"provider"` // e.g. "Google Calendar"; "" with no calendar
	Date         string       `json:"date"`     // today in TimeZone, YYYY-MM-DD
	TimeZone     string       `json:"timeZone"`
	Today        []TodayEvent `json:"today"` // never null
	MoreToday    int          `json:"moreToday"`
	Tomorrow     []TodayEvent `json:"tomorrow"` // never null
	MoreTomorrow int          `json:"moreTomorrow"`
}

// Today handles GET /api/widgets/today?tz=<IANA zone>[&refresh=1]: the
// member's events for today and tomorrow in time zone tz (UTC if tz is
// missing or unknown). Events are remembered for the day and sent again
// without fetching; after TodayFreshFor they're sent marked stale, and the
// page asks for a refresh. A new day, time zone or link fetches again.
func (h *TodayHandler) Today(w http.ResponseWriter, r *http.Request) {
	member, ok := auth.SubjectFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	loc := h.location(r)
	now := timeNow()
	day := todayDay{Date: now.In(loc).Format(time.DateOnly), TimeZone: loc.String(), Today: []TodayEvent{}, Tomorrow: []TodayEvent{}}

	link, err := h.store.GetCalendarLink(r.Context(), member.ID)
	if err != nil {
		h.serverError(w, r, "load calendar link", err)
		return
	}
	if link == "" {
		day.EmptyReason = EmptyNoCalendar
		respond.JSON(w, http.StatusOK, TodayWidget{todayDay: day, Freshness: justUpdated(now)})
		return
	}
	day.Provider = calendar.Provider(link)
	inputs := linkHash(link) + "\n" + day.TimeZone + "\n" + day.Date
	if h.sendRemembered(w, r, member.ID, inputs, now) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), todayBudget)
	defer cancel()
	feed, err := h.fetcher.Fetch(ctx, link)
	if errors.Is(err, calendar.ErrLinkGone) {
		// The link was revoked or deleted: say so, and forget its events.
		h.logger.InfoContext(ctx, "calendar link stopped working", "user_id", member.ID, "provider", day.Provider)
		if err := h.store.DeleteCuratedResult(context.WithoutCancel(ctx), member.ID, kindToday); err != nil {
			h.logger.WarnContext(ctx, "couldn't forget the remembered events", "user_id", member.ID, "err", err)
		}
		day.EmptyReason = EmptyCalendarGone
		respond.JSON(w, http.StatusOK, TodayWidget{todayDay: day, Freshness: justUpdated(now)})
		return
	}
	if err != nil {
		h.calendarError(w, r, member.ID, day.Provider, err)
		return
	}

	// Today and tomorrow, as calendar days in the member's time zone.
	y, m, d := now.In(loc).Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	events, err := calendar.Between(feed, todayStart, tomorrowStart.AddDate(0, 0, 1), loc)
	if err != nil {
		h.calendarError(w, r, member.ID, day.Provider, err)
		return
	}
	day.Today, day.MoreToday = eventsOn(events, todayStart, tomorrowStart)
	day.Tomorrow, day.MoreTomorrow = eventsOn(events, tomorrowStart, tomorrowStart.AddDate(0, 0, 1))

	h.remember(r.Context(), member.ID, inputs, day, now)
	h.logger.InfoContext(ctx, "today's events fetched", "user_id", member.ID, "provider", day.Provider, "today", len(day.Today)+day.MoreToday, "tomorrow", len(day.Tomorrow)+day.MoreTomorrow)
	respond.JSON(w, http.StatusOK, TodayWidget{todayDay: day, Freshness: justUpdated(now)})
}

// location returns the time zone in the request's tz parameter, or UTC if
// it's missing or unknown.
func (h *TodayHandler) location(r *http.Request) *time.Location {
	tz := r.URL.Query().Get("tz")
	if tz == "" || tz == "Local" { // "Local" would be the server's zone, not the member's
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		h.logger.InfoContext(r.Context(), "unknown time zone; using UTC", "tz", tz)
		return time.UTC
	}
	return loc
}

// eventsOn returns the events that overlap [from, to), at most maxDayEvents
// of them, and how many more there were. An event that spans several days
// is on each of them.
func eventsOn(events []calendar.Event, from, to time.Time) (out []TodayEvent, more int) {
	out = []TodayEvent{}
	for _, e := range events {
		overlaps := e.Start.Before(to) && (e.End.After(from) || e.End.Equal(e.Start) && !e.Start.Before(from))
		if !overlaps {
			continue
		}
		if len(out) == maxDayEvents {
			more++
			continue
		}
		out = append(out, TodayEvent{Title: e.Title, Location: e.Location, Start: e.Start, End: e.End, AllDay: e.AllDay})
	}
	return out, more
}

// linkHash identifies a calendar link in a remembered result's inputs,
// without storing the link itself there.
func linkHash(link string) string {
	sum := sha256.Sum256([]byte(link))
	return hex.EncodeToString(sum[:8])
}

// sendRemembered sends the member's remembered events, and reports true,
// when they were fetched for inputs (this link, time zone and day) and the
// request doesn't need new ones. As for the curated cards, a refresh needs
// new ones unless the remembered ones are under a minute old.
func (h *TodayHandler) sendRemembered(w http.ResponseWriter, r *http.Request, userID int64, inputs string, now time.Time) bool {
	ctx := r.Context()
	remembered, err := h.store.GetCuratedResult(ctx, userID, kindToday)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return false
	case err != nil:
		h.logger.WarnContext(ctx, "couldn't load the remembered events; fetching again", "user_id", userID, "err", err)
		return false
	case remembered.Inputs != inputs:
		return false
	case r.URL.Query().Get("refresh") == "1" && !recentlyUpdated(remembered.UpdatedAt, now):
		return false
	}
	var day todayDay
	if err := json.Unmarshal(remembered.Payload, &day); err != nil {
		h.logger.WarnContext(ctx, "couldn't read the remembered events; fetching again", "user_id", userID, "err", err)
		return false
	}
	if day.Today == nil {
		day.Today = []TodayEvent{}
	}
	if day.Tomorrow == nil {
		day.Tomorrow = []TodayEvent{}
	}
	respond.JSON(w, http.StatusOK, TodayWidget{todayDay: day, Freshness: freshness(remembered.UpdatedAt, now, TodayFreshFor)})
	return true
}

// remember saves day as the member's remembered events. Like
// CuratedHandler.remember, it outlives the request, and a failure is only
// logged.
func (h *TodayHandler) remember(ctx context.Context, userID int64, inputs string, day todayDay, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	payload, err := json.Marshal(day)
	if err == nil {
		err = h.store.PutCuratedResult(ctx, store.CuratedResult{UserID: userID, Kind: kindToday, Inputs: inputs, Payload: payload, UpdatedAt: now})
	}
	if err != nil {
		h.logger.WarnContext(ctx, "couldn't remember today's events", "user_id", userID, "err", err)
	}
}

// calendarError answers a failed calendar fetch or read with a 502, so the
// page keeps what it shows and offers "Try again". err never contains the
// link (see the calendar package).
func (h *TodayHandler) calendarError(w http.ResponseWriter, r *http.Request, userID int64, provider string, err error) {
	if writeIfCancelled(w, r, h.logger, err) {
		return
	}
	h.logger.WarnContext(r.Context(), "couldn't load the calendar", "user_id", userID, "provider", provider, "err", err)
	respond.Error(w, http.StatusBadGateway, "Couldn't load the calendar right now.", "Try again in a moment.")
}

// CalendarResponse is the response body of SetCalendar and RemoveCalendar.
type CalendarResponse struct {
	Calendar calendar.Summary `json:"calendar"`
}

type setCalendarRequest struct {
	URL string `json:"url"`
}

const calendarExample = `Send JSON like {"url": "https://calendar.google.com/calendar/ical/.../basic.ics"}.`

// calendarHelp says where to find a calendar's private link.
const calendarHelp = `In Google Calendar, open the calendar's settings and copy "Secret address in iCal format". In iCloud, share the calendar as a public calendar and copy its link.`

// SetCalendar handles PUT /api/me/calendar (and PUT
// /api/members/{id}/calendar, where a parent sets a family member's): it
// checks that the pasted link is a calendar by fetching it once, saves it,
// forgets the events remembered from any earlier link, and responds with
// what a browser may know about it. A link that fails the check is not
// saved, and an earlier one stays.
func (h *TodayHandler) SetCalendar(w http.ResponseWriter, r *http.Request) {
	member, ok := auth.SubjectFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	var req setCalendarRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respond.Error(w, http.StatusBadRequest, calendarExample, "")
		return
	}
	link, err := calendar.NormalizeLink(req.URL)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "That doesn't look like a calendar link.", "Paste the https:// or webcal:// address. "+calendarHelp)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), connectBudget)
	defer cancel()
	feed, err := h.fetcher.Fetch(ctx, link)
	if err == nil {
		// Check it reads as a calendar too, not just that it starts like one.
		now := time.Now()
		_, err = calendar.Between(feed, now, now.Add(time.Hour), time.UTC)
	}
	if err != nil {
		if writeIfCancelled(w, r, h.logger, err) {
			return
		}
		viewer, _ := auth.UserFrom(r.Context())
		h.logger.InfoContext(r.Context(), "calendar link refused", "user_id", member.ID, "viewer_id", viewer.ID, "provider", calendar.Provider(link), "err", err)
		switch {
		case errors.Is(err, calendar.ErrNotCalendar):
			respond.Error(w, http.StatusUnprocessableEntity, "That link didn't give us a calendar.", calendarHelp)
		case errors.Is(err, calendar.ErrBlockedAddress):
			respond.Error(w, http.StatusUnprocessableEntity, "That link points somewhere Porchlight can't reach.", "Use the calendar's public https:// address. "+calendarHelp)
		default:
			respond.Error(w, http.StatusUnprocessableEntity, "Couldn't open that calendar link.", "Check it and try again. "+calendarHelp)
		}
		return
	}

	saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(r.Context()), saveTimeout)
	defer cancelSave()
	if err := h.store.SetCalendarLink(saveCtx, member.ID, link); err != nil {
		h.serverError(w, r, "save calendar link", err)
		return
	}
	h.forgetEvents(saveCtx, member.ID)
	viewer, _ := auth.UserFrom(r.Context())
	h.logger.InfoContext(r.Context(), "calendar connected", "user_id", member.ID, "viewer_id", viewer.ID, "provider", calendar.Provider(link))
	respond.JSON(w, http.StatusOK, CalendarResponse{Calendar: calendar.Summarize(link)})
}

// RemoveCalendar handles DELETE /api/me/calendar (and DELETE
// /api/members/{id}/calendar): it removes the member's calendar link and
// forgets their remembered events.
func (h *TodayHandler) RemoveCalendar(w http.ResponseWriter, r *http.Request) {
	member, ok := auth.SubjectFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), saveTimeout)
	defer cancel()
	if err := h.store.SetCalendarLink(ctx, member.ID, ""); err != nil {
		h.serverError(w, r, "remove calendar link", err)
		return
	}
	h.forgetEvents(ctx, member.ID)
	viewer, _ := auth.UserFrom(r.Context())
	h.logger.InfoContext(r.Context(), "calendar removed", "user_id", member.ID, "viewer_id", viewer.ID)
	respond.JSON(w, http.StatusOK, CalendarResponse{Calendar: calendar.Summary{}})
}

// forgetEvents drops the member's remembered Today events. A failure is
// only logged: the remembered events are keyed by the link's hash, so
// they'd be ignored anyway.
func (h *TodayHandler) forgetEvents(ctx context.Context, userID int64) {
	if err := h.store.DeleteCuratedResult(ctx, userID, kindToday); err != nil {
		h.logger.WarnContext(ctx, "couldn't forget the remembered events", "user_id", userID, "err", err)
	}
}

func (h *TodayHandler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}
