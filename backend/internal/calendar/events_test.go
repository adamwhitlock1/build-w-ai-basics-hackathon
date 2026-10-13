package calendar

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// day returns the events in testdata/name on date (YYYY-MM-DD) in zone,
// each written "15:04-15:04 Title @ Location" (or "all day Title").
func day(t *testing.T, name, date, zone string) []string {
	t.Helper()
	feed, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	from, err := time.ParseInLocation(time.DateOnly, date, loc)
	if err != nil {
		t.Fatal(err)
	}
	events, err := Between(feed, from, from.AddDate(0, 0, 1), loc)
	if err != nil {
		t.Fatalf("Between: %v", err)
	}
	out := make([]string, len(events))
	for i, e := range events {
		var s string
		if e.AllDay {
			s = "all day " + e.Title
		} else {
			s = fmt.Sprintf("%s-%s %s", e.Start.Format("Jan 2 15:04"), e.End.Format("Jan 2 15:04"), e.Title)
		}
		if e.Location != "" {
			s += " @ " + e.Location
		}
		out[i] = s
	}
	return out
}

func TestBetween(t *testing.T) {
	tests := []struct {
		name, feed, date, zone string
		want                   []string
	}{
		{
			name: "a normal Wednesday: all-day first, an event still going on, time zones, cancelled left out",
			feed: "google.ics", date: "2026-10-21", zone: "America/Denver",
			want: []string{
				"all day Fall break",
				"Oct 20 17:00-Oct 22 12:00 Camping trip",
				"Oct 21 07:00-Oct 21 07:30 Video call with Grandma", // 9 am in New York
				"Oct 21 14:00-Oct 21 14:30 Library pickup",          // 20:00 UTC
				"Oct 21 15:30-Oct 21 16:30 Dentist @ Smile Dental, 123 Main St, Denver",
			},
		},
		{
			name: "weekly practice on a Thursday",
			feed: "google.ics", date: "2026-10-08", zone: "America/Denver",
			want: []string{"Oct 8 18:00-Oct 8 19:30 Soccer practice @ Riverside Park, Field 3"},
		},
		{
			name: "weekly practice on a Tuesday",
			feed: "google.ics", date: "2026-10-13", zone: "America/Denver",
			want: []string{"Oct 13 18:00-Oct 13 19:30 Soccer practice @ Riverside Park, Field 3"},
		},
		{
			name: "an occurrence removed with EXDATE",
			feed: "google.ics", date: "2026-10-15", zone: "America/Denver",
			want: []string{},
		},
		{
			name: "a moved occurrence shows at its new time only",
			feed: "google.ics", date: "2026-10-22", zone: "America/Denver",
			want: []string{
				"all day Fall break",
				"Oct 20 17:00-Oct 22 12:00 Camping trip",
				"Oct 22 19:00-Oct 22 20:30 Soccer practice (late start) @ Riverside Park, Field 3",
			},
		},
		{
			name: "an occurrence cancelled with a RECURRENCE-ID override",
			feed: "google.ics", date: "2026-10-29", zone: "America/Denver",
			want: []string{},
		},
		{
			name: "weekly 9 am before daylight saving ends",
			feed: "google.ics", date: "2026-10-26", zone: "America/Denver",
			want: []string{"Oct 26 09:00-Oct 26 09:45 Piano lesson"},
		},
		{
			name: "weekly 9 am stays at 9 am after daylight saving ends (Nov 1)",
			feed: "google.ics", date: "2026-11-02", zone: "America/Denver",
			want: []string{"Nov 2 09:00-Nov 2 09:45 Piano lesson"},
		},
		{
			name: "the same day seen from New York",
			feed: "google.ics", date: "2026-10-21", zone: "America/New_York",
			want: []string{
				"all day Fall break",
				"Oct 20 19:00-Oct 22 14:00 Camping trip",
				"Oct 21 09:00-Oct 21 09:30 Video call with Grandma",
				"Oct 21 16:00-Oct 21 16:30 Library pickup",
				"Oct 21 17:30-Oct 21 18:30 Dentist @ Smile Dental, 123 Main St, Denver",
			},
		},
		{
			name: "iCloud: yearly birthday, daily count, folded title",
			feed: "icloud.ics", date: "2026-10-21", zone: "America/Los_Angeles",
			want: []string{
				"all day Mom's birthday",
				"Oct 21 07:00-Oct 21 08:30 Swim team morning practice at the community pool before school starts",
				"Oct 21 12:00-Oct 21 13:00 Lunch with Alex",
			},
		},
		{
			name: "iCloud: RDATE adds a day, COUNT has ended",
			feed: "icloud.ics", date: "2026-10-24", zone: "America/Los_Angeles",
			want: []string{"Oct 24 12:00-Oct 24 13:00 Lunch with Alex"},
		},
		{
			name: "iCloud: a day between RDATEs",
			feed: "icloud.ics", date: "2026-10-22", zone: "America/Los_Angeles",
			want: []string{"Oct 22 07:00-Oct 22 08:30 Swim team morning practice at the community pool before school starts"},
		},
		{
			name: "floating time and an unknown zone name use the member's zone",
			feed: "floating.ics", date: "2026-10-21", zone: "America/Denver",
			want: []string{
				"Oct 21 08:00-Oct 21 09:00 School assembly @ Gym",
				"Oct 21 16:00-Oct 21 17:00 Art club",
			},
		},
		{
			name: "floating time seen from New York is still 8 am",
			feed: "floating.ics", date: "2026-10-21", zone: "America/New_York",
			want: []string{
				"Oct 21 08:00-Oct 21 09:00 School assembly @ Gym",
				"Oct 21 16:00-Oct 21 17:00 Art club",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := day(t, tt.feed, tt.date, tt.zone)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("events on %s in %s:\n got: %q\nwant: %q", tt.date, tt.zone, got, tt.want)
			}
		})
	}
}

func TestBetweenSkipsUnreadableEvents(t *testing.T) {
	for _, e := range day(t, "google.ics", "2026-10-21", "America/Denver") {
		if strings.Contains(e, "No start time") {
			t.Errorf("an event with no start was listed: %s", e)
		}
	}
}

func TestBetweenNotACalendar(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, loc)
	for _, feed := range []string{"<html>hello</html>", ""} {
		if _, err := Between([]byte(feed), now, now.AddDate(0, 0, 1), loc); !errors.Is(err, ErrNotCalendar) {
			t.Errorf("Between(%q) error = %v, want ErrNotCalendar", feed, err)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"PT45M":   45 * time.Minute,
		"PT1H30M": 90 * time.Minute,
		"P1D":     24 * time.Hour,
		"P1W":     7 * 24 * time.Hour,
		"P1DT2H":  26 * time.Hour,
		"-PT15M":  -15 * time.Minute,
	}
	for in, want := range tests {
		if got, err := parseDuration(in); err != nil || got != want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1H", "PTH", "P1H"} {
		if _, err := parseDuration(bad); err == nil {
			t.Errorf("parseDuration(%q) succeeded, want an error", bad)
		}
	}
}
