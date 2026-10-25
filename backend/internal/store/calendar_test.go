package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"kinlight/internal/store"
)

func TestCalendarLink(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	u := signUp(t, s, "parent@example.com")
	const link = "https://calendar.google.com/calendar/ical/x/private-y/basic.ics"

	// No profile yet: no link.
	if got, err := s.GetCalendarLink(ctx, u.ID); err != nil || got != "" {
		t.Fatalf("GetCalendarLink before any profile = %q, %v; want \"\"", got, err)
	}

	// Setting the link keeps interests and location.
	if err := s.UpsertMemberProfile(ctx, store.MemberProfile{UserID: u.ID, Interests: "space", Location: "Denver, CO"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCalendarLink(ctx, u.ID, link); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetMemberProfile(ctx, u.ID)
	if err != nil || p.Interests != "space" || p.Location != "Denver, CO" {
		t.Errorf("profile after SetCalendarLink = %+v, %v; want interests and location kept", p, err)
	}

	// Saving the profile keeps the link.
	if err := s.UpsertMemberProfile(ctx, store.MemberProfile{UserID: u.ID, Interests: "soccer", Location: ""}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetCalendarLink(ctx, u.ID); err != nil || got != link {
		t.Errorf("GetCalendarLink after a profile save = %q, %v; want the link", got, err)
	}

	// Removing it.
	if err := s.SetCalendarLink(ctx, u.ID, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetCalendarLink(ctx, u.ID); got != "" {
		t.Errorf("GetCalendarLink after removing = %q, want \"\"", got)
	}

	// A member with no profile row can get a link too.
	kid := signUp(t, s, "other@example.com")
	if err := s.SetCalendarLink(ctx, kid.ID, link); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetCalendarLink(ctx, kid.ID); got != link {
		t.Errorf("GetCalendarLink = %q, want the link", got)
	}
}

func TestDeleteCuratedResult(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	u := signUp(t, s, "parent@example.com")
	now := time.Now().Truncate(time.Second)
	for _, kind := range []string{"today", "articles"} {
		if err := s.PutCuratedResult(ctx, store.CuratedResult{UserID: u.ID, Kind: kind, Inputs: "x", Payload: json.RawMessage(`{}`), UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteCuratedResult(ctx, u.ID, "today"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetCuratedResult(ctx, u.ID, "today"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("today result after delete: err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetCuratedResult(ctx, u.ID, "articles"); err != nil {
		t.Errorf("articles result was deleted too: %v", err)
	}
	// Deleting what isn't there is fine.
	if err := s.DeleteCuratedResult(ctx, u.ID, "today"); err != nil {
		t.Errorf("second delete: %v", err)
	}
}
