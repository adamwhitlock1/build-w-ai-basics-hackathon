package widgets_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/calendar"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// The link's secret part: no response or log line may contain it.
const (
	calSecret = "private-SECRET123"
	calLink   = "https://calendar.google.com/calendar/ical/sam/" + calSecret + "/basic.ics"
)

// fakeFetcher returns a canned feed, or err, and counts its fetches.
type fakeFetcher struct {
	feed    string
	err     error
	fetches int
	links   []string
}

func (f *fakeFetcher) Fetch(_ context.Context, link string) ([]byte, error) {
	f.fetches++
	f.links = append(f.links, link)
	return []byte(f.feed), f.err
}

// memCalendars is a widgets.CalendarStore in memory.
type memCalendars struct {
	*memResults
	links map[int64]string
}

func newMemCalendars() *memCalendars {
	return &memCalendars{memResults: newMemResults(), links: map[int64]string{}}
}

func (m *memCalendars) GetCalendarLink(_ context.Context, userID int64) (string, error) {
	return m.links[userID], nil
}

func (m *memCalendars) SetCalendarLink(_ context.Context, userID int64, link string) error {
	m.links[userID] = link
	return nil
}

func (m *memCalendars) DeleteCuratedResult(_ context.Context, userID int64, kind string) error {
	delete(m.rows, resultKey(userID, kind))
	return nil
}

// dayFeed is a calendar, built around today in Denver, with:
//   - an all-day "Family day" every day;
//   - "Breakfast" at 7:00 Denver time every day;
//   - "Lunch call" today at noon New York time (10:00 in Denver);
//   - "Recital" tomorrow at 18:00 Denver time.
func dayFeed(extra ...string) string {
	denver, _ := time.LoadLocation("America/Denver")
	today := time.Now().In(denver)
	tomorrow := today.AddDate(0, 0, 1)
	d := func(t time.Time) string { return t.Format("20060102") }
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Test//EN",
		"BEGIN:VEVENT", "UID:family-day", "DTSTART;VALUE=DATE:20200101", "RRULE:FREQ=DAILY", "SUMMARY:Family day", "END:VEVENT",
		"BEGIN:VEVENT", "UID:breakfast", "DTSTART;TZID=America/Denver:20200101T070000", "DTEND;TZID=America/Denver:20200101T073000", "RRULE:FREQ=DAILY", "SUMMARY:Breakfast", "END:VEVENT",
		"BEGIN:VEVENT", "UID:lunch", "DTSTART;TZID=America/New_York:" + d(today) + "T120000", "DTEND;TZID=America/New_York:" + d(today) + "T123000", "SUMMARY:Lunch call", "LOCATION:Kitchen", "END:VEVENT",
		"BEGIN:VEVENT", "UID:recital", "DTSTART;TZID=America/Denver:" + d(tomorrow) + "T180000", "DTEND;TZID=America/Denver:" + d(tomorrow) + "T190000", "SUMMARY:Recital", "END:VEVENT",
	}
	lines = append(lines, extra...)
	lines = append(lines, "END:VCALENDAR")
	return strings.Join(lines, "\r\n") + "\r\n"
}

// getToday asks h for user's (or, with subject, subject's) Today card.
func getToday(t *testing.T, h *widgets.TodayHandler, user store.User, subject *store.User, query string) (*httptest.ResponseRecorder, widgets.TodayWidget) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/widgets/today?"+query, nil)
	ctx := auth.WithUser(req.Context(), user)
	if subject != nil {
		ctx = auth.WithSubject(ctx, *subject)
	}
	rec := httptest.NewRecorder()
	h.Today(rec, req.WithContext(ctx))
	var w widgets.TodayWidget
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
	}
	if strings.Contains(rec.Body.String(), calSecret) {
		t.Errorf("response leaks the calendar link: %s", rec.Body)
	}
	return rec, w
}

// describe lists events as "15:04 Title" (or "all day Title").
func describeEvents(events []widgets.TodayEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		if e.AllDay {
			out[i] = "all day " + e.Title
		} else {
			out[i] = e.Start.Format("15:04") + " " + e.Title
		}
	}
	return out
}

