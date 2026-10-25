package widgets_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kinlight/internal/calendar"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// These tests call the Agenda directly, with no HTTP request, as the agent
// will. The fakes are the Today card tests' (see today_test.go).

func newAgenda(fetcher *fakeFetcher, links map[int64]string) (*widgets.Agenda, *memCalendars) {
	st := newMemCalendars()
	for id, link := range links {
		st.links[id] = link
	}
	return widgets.NewAgenda(fetcher, st, discardLogger), st
}

func TestAgendaNoCalendar(t *testing.T) {
	fetcher := &fakeFetcher{feed: dayFeed()}
	agenda, st := newAgenda(fetcher, nil)

	day, err := agenda.Day(context.Background(), alice, time.UTC, widgets.DayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if day.EmptyReason != widgets.EmptyNoCalendar || day.Today == nil || day.Tomorrow == nil || day.Provider != "" {
		t.Errorf("day = %+v, want no_calendar with empty lists", day)
	}
	if fetcher.fetches != 0 || st.puts != 0 {
		t.Errorf("fetches %d, remembered %d; want neither", fetcher.fetches, st.puts)
	}
}

func TestAgendaFixtureDay(t *testing.T) {
	denver, _ := time.LoadLocation("America/Denver")
	fetcher := &fakeFetcher{feed: dayFeed()}
	agenda, st := newAgenda(fetcher, map[int64]string{alice.ID: calLink})

	day, err := agenda.Day(context.Background(), alice, denver, widgets.DayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if day.Date != time.Now().In(denver).Format(time.DateOnly) || day.TimeZone != "America/Denver" || day.Provider != "Google Calendar" {
		t.Errorf("day = date %q, zone %q, provider %q", day.Date, day.TimeZone, day.Provider)
	}
	if got, want := strings.Join(describeEvents(day.Today), "|"), "all day Family day|07:00 Breakfast|10:00 Lunch call"; got != want {
		t.Errorf("today = %s, want %s", got, want)
	}
	if got, want := strings.Join(describeEvents(day.Tomorrow), "|"), "all day Family day|07:00 Breakfast|18:00 Recital"; got != want {
		t.Errorf("tomorrow = %s, want %s", got, want)
	}
	if day.Stale {
		t.Error("a day just fetched is stale")
	}
	if fetcher.links[0] != calLink {
		t.Errorf("fetched %v, want alice's link", fetcher.links)
	}
	if st.puts != 1 {
		t.Errorf("remembered %d times, want 1", st.puts)
	}
}

func TestAgendaRememberedDay(t *testing.T) {
	fetcher := &fakeFetcher{feed: dayFeed()}
	agenda, st := newAgenda(fetcher, map[int64]string{alice.ID: calLink})
	ctx := context.Background()

	first, err := agenda.Day(ctx, alice, time.UTC, widgets.DayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := agenda.Day(ctx, alice, time.UTC, widgets.DayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.fetches != 1 {
		t.Errorf("fetches = %d after asking twice, want 1", fetcher.fetches)
	}
	if len(again.Today) != len(first.Today) || !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("remembered day = %+v, want the first one", again)
	}

	// Remembered 20 minutes ago: given stale, still without fetching, until
	// the caller asks for a refresh.
	key := resultKey(alice.ID, "today")
	row := st.rows[key]
	row.UpdatedAt = time.Now().Add(-20 * time.Minute).Truncate(time.Second)
	st.rows[key] = row
	stale, err := agenda.Day(ctx, alice, time.UTC, widgets.DayOptions{})
	if err != nil || !stale.Stale || fetcher.fetches != 1 {
		t.Errorf("after 20 minutes: stale %v, fetches %d, err %v; want stale and no fetch", stale.Stale, fetcher.fetches, err)
	}
	refreshed, err := agenda.Day(ctx, alice, time.UTC, widgets.DayOptions{Refresh: true})
	if err != nil || refreshed.Stale || fetcher.fetches != 2 {
		t.Errorf("after a refresh: stale %v, fetches %d, err %v; want fresh and 2 fetches", refreshed.Stale, fetcher.fetches, err)
	}
	if _, err := agenda.Day(ctx, alice, time.UTC, widgets.DayOptions{Refresh: true}); err != nil || fetcher.fetches != 2 {
		t.Errorf("a second refresh within a minute fetched again (fetches %d, err %v)", fetcher.fetches, err)
	}
}

func TestAgendaLinkGone(t *testing.T) {
	fetcher := &fakeFetcher{err: fmt.Errorf("%w: HTTP 404", calendar.ErrLinkGone)}
	agenda, st := newAgenda(fetcher, map[int64]string{alice.ID: calLink})
	st.rows[resultKey(alice.ID, "today")] = store.CuratedResult{UserID: alice.ID, Kind: "today", Inputs: "old"}

	day, err := agenda.Day(context.Background(), alice, time.UTC, widgets.DayOptions{})
	if err != nil {
		t.Fatalf("err = %v; a link that stopped working is a result, not an error", err)
	}
	if day.EmptyReason != widgets.EmptyCalendarGone || day.Provider != "Google Calendar" {
		t.Errorf("day = %+v, want calendar_gone", day)
	}
	if _, ok := st.rows[resultKey(alice.ID, "today")]; ok {
		t.Error("the remembered events were kept after the link stopped working")
	}
}

func TestAgendaFetchFailureIsAStepError(t *testing.T) {
	fetcher := &fakeFetcher{err: fmt.Errorf("%w: HTTP 500", calendar.ErrUnavailable)}
	agenda, _ := newAgenda(fetcher, map[int64]string{alice.ID: calLink})

	_, err := agenda.Day(context.Background(), alice, time.UTC, widgets.DayOptions{})
	step, ok := errors.AsType[*widgets.StepError](err)
	if !ok || step.Step != widgets.StepCalendar {
		t.Fatalf("err = %v, want a calendar StepError", err)
	}
	if !errors.Is(err, calendar.ErrUnavailable) {
		t.Errorf("err = %v, want it to wrap calendar.ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), calSecret) {
		t.Errorf("err = %v, holds the calendar link", err)
	}
}
