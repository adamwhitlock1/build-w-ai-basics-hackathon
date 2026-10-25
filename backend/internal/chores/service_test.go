package chores

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kinlight/internal/store"
)

func TestHouseholdDays(t *testing.T) {
	for _, tt := range []struct {
		instant, date string
		hours         int
	}{
		{"2026-03-08T07:00:00Z", "2026-03-08", 23}, {"2026-11-01T06:00:00Z", "2026-11-01", 25}, {"2026-10-26T05:59:59Z", "2026-10-25", 0},
	} {
		t.Run(tt.date, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tt.instant)
			s := New(nil, func() time.Time { return now.In(time.FixedZone("travel", 9*3600)) })
			d := s.day()
			if d.Date != tt.date || d.TimeZone != TimeZone {
				t.Fatal(d)
			}
			if tt.hours > 0 && d.NextResetAt.Sub(now) != time.Duration(tt.hours)*time.Hour {
				t.Fatal(d)
			}
		})
	}
}
func TestServiceRolloverAndScope(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	parent, err := st.CreateFamilyWithAdmin(ctx, "parent@test", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 5, 59, 59, 0, time.UTC)
	s := New(st, func() time.Time { return now })
	list, err := s.Mutate(ctx, parent, parent, 0, "create", "Daily", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id := list.Items[0].ID
	date := list.Date
	if _, err = s.Mutate(ctx, parent, parent, id, "completion", "", date, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err = s.Mutate(ctx, parent, parent, id, "completion", "", date, false); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
	list, err = s.List(ctx, parent, parent)
	if err != nil || list.CompletedCount != 0 {
		t.Fatal(list, err)
	}
	old, _ := st.ListChores(ctx, parent.FamilyID, parent.ID, date)
	if !old[0].Completed {
		t.Fatal("stale write changed yesterday")
	}
	now = now.AddDate(0, 0, 4)
	s = New(st, func() time.Time { return now })
	list, _ = s.List(ctx, parent, parent)
	if list.CompletedCount != 0 {
		t.Fatal("restart reset failed")
	}
	kid := parent
	kid.IsAdmin = false
	if _, err = s.Mutate(ctx, kid, kid, 0, "create", "No", "", false); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	other := parent
	other.FamilyID++
	if _, err = s.List(ctx, parent, other); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}