func TestToday(t *testing.T) {
	fetcher := &fakeFetcher{feed: dayFeed()}
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	h := widgets.NewTodayHandler(fetcher, st, discardLogger)

	rec, w := getToday(t, h, alice, nil, "tz=America/Denver")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	denver, _ := time.LoadLocation("America/Denver")
	if w.Date != time.Now().In(denver).Format(time.DateOnly) || w.TimeZone != "America/Denver" || w.Provider != "Google Calendar" || w.EmptyReason != "" {
		t.Errorf("widget = date %q, zone %q, provider %q, empty %q", w.Date, w.TimeZone, w.Provider, w.EmptyReason)
	}
	if got, want := strings.Join(describeEvents(w.Today), "|"), "all day Family day|07:00 Breakfast|10:00 Lunch call"; got != want {
		t.Errorf("today = %s, want %s", got, want)
	}
	if got, want := strings.Join(describeEvents(w.Tomorrow), "|"), "all day Family day|07:00 Breakfast|18:00 Recital"; got != want {
		t.Errorf("tomorrow = %s, want %s", got, want)
	}
	if w.Today[2].Location != "Kitchen" {
		t.Errorf("lunch location = %q, want Kitchen", w.Today[2].Location)
	}
	if w.Stale || time.Since(w.UpdatedAt) > time.Minute {
		t.Errorf("freshness = %+v, want just updated", w.Freshness)
	}

	// The result is remembered, without the link in its inputs.
	row := st.rows[resultKey(alice.ID, "today")]
	if row.Inputs == "" || strings.Contains(row.Inputs, calSecret) || strings.Contains(string(row.Payload), calSecret) {
		t.Errorf("remembered row = %+v, want inputs set and no link", row)
	}

	// Asked again: sent from memory, no fetch.
	getToday(t, h, alice, nil, "tz=America/Denver")
	if fetcher.fetches != 1 {
		t.Errorf("fetches = %d after a second request, want 1", fetcher.fetches)
	}

	// 20 minutes later it's sent stale, still without fetching.
	row.UpdatedAt = time.Now().Add(-20 * time.Minute).Truncate(time.Second)
	st.rows[resultKey(alice.ID, "today")] = row
	_, w = getToday(t, h, alice, nil, "tz=America/Denver")
	if !w.Stale || fetcher.fetches != 1 {
		t.Errorf("after 20 minutes: stale %v, fetches %d; want stale and no fetch", w.Stale, fetcher.fetches)
	}

	// A refresh fetches again.
	_, w = getToday(t, h, alice, nil, "tz=America/Denver&refresh=1")
	if w.Stale || fetcher.fetches != 2 {
		t.Errorf("after a refresh: stale %v, fetches %d; want fresh and 2", w.Stale, fetcher.fetches)
	}
	// A second refresh within a minute doesn't.
	getToday(t, h, alice, nil, "tz=America/Denver&refresh=1")
	if fetcher.fetches != 2 {
		t.Errorf("fetches = %d after a quick second refresh, want 2", fetcher.fetches)
	}

	// Another time zone, another day or another link all fetch again.
	getToday(t, h, alice, nil, "tz=Asia/Tokyo")
	if fetcher.fetches != 3 {
		t.Errorf("fetches = %d after a new time zone, want 3", fetcher.fetches)
	}
	row = st.rows[resultKey(alice.ID, "today")]
	row.Inputs = strings.Replace(row.Inputs, row.Inputs[strings.LastIndex(row.Inputs, "\n")+1:], "2020-01-01", 1)
	st.rows[resultKey(alice.ID, "today")] = row
	getToday(t, h, alice, nil, "tz=Asia/Tokyo")
	if fetcher.fetches != 4 {
		t.Errorf("fetches = %d after the day changed, want 4", fetcher.fetches)
	}
	st.links[alice.ID] = calLink + "?new"
	getToday(t, h, alice, nil, "tz=Asia/Tokyo")
	if fetcher.fetches != 5 {
		t.Errorf("fetches = %d after a new link, want 5", fetcher.fetches)
	}
}

func TestTodayTimeZoneFallback(t *testing.T) {
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	h := widgets.NewTodayHandler(&fakeFetcher{feed: dayFeed()}, st, discardLogger)
	for _, q := range []string{"", "tz=Mars/Olympus", "tz=Local"} {
		_, w := getToday(t, h, alice, nil, q)
		if w.TimeZone != "UTC" {
			t.Errorf("query %q: time zone %q, want UTC", q, w.TimeZone)
		}
	}
}

