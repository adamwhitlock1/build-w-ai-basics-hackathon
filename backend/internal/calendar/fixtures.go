package calendar

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// FixtureHost is the made-up host the demo family's calendar links use.
// .test is reserved and never resolves, so outside development a fixture
// link simply fails like any unreachable calendar.
const FixtureHost = "fixtures.kinlight.test"

// FixtureLink returns the calendar link of the fixture feed called name.
func FixtureLink(name string) string {
	return "https://" + FixtureHost + "/calendars/" + name + ".ics"
}

// feedFetcher is what FixtureFetcher hands real links to. *Fetcher
// satisfies it.
type feedFetcher interface {
	Fetch(ctx context.Context, link string) ([]byte, error)
}

// FixtureFetcher answers links on FixtureHost itself, with feeds built in
// code around the current time, and passes every other link to the real
// fetcher unchanged. It's for local development only (main.go turns it on
// with KINLIGHT_DEV_FIXTURES=1): it adds answers for one fake host, and
// never opens a connection of its own.
type FixtureFetcher struct {
	next feedFetcher
	now  func() time.Time
}

// NewFixtureFetcher returns a FixtureFetcher that sends real links to next
// and builds fixture feeds around now().
func NewFixtureFetcher(next feedFetcher, now func() time.Time) *FixtureFetcher {
	return &FixtureFetcher{next: next, now: now}
}

// Fetch returns the fixture feed a fixture link names, or ErrLinkGone if
// there's no such feed. Any other link is fetched by the real fetcher, with
// all of its checks.
func (f *FixtureFetcher) Fetch(ctx context.Context, link string) ([]byte, error) {
	normalized, err := NormalizeLink(link)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(normalized)
	if err != nil || !strings.EqualFold(u.Hostname(), FixtureHost) {
		return f.next.Fetch(ctx, link)
	}
	name, ok := strings.CutPrefix(u.Path, "/calendars/")
	if ok {
		name, ok = strings.CutSuffix(name, ".ics")
	}
	if !ok || strings.Contains(name, "/") {
		return nil, fmt.Errorf("%w: no such fixture", ErrLinkGone)
	}
	feed, ok := fixtureFeed(name, f.now())
	if !ok {
		return nil, fmt.Errorf("%w: no such fixture", ErrLinkGone)
	}
	return feed, nil
}

// fixtureEvent is one VEVENT of a fixture feed. Times are floating (no
// zone), so they show at the same clock time whatever zone the viewer is in.
type fixtureEvent struct {
	uid, title, location string
	start, end           time.Time // end is exclusive; for allDay, the day after
	allDay               bool
	rrule                string    // "" unless it repeats
	exdate               time.Time // a removed occurrence, or zero
	recurrenceID         time.Time // the occurrence this one moves, or zero
}

