package calendar

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fixtureDays parses the fixture feed name, built at now, and returns its
// events today and tomorrow, each written "15:04-15:04 Title" (or "all day
// Title").
func fixtureDays(t *testing.T, name string, now time.Time) (today, tomorrow []string) {
	t.Helper()
	feed, ok := fixtureFeed(name, now)
	if !ok {
		t.Fatalf("no fixture %q", name)
	}
	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	list := func(from time.Time) []string {
		events, err := Between(feed, from, from.AddDate(0, 0, 1), loc)
		if err != nil {
			t.Fatalf("Between(%s): %v", name, err)
		}
		out := []string{}
		for _, e := range events {
			if e.AllDay {
				out = append(out, "all day "+e.Title)
				continue
			}
			out = append(out, fmt.Sprintf("%s-%s %s", e.Start.Format("15:04"), e.End.Format("15:04"), e.Title))
		}
		return out
	}
	return list(start), list(start.AddDate(0, 0, 1))
}

func denver(t *testing.T, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	now, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return now
}

func TestFixtureFeeds(t *testing.T) {
	now := denver(t, "2026-10-14 14:00") // a Wednesday afternoon
	tests := []struct {
		name            string
		today, tomorrow []string
	}{
		{
			name: "maya",
			today: []string{
				"all day Recycling day",
				"12:30-13:30 Coffee with Priya", // already ended
				"15:30-16:30 Dentist appointment",
			},
			tomorrow: []string{"18:30-19:30 Book club", "19:30-20:30 PTA meeting"},
		},
		{
			name: "sam",
			// Today's standup moved from 9:00 to 11:00; tomorrow's is back at 9:00.
			today:    []string{"11:00-11:15 Team standup", "15:30-16:30 Pick up Lily"},
			tomorrow: []string{"09:00-09:15 Team standup", "18:30-19:15 Evening run"},
		},
		{
			name: "jordan",
			// Today's band practice was deleted; tomorrow's is still on.
			today:    []string{"12:30-13:30 Study group"},
			tomorrow: []string{"all day Science test", "16:00-17:00 Band practice", "18:30-20:00 Soccer game"},
		},
		{
			name:     "alex",
			today:    []string{"15:30-16:30 Call with the landlord"},
			tomorrow: []string{"18:00-18:45 Riley's swim lesson"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			today, tomorrow := fixtureDays(t, tt.name, now)
			if !reflect.DeepEqual(today, tt.today) {
				t.Errorf("today = %q\nwant    %q", today, tt.today)
			}
			if !reflect.DeepEqual(tomorrow, tt.tomorrow) {
				t.Errorf("tomorrow = %q\nwant       %q", tomorrow, tt.tomorrow)
			}
		})
	}

	if _, ok := fixtureFeed("nobody", now); ok {
		t.Error(`fixtureFeed("nobody") reported a feed`)
	}
}

// Near midnight the clock-relative events are left out rather than spilling
// into another day, and every feed still parses.
func TestFixtureFeedsNearMidnight(t *testing.T) {
	for _, clock := range []string{"2026-10-14 00:20", "2026-10-14 23:40"} {
		now := denver(t, clock)
		for _, name := range []string{"maya", "sam", "jordan", "alex"} {
			feed, _ := fixtureFeed(name, now)
			loc := now.Location()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			// Everything in the feed, a week either side.
			events, err := Between(feed, today.AddDate(0, 0, -7), today.AddDate(0, 0, 7), loc)
			if err != nil {
				t.Fatalf("%s at %s: Between: %v", name, clock, err)
			}
			for _, e := range events {
				// Repeating events go back a week by design; the one-off
				// events must all be today or tomorrow.
				if e.Title == "Team standup" || e.Title == "Band practice" {
					continue
				}
				if e.Start.Before(today) || !e.End.Before(today.AddDate(0, 0, 2).Add(time.Second)) {
					t.Errorf("%s at %s: %q runs %s to %s, outside today and tomorrow", name, clock, e.Title, e.Start, e.End)
				}
			}
		}
	}
	// Just after midnight there's nothing that has already ended.
	today, _ := fixtureDays(t, "maya", denver(t, "2026-10-14 00:20"))
	if strings.Contains(strings.Join(today, "\n"), "Coffee") {
		t.Errorf("at 00:20 Maya's today has the ended event: %q", today)
	}
	// Late in the evening there's nothing still to come.
	today, _ = fixtureDays(t, "maya", denver(t, "2026-10-14 23:40"))
	if strings.Contains(strings.Join(today, "\n"), "Dentist") {
		t.Errorf("at 23:40 Maya's today has the later event: %q", today)
	}
}

// recordingFetcher stands in for the real fetcher and records what it's asked.
type recordingFetcher struct{ links []string }

func (r *recordingFetcher) Fetch(_ context.Context, link string) ([]byte, error) {
	r.links = append(r.links, link)
	return []byte(tinyFeed), nil
}

func TestFixtureFetcher(t *testing.T) {
	now := denver(t, "2026-10-14 14:00")
	next := &recordingFetcher{}
	f := NewFixtureFetcher(next, func() time.Time { return now })
	ctx := context.Background()

	for _, link := range []string{FixtureLink("maya"), "webcal://fixtures.porchlight.test/calendars/sam.ics"} {
		feed, err := f.Fetch(ctx, link)
		if err != nil {
			t.Fatalf("Fetch(%s): %v", link, err)
		}
		if !looksLikeCalendar(feed) || strings.Contains(string(feed), "Team standup") != strings.Contains(link, "sam") {
			t.Errorf("Fetch(%s) returned the wrong feed:\n%s", link, feed)
		}
	}
	if len(next.links) != 0 {
		t.Fatalf("fixture links reached the real fetcher: %q", next.links)
	}

	for _, link := range []string{
		"https://fixtures.porchlight.test/calendars/nobody.ics",
		"https://fixtures.porchlight.test/other/maya.ics",
		"https://fixtures.porchlight.test/calendars/maya.txt",
	} {
		_, err := f.Fetch(ctx, link)
		if !errors.Is(err, ErrLinkGone) {
			t.Errorf("Fetch(%s) = %v, want ErrLinkGone", link, err)
		}
		if err != nil && strings.Contains(err.Error(), "fixtures.porchlight.test") {
			t.Errorf("Fetch(%s) error contains the link: %v", link, err)
		}
	}

	real := []string{"https://calendar.google.com" + secretPath, "https://127.0.0.1/x.ics"}
	for _, link := range real {
		if _, err := f.Fetch(ctx, link); err != nil {
			t.Fatalf("Fetch(%s): %v", link, err)
		}
	}
	if !reflect.DeepEqual(next.links, real) {
		t.Errorf("real fetcher got %q, want %q", next.links, real)
	}

	if _, err := f.Fetch(ctx, "ftp://fixtures.porchlight.test/calendars/maya.ics"); !errors.Is(err, ErrInvalidLink) {
		t.Errorf("an ftp fixture link gave %v, want ErrInvalidLink", err)
	}
}

// The clock-relative events land on quarter hours, and are still in the past
// and the future.
func TestFixtureFeedsQuarterHours(t *testing.T) {
	today, _ := fixtureDays(t, "maya", denver(t, "2026-10-14 14:23"))
	want := []string{"all day Recycling day", "12:45-13:45 Coffee with Priya", "15:45-16:45 Dentist appointment"}
	if !reflect.DeepEqual(today, want) {
		t.Errorf("at 14:23 today = %q\nwant              %q", today, want)
	}
}
