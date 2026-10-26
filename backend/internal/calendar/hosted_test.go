package calendar

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestHostedDemoNeverFetchesExternal(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	f := NewHostedDemoFetcher(func() time.Time { return now })
	for _, link := range []string{"https://example.com/private.ics", "https://127.0.0.1/calendar", "https://fixtures.kinlight.test/calendars/unknown.ics"} {
		if _, err := f.Fetch(context.Background(), link); err == nil {
			t.Fatalf("accepted %s", link)
		}
	}
	data, err := f.Fetch(context.Background(), FixtureLink("maya"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "20261101") {
		t.Fatal("not current date")
	}
}
