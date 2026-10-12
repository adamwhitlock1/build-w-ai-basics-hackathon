package calendar

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

// Event is one occurrence of a calendar event. It deliberately has no
// description, attendees or links: those can be private, and the Today
// card doesn't show them.
type Event struct {
	Title    string
	Location string
	Start    time.Time // in the member's time zone
	End      time.Time // exclusive; equal to Start for an event with no length
	AllDay   bool
}

// occurrence is one start of a VEVENT, before it's turned into an Event.
type occurrence struct {
	start, end time.Time
	allDay     bool
}

// vevent is the part of a VEVENT this package uses, already parsed.
type vevent struct {
	uid          string
	title        string
	location     string
	cancelled    bool
	start        time.Time
	allDay       bool
	length       time.Duration
	rrule        string      // the RRULE value, or ""
	rdates       []time.Time // extra starts
	exdates      []time.Time // removed starts
	recurrenceID time.Time   // non-zero for a moved or edited occurrence of a series
}

// Between parses feed and returns the occurrences that overlap [from, to),
// with all-day events first, then by start time, then by title. Repeating
// events are expanded; occurrences removed from a series are left out,
// and ones moved or edited on their own replace the original. loc is the
// member's time zone: times are returned in it, all-day dates are read in
// it, and so are times with no zone or a zone Go doesn't know.
//
// An event that can't be read (no start time, say) is skipped rather than
// failing the whole calendar. A feed that isn't a calendar at all returns
// ErrNotCalendar.
func Between(feed []byte, from, to time.Time, loc *time.Location) ([]Event, error) {
	if !looksLikeCalendar(feed) {
		return nil, ErrNotCalendar
	}
	cal, err := ics.ParseCalendar(bytes.NewReader(feed))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotCalendar, err)
	}

	var series []vevent
	// overrides maps a series' UID to its moved or edited occurrences,
	// keyed by the start they replace (as a Unix time, so zones don't
	// matter when comparing).
	overrides := map[string]map[int64]vevent{}
	for _, ve := range cal.Events() {
		e, ok := readEvent(ve, loc)
		if !ok {
			continue
		}
		if !e.recurrenceID.IsZero() {
			if overrides[e.uid] == nil {
				overrides[e.uid] = map[int64]vevent{}
			}
			overrides[e.uid][e.recurrenceID.Unix()] = e
			continue
		}
		series = append(series, e)
	}

	var events []Event
	add := func(e vevent, o occurrence) {
		// An occurrence overlaps the window if it starts before the window
		// ends and ends after it starts. One with no length (a reminder at
		// 9:00) counts if it starts inside the window.
		overlaps := o.start.Before(to) && (o.end.After(from) || o.end.Equal(o.start) && !o.start.Before(from))
		if e.cancelled || !overlaps {
			return
		}
		events = append(events, Event{
			Title:    e.title,
			Location: e.location,
			Start:    o.start.In(loc),
			End:      o.end.In(loc),
			AllDay:   o.allDay,
		})
	}

	for _, e := range series {
		moved := overrides[e.uid]
		for _, start := range e.starts(from, to) {
			if o, ok := moved[start.Unix()]; ok {
				delete(moved, start.Unix()) // handled: don't add it again below
				add(o, o.occurrence(o.start))
				continue
			}
			add(e, e.occurrence(start))
		}
	}
	// Overrides whose original start falls outside the window can still be
	// moved into it (Thursday's practice moved to Friday, say).
	for _, moved := range overrides {
		for _, o := range moved {
			add(o, o.occurrence(o.start))
		}
	}

	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if a.AllDay != b.AllDay {
			return a.AllDay
		}
		if !a.Start.Equal(b.Start) {
			return a.Start.Before(b.Start)
		}
		return a.Title < b.Title
	})
	return events, nil
}

// occurrence returns the occurrence of e that starts at start.
func (e vevent) occurrence(start time.Time) occurrence {
	if e.allDay {
		// An all-day event covers whole days, whatever the clock does in
		// between, so its end is counted in days, not hours.
		days := int(e.length.Round(24*time.Hour) / (24 * time.Hour))
		if days < 1 {
			days = 1
		}
		return occurrence{start: start, end: start.AddDate(0, 0, days), allDay: true}
	}
	return occurrence{start: start, end: start.Add(e.length)}
}

// starts returns the starts of e's occurrences that could overlap
// [from, to): every start in [from - length, to). For an event that doesn't
// repeat that's at most its own start.
func (e vevent) starts(from, to time.Time) []time.Time {
	// Look back by the event's length, so an occurrence that began before
	// from and is still going on is found.
	after := from.Add(-e.length)
	if e.allDay {
		after = after.AddDate(0, 0, -1) // all-day lengths are in days; leave room for a DST hour
	}
	if e.rrule == "" && len(e.rdates) == 0 {
		if e.start.Before(after) || !e.start.Before(to) {
			return nil
		}
		return []time.Time{e.start}
	}

	set := &rrule.Set{}
	if e.rrule != "" {
		opt, err := rrule.StrToROption(e.rrule)
		if err != nil {
			return nil // a rule we can't read: skip the series, not the calendar
		}
		// Dtstart carries the event's own time zone, so a weekly 9 am
		// event stays at 9 am across daylight-saving changes.
		opt.Dtstart = e.start
		r, err := rrule.NewRRule(*opt)
		if err != nil {
			return nil
		}
		set.RRule(r)
	} else {
		set.DTStart(e.start)
		set.RDate(e.start) // with only RDATEs, the event's own start still counts
	}
	for _, t := range e.rdates {
		set.RDate(t)
	}
	for _, t := range e.exdates {
		set.ExDate(t)
	}
	var out []time.Time
	for _, t := range set.Between(after, to, true) {
		if t.Before(to) { // Between's inc applies to both ends; to is exclusive
			out = append(out, t)
		}
	}
	return out
}