func TestTodayNoCalendar(t *testing.T) {
	fetcher := &fakeFetcher{feed: dayFeed()}
	st := newMemCalendars()
	h := widgets.NewTodayHandler(fetcher, st, discardLogger)
	rec, w := getToday(t, h, alice, nil, "tz=America/Denver")
	if rec.Code != http.StatusOK || w.EmptyReason != widgets.EmptyNoCalendar || len(w.Today) != 0 || w.Today == nil {
		t.Errorf("status %d, widget %+v; want no_calendar with empty lists", rec.Code, w)
	}
	if fetcher.fetches != 0 || st.puts != 0 {
		t.Errorf("fetches %d, remembered %d; want neither", fetcher.fetches, st.puts)
	}
	if !strings.Contains(rec.Body.String(), `"today":[]`) {
		t.Errorf("body = %s, want today as []", rec.Body)
	}
}

func TestTodayLinkGone(t *testing.T) {
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	st.rows[resultKey(alice.ID, "today")] = store.CuratedResult{UserID: alice.ID, Kind: "today", Inputs: "old"}
	h := widgets.NewTodayHandler(&fakeFetcher{err: fmt.Errorf("%w: HTTP 404", calendar.ErrLinkGone)}, st, discardLogger)

	rec, w := getToday(t, h, alice, nil, "tz=America/Denver")
	if rec.Code != http.StatusOK || w.EmptyReason != widgets.EmptyCalendarGone || w.Provider != "Google Calendar" {
		t.Errorf("status %d, widget %+v; want calendar_gone", rec.Code, w)
	}
	if _, ok := st.rows[resultKey(alice.ID, "today")]; ok {
		t.Error("the remembered events were kept after the link stopped working")
	}
}

func TestTodayUnavailable(t *testing.T) {
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	old := store.CuratedResult{UserID: alice.ID, Kind: "today", Inputs: "old"}
	st.rows[resultKey(alice.ID, "today")] = old
	h := widgets.NewTodayHandler(&fakeFetcher{err: fmt.Errorf("%w: HTTP 500", calendar.ErrUnavailable)}, st, discardLogger)

	rec, _ := getToday(t, h, alice, nil, "tz=America/Denver")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "Couldn't load the calendar") {
		t.Errorf("status %d, body %s; want 502 saying the calendar couldn't load", rec.Code, rec.Body)
	}
	if st.rows[resultKey(alice.ID, "today")].Inputs != "old" {
		t.Error("a failed fetch changed the remembered events")
	}
}

func TestTodayParentViewsKid(t *testing.T) {
	fetcher := &fakeFetcher{feed: dayFeed()}
	st := newMemCalendars()
	st.links[kid.ID] = calLink
	h := widgets.NewTodayHandler(fetcher, st, discardLogger)

	parent := store.User{ID: 1, Email: "parent@example.com", IsAdmin: true}
	_, w := getToday(t, h, parent, &kid, "tz=America/Denver")
	if len(w.Today) != 3 || fetcher.links[0] != calLink {
		t.Errorf("parent's view of the kid: %d events from %v; want the kid's calendar", len(w.Today), fetcher.links)
	}
	if _, ok := st.rows[resultKey(kid.ID, "today")]; !ok {
		t.Error("the kid's events weren't remembered for the kid")
	}
}

func TestTodayCapsEvents(t *testing.T) {
	var extra []string
	for i := range 15 {
		extra = append(extra, "BEGIN:VEVENT", fmt.Sprintf("UID:busy-%d", i),
			fmt.Sprintf("DTSTART;TZID=America/Denver:20200101T%02d0000", 8+i%10), "DURATION:PT30M",
			"RRULE:FREQ=DAILY", fmt.Sprintf("SUMMARY:Busy %d", i), "END:VEVENT")
	}
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	h := widgets.NewTodayHandler(&fakeFetcher{feed: dayFeed(extra...)}, st, discardLogger)

	_, w := getToday(t, h, alice, nil, "tz=America/Denver")
	// 3 events of the day's own, plus 15 busy ones: 18 in all.
	if len(w.Today) != 12 || w.MoreToday != 6 {
		t.Errorf("today: %d events and %d more, want 12 and 6", len(w.Today), w.MoreToday)
	}
}

