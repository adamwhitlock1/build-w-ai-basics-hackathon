package store_test

import (
	"context"
	"errors"
	"kinlight/internal/store"
	"strings"
	"sync"
	"testing"
)

func TestChoreStoreLifecycleAndRaces(t *testing.T) {
	s := openTestStore(t)
	f := newChatFamily(t, s)
	ctx := context.Background()
	day := func() (string, error) { return "2026-10-25", nil }
	mutate := func(id int64, op, title string, done bool) error {
		return s.MutateChore(ctx, f.parent.FamilyID, f.kid.ID, f.parent.ID, id, op, title, done, day)
	}
	for _, title := range []string{" ", strings.Repeat("🌞", 121)} {
		if err := mutate(0, "create", title, false); !errors.Is(err, store.ErrChoreTitle) {
			t.Fatal(err)
		}
	}
	if err := mutate(0, "create", "  Feed dog  ", false); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, "2026-10-25")
	if err != nil || len(items) != 1 || items[0].Title != "Feed dog" {
		t.Fatalf("%+v %v", items, err)
	}
	id := items[0].ID
	for range 2 {
		if err := mutate(id, "completion", "", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := mutate(id, "rename", strings.Repeat("🌞", 120), false); err != nil {
		t.Fatal(err)
	}
	summary, err := s.ChoreSummary(ctx, f.parent.FamilyID, "2026-10-25")
	if err != nil || len(summary) != 2 || summary[0].TotalCount != 0 || summary[1].CompletedCount != 1 {
		t.Fatalf("%+v %v", summary, err)
	}
	tomorrow, _ := s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, "2026-10-26")
	if tomorrow[0].Completed {
		t.Fatal("yesterday counted today")
	}
	if err := s.MutateChore(ctx, f.outsider.FamilyID, f.kid.ID, f.outsider.ID, id, "rename", "intruder", false, day); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 50)
	for range 40 {
		wg.Go(func() {
			err := mutate(0, "create", "Daily", false)
			if err != nil && !errors.Is(err, store.ErrChoreLimit) {
				failures <- err
			}
		})
	}
	wg.Wait()
	items, _ = s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, "2026-10-25")
	if len(items) != 30 || items[0].ID != id || !items[0].Completed {
		t.Fatalf("cap/order/rename: %+v", items)
	}
	for range 10 {
		wg.Go(func() {
			err := mutate(id, "completion", "", false)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				failures <- err
			}
		})
	}
	wg.Go(func() {
		if err := mutate(id, "remove", "", false); err != nil {
			failures <- err
		}
	})
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if err := mutate(id, "completion", "", true); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	items, _ = s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, "2026-10-25")
	if len(items) != 29 {
		t.Fatal("archive resurrected")
	}
}

func TestChoreConflictingWritesAndHistory(t *testing.T) {
	s := openTestStore(t)
	f := newChatFamily(t, s)
	ctx := context.Background()
	date := "2026-10-24"
	mutate := func(id int64, op string, done bool) error {
		return s.MutateChore(ctx, f.parent.FamilyID, f.kid.ID, f.kid.ID, id, op, "Daily", done, func() (string, error) { return date, nil })
	}
	if err := mutate(0, "create", false); err != nil {
		t.Fatal(err)
	}
	items, _ := s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, date)
	id := items[0].ID
	if err := mutate(id, "completion", true); err != nil {
		t.Fatal(err)
	}
	date = "2026-10-25"
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for n := range 20 {
		wg.Go(func() { errs <- mutate(id, "completion", n%2 == 0) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := mutate(id, "completion", false); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, date)
	if items[0].Completed {
		t.Fatal("last write lost")
	}
	items, _ = s.ListChores(ctx, f.parent.FamilyID, f.kid.ID, "2026-10-24")
	if !items[0].Completed {
		t.Fatal("history cleared")
	}
}