// fixtureFeed builds the feed called name around now: "today" and
// "tomorrow" are now's dates in now's location. It reports false for a name
// with no feed.
//
// Two events are placed relative to the clock: one that ended about half an
// hour ago, and one starting in about an hour and a half. Each is left out when it
// wouldn't fit inside today, so just after midnight there's no ended event,
// and late in the evening there's no later one.
func fixtureFeed(name string, now time.Time) ([]byte, bool) {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)
	at := func(day time.Time, hour, min int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), hour, min, 0, 0, loc)
	}
	weekAgo := today.AddDate(0, 0, -7)

	// The two clock-relative events start from the last quarter hour, so
	// they read like real plans ("3:30", not "3:23"). The ended one still
	// ends at least half an hour ago, and the later one still starts at
	// least an hour from now.
	now = now.Truncate(15 * time.Minute)
	ended := fixtureEvent{start: now.Add(-90 * time.Minute), end: now.Add(-30 * time.Minute)}
	later := fixtureEvent{start: now.Add(90 * time.Minute), end: now.Add(150 * time.Minute)}
	endedFits := !ended.start.Before(today)
	laterFits := !later.end.After(tomorrow)

	var events []fixtureEvent
	addIf := func(ok bool, e fixtureEvent) {
		if ok {
			events = append(events, e)
		}
	}
	switch name {
	case "maya":
		addIf(endedFits, fixtureEvent{uid: "maya-coffee", title: "Coffee with Priya", location: "Corner Bakery",
			start: ended.start, end: ended.end})
		addIf(laterFits, fixtureEvent{uid: "maya-dentist", title: "Dentist appointment", location: "Smile Dental",
			start: later.start, end: later.end})
		addIf(true, fixtureEvent{uid: "maya-recycling", title: "Recycling day",
			start: today, end: tomorrow, allDay: true})
		addIf(true, fixtureEvent{uid: "maya-book-club", title: "Book club", location: "Library community room",
			start: at(tomorrow, 18, 30), end: at(tomorrow, 19, 30)})
		addIf(true, fixtureEvent{uid: "maya-pta", title: "PTA meeting", location: "Lincoln Elementary",
			start: at(tomorrow, 19, 30), end: at(tomorrow, 20, 30)})
	case "sam":
		// A daily standup whose occurrence today was moved from 9:00 to 11:00.
		addIf(true, fixtureEvent{uid: "sam-standup", title: "Team standup",
			start: at(weekAgo, 9, 0), end: at(weekAgo, 9, 15), rrule: "FREQ=DAILY"})
		addIf(true, fixtureEvent{uid: "sam-standup", title: "Team standup",
			start: at(today, 11, 0), end: at(today, 11, 15), recurrenceID: at(today, 9, 0)})
		addIf(laterFits, fixtureEvent{uid: "sam-pickup", title: "Pick up Lily", location: "Lincoln Elementary",
			start: later.start, end: later.end})
		addIf(true, fixtureEvent{uid: "sam-run", title: "Evening run", location: "City Park",
			start: at(tomorrow, 18, 30), end: at(tomorrow, 19, 15)})
	case "jordan":
		// Daily band practice, with today's occurrence deleted.
		addIf(true, fixtureEvent{uid: "jordan-band", title: "Band practice", location: "School music room",
			start: at(weekAgo, 16, 0), end: at(weekAgo, 17, 0), rrule: "FREQ=DAILY", exdate: at(today, 16, 0)})
		addIf(endedFits, fixtureEvent{uid: "jordan-study", title: "Study group", location: "Library",
			start: ended.start, end: ended.end})
		addIf(true, fixtureEvent{uid: "jordan-test", title: "Science test",
			start: tomorrow, end: tomorrow.AddDate(0, 0, 1), allDay: true})
		addIf(true, fixtureEvent{uid: "jordan-soccer", title: "Soccer game", location: "Westside Fields",
			start: at(tomorrow, 18, 30), end: at(tomorrow, 20, 0)})
	case "alex":
		addIf(laterFits, fixtureEvent{uid: "alex-call", title: "Call with the landlord",
			start: later.start, end: later.end})
		addIf(true, fixtureEvent{uid: "alex-swim", title: "Riley's swim lesson", location: "Rec center pool",
			start: at(tomorrow, 18, 0), end: at(tomorrow, 18, 45)})
	default:
		return nil, false
	}
	return writeFeed(events, now), true
}

// writeFeed writes events as an iCalendar document, with the CRLF line
// endings the format requires.
func writeFeed(events []fixtureEvent, now time.Time) []byte {
	const (
		dateTime = "20060102T150405" // floating: no Z, no TZID
		date     = "20060102"
	)
	var b strings.Builder
	line := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteString("\r\n")
	}
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Porchlight//Dev fixtures//EN")
	line("CALSCALE:GREGORIAN")
	for _, e := range events {
		line("BEGIN:VEVENT")
		line("UID:%s@%s", e.uid, FixtureHost)
		line("DTSTAMP:%s", now.UTC().Format("20060102T150405Z"))
		line("SUMMARY:%s", escapeText(e.title))
		if e.location != "" {
			line("LOCATION:%s", escapeText(e.location))
		}
		if e.allDay {
			line("DTSTART;VALUE=DATE:%s", e.start.Format(date))
			line("DTEND;VALUE=DATE:%s", e.end.Format(date))
		} else {
			line("DTSTART:%s", e.start.Format(dateTime))
			line("DTEND:%s", e.end.Format(dateTime))
		}
		if e.rrule != "" {
			line("RRULE:%s", e.rrule)
		}
		if !e.exdate.IsZero() {
			line("EXDATE:%s", e.exdate.Format(dateTime))
		}
		if !e.recurrenceID.IsZero() {
			line("RECURRENCE-ID:%s", e.recurrenceID.Format(dateTime))
		}
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return []byte(b.String())
}

// escapeText escapes a TEXT value the way iCalendar requires.
func escapeText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(s)
}