// putCalendar sends link to h.SetCalendar as user, for subject if given.
func putCalendar(t *testing.T, h *widgets.TodayHandler, user store.User, subject *store.User, link string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"url": link})
	req := httptest.NewRequest(http.MethodPut, "/api/me/calendar", bytes.NewReader(body))
	ctx := auth.WithUser(req.Context(), user)
	if subject != nil {
		ctx = auth.WithSubject(ctx, *subject)
	}
	rec := httptest.NewRecorder()
	h.SetCalendar(rec, req.WithContext(ctx))
	if strings.Contains(rec.Body.String(), calSecret) {
		t.Errorf("response leaks the calendar link: %s", rec.Body)
	}
	return rec
}

func TestSetCalendar(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	tests := []struct {
		name       string
		link       string
		fetchErr   error
		feed       string
		wantStatus int
		wantError  string
		wantSaved  string // the link stored afterwards
	}{
		{"google link", calLink, nil, dayFeed(), 200, "", calLink},
		{"webcal link", strings.Replace(calLink, "https://", "webcal://", 1), nil, dayFeed(), 200, "", calLink},
		{"web page", "https://example.com/" + calSecret, calendar.ErrNotCalendar, "", 422, "didn't give us a calendar", "https://old.example.com/cal.ics"},
		{"feed that won't parse", calLink, nil, "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nnot ical at all", 422, "", "https://old.example.com/cal.ics"},
		{"internal address", "https://127.0.0.1/" + calSecret, calendar.ErrBlockedAddress, "", 422, "can't reach", "https://old.example.com/cal.ics"},
		{"unreachable", calLink, calendar.ErrUnavailable, "", 422, "Couldn't open that calendar link", "https://old.example.com/cal.ics"},
		{"not a link", "http://example.com/" + calSecret, nil, "", 422, "doesn't look like a calendar link", "https://old.example.com/cal.ics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newMemCalendars()
			st.links[alice.ID] = "https://old.example.com/cal.ics"
			st.rows[resultKey(alice.ID, "today")] = store.CuratedResult{Inputs: "old"}
			h := widgets.NewTodayHandler(&fakeFetcher{feed: tt.feed, err: tt.fetchErr}, st, logger)

			rec := putCalendar(t, h, alice, nil, tt.link)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantError != "" && !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Errorf("body = %s, want it to mention %q", rec.Body, tt.wantError)
			}
			if st.links[alice.ID] != tt.wantSaved {
				t.Errorf("saved link = %q, want %q", st.links[alice.ID], tt.wantSaved)
			}
			_, remembered := st.rows[resultKey(alice.ID, "today")]
			if tt.wantStatus == 200 {
				if rec.Body.String() != `{"calendar":{"connected":true,"provider":"Google Calendar"}}`+"\n" {
					t.Errorf("body = %s", rec.Body)
				}
				if remembered {
					t.Error("events remembered from the old link were kept")
				}
			} else if !remembered {
				t.Error("a refused link discarded the remembered events")
			}
		})
	}
	if strings.Contains(logs.String(), calSecret) {
		t.Errorf("the log contains a calendar link:\n%s", logs.String())
	}
}

func TestSetCalendarForKid(t *testing.T) {
	st := newMemCalendars()
	h := widgets.NewTodayHandler(&fakeFetcher{feed: dayFeed()}, st, discardLogger)
	parent := store.User{ID: 1, Email: "parent@example.com", IsAdmin: true}
	if rec := putCalendar(t, h, parent, &kid, calLink); rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	if st.links[kid.ID] != calLink || st.links[parent.ID] != "" {
		t.Errorf("links = %v, want the kid's set and the parent's untouched", st.links)
	}
}

func TestRemoveCalendar(t *testing.T) {
	st := newMemCalendars()
	st.links[alice.ID] = calLink
	st.rows[resultKey(alice.ID, "today")] = store.CuratedResult{Inputs: "old"}
	h := widgets.NewTodayHandler(&fakeFetcher{}, st, discardLogger)

	req := httptest.NewRequest(http.MethodDelete, "/api/me/calendar", nil)
	rec := httptest.NewRecorder()
	h.RemoveCalendar(rec, req.WithContext(auth.WithUser(req.Context(), alice)))
	if rec.Code != http.StatusOK || rec.Body.String() != `{"calendar":{"connected":false,"provider":""}}`+"\n" {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
	if st.links[alice.ID] != "" {
		t.Error("the link wasn't removed")
	}
	if _, ok := st.rows[resultKey(alice.ID, "today")]; ok {
		t.Error("the remembered events weren't discarded")
	}
}