// readEvent reads the parts of ve this package uses. ok is false for an
// event that can't be placed in time.
func readEvent(ve *ics.VEvent, loc *time.Location) (e vevent, ok bool) {
	e.uid = propValue(ve, ics.ComponentPropertyUniqueId)
	e.title = text(propValue(ve, ics.ComponentPropertySummary))
	e.location = text(propValue(ve, ics.ComponentPropertyLocation))
	e.cancelled = strings.EqualFold(propValue(ve, ics.ComponentPropertyStatus), "CANCELLED")

	startProp := ve.GetProperty(ics.ComponentPropertyDtStart)
	if startProp == nil {
		return e, false
	}
	starts, allDay, err := parseTimes(startProp, loc)
	if err != nil || len(starts) == 0 {
		return e, false
	}
	e.start, e.allDay = starts[0], allDay

	switch {
	case ve.GetProperty(ics.ComponentPropertyDtEnd) != nil:
		ends, _, err := parseTimes(ve.GetProperty(ics.ComponentPropertyDtEnd), loc)
		if err == nil && len(ends) > 0 && ends[0].After(e.start) {
			e.length = ends[0].Sub(e.start)
		}
	case ve.GetProperty(ics.ComponentPropertyDuration) != nil:
		if d, err := parseDuration(propValue(ve, ics.ComponentPropertyDuration)); err == nil && d > 0 {
			e.length = d
		}
	}
	if e.allDay && e.length == 0 {
		e.length = 24 * time.Hour // RFC 5545: a DATE start with no end lasts one day
	}

	e.rrule = propValue(ve, ics.ComponentPropertyRrule)
	for _, p := range ve.GetProperties(ics.ComponentPropertyRdate) {
		if ts, _, err := parseTimes(p, loc); err == nil {
			e.rdates = append(e.rdates, ts...)
		}
	}
	for _, p := range ve.GetProperties(ics.ComponentPropertyExdate) {
		if ts, _, err := parseTimes(p, loc); err == nil {
			e.exdates = append(e.exdates, ts...)
		}
	}
	if p := ve.GetProperty(ics.ComponentPropertyRecurrenceId); p != nil {
		if ts, _, err := parseTimes(p, loc); err == nil && len(ts) > 0 {
			e.recurrenceID = ts[0]
		}
	}
	return e, true
}

func propValue(ve *ics.VEvent, p ics.ComponentProperty) string {
	if prop := ve.GetProperty(p); prop != nil {
		return strings.TrimSpace(prop.Value)
	}
	return ""
}

// text undoes iCalendar's TEXT escaping (\, \; \n \\) and puts a line
// break in a location or title on one line.
func text(s string) string {
	s = ics.FromText(s)
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", ", ")), " ")
}

// parseTimes reads a date-time property, which may hold several
// comma-separated values (as EXDATE and RDATE can). allDay is true when the
// values are dates, not date-times.
//
//   - A date (VALUE=DATE, or 8 digits) is midnight of that day in loc.
//   - A time ending in Z is UTC.
//   - A time with TZID is in that zone, or in loc if Go doesn't know the zone
//     (Outlook's Windows zone names, say).
//   - A time with neither is "floating": the same clock time wherever you
//     are, so it's read in loc.
func parseTimes(p *ics.IANAProperty, loc *time.Location) (times []time.Time, allDay bool, err error) {
	zone := loc
	if tzid := firstParam(p, "TZID"); tzid != "" {
		if z, err := time.LoadLocation(strings.Trim(tzid, `"`)); err == nil {
			zone = z
		}
	}
	isDate := strings.EqualFold(firstParam(p, "VALUE"), "DATE")
	for _, v := range strings.Split(p.Value, ",") {
		v = strings.TrimSpace(v)
		var t time.Time
		switch {
		case isDate || len(v) == len("20060102"):
			t, err = time.ParseInLocation("20060102", v, loc)
			allDay = true
		case strings.HasSuffix(v, "Z"):
			t, err = time.Parse("20060102T150405Z", v)
		default:
			t, err = time.ParseInLocation("20060102T150405", v, zone)
		}
		if err != nil {
			return nil, false, err
		}
		times = append(times, t)
	}
	return times, allDay, nil
}

func firstParam(p *ics.IANAProperty, name string) string {
	for k, vs := range p.ICalParameters {
		if strings.EqualFold(k, name) && len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

// parseDuration reads an iCalendar DURATION such as PT45M, PT1H30M, P1D or
// P1W. Months and years aren't allowed in iCalendar durations.
func parseDuration(s string) (time.Duration, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	if !strings.HasPrefix(s, "P") {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	s = s[1:]
	var d time.Duration
	inTime := false
	num := 0
	digits := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num = num*10 + int(r-'0')
			digits = true
			continue
		case r == 'T':
			inTime = true
			continue
		}
		if !digits {
			return 0, fmt.Errorf("bad duration %q", s)
		}
		n := time.Duration(num)
		switch {
		case r == 'W' && !inTime:
			d += n * 7 * 24 * time.Hour
		case r == 'D' && !inTime:
			d += n * 24 * time.Hour
		case r == 'H' && inTime:
			d += n * time.Hour
		case r == 'M' && inTime:
			d += n * time.Minute
		case r == 'S' && inTime:
			d += n * time.Second
		default:
			return 0, fmt.Errorf("bad duration %q", s)
		}
		num, digits = 0, false
	}
	if neg {
		d = -d
	}
	return d, nil
}
