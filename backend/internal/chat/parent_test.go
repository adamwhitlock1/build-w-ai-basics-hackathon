package chat_test

import (
	"context"
	"errors"
	"testing"

	"porchlight/internal/aikit"
	"porchlight/internal/chat"
)

func TestParentReview(t *testing.T) {
	ctx := context.Background()
	f := newFamily(t)
	agent := &fakeAgent{reply: "ok"}
	svc := newService(f, agent)

	// Jordan: one conversation with a medium flag, then a crisis.
	c, err := svc.Start(ctx, f.kid, aikit.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	agent.verdict = aikit.Verdict{Flag: true, Category: aikit.CategoryStrangers, Severity: "medium", Reason: "Shared their school."}
	if _, _, _, err := send(t, svc, f.kid, c.ID, "i told a guy online my school"); err != nil {
		t.Fatal(err)
	}
	agent.verdict = aikit.Verdict{Flag: true, Category: aikit.CategorySelfHarm, Severity: "high", Crisis: true, Reason: "Said they want to disappear."}
	if _, _, _, err := send(t, svc, f.kid, c.ID, "i want to disappear"); err != nil {
		t.Fatal(err)
	}

	t.Run("kids can't use parent tools", func(t *testing.T) {
		if _, err := svc.Reviews(ctx, f.kid); !errors.Is(err, chat.ErrNotParent) {
			t.Errorf("Reviews err = %v, want ErrNotParent", err)
		}
		if _, err := svc.Alerts(ctx, f.kid); !errors.Is(err, chat.ErrNotParent) {
			t.Errorf("Alerts err = %v, want ErrNotParent", err)
		}
		if _, err := svc.ListFor(ctx, f.kid2, f.kid); !errors.Is(err, chat.ErrNotParent) {
			t.Errorf("ListFor err = %v, want ErrNotParent", err)
		}
		if err := svc.Delete(ctx, f.kid, f.kid, c.ID); !errors.Is(err, chat.ErrNotParent) {
			t.Errorf("Delete err = %v, want ErrNotParent", err)
		}
	})

	t.Run("a parent isn't a kid to review", func(t *testing.T) {
		if _, err := svc.ListFor(ctx, f.parent, f.parent2); !errors.Is(err, chat.ErrNoSuchKid) {
			t.Errorf("ListFor a parent err = %v, want ErrNoSuchKid", err)
		}
		if _, err := svc.ListFor(ctx, f.parent, f.otherKid); !errors.Is(err, chat.ErrNoSuchKid) {
			t.Errorf("ListFor another family's kid err = %v, want ErrNoSuchKid", err)
		}
	})

	t.Run("lists and transcripts", func(t *testing.T) {
		list, err := svc.ListFor(ctx, f.parent2, f.kid)
		if err != nil || len(list) != 1 || list[0].UnreviewedFlags != 2 {
			t.Fatalf("ListFor = %+v, %v; want one conversation with 2 unreviewed flags", list, err)
		}
		tr, err := svc.Transcript(ctx, f.parent, f.kid, c.ID)
		if err != nil || len(tr.Messages) != 4 || len(tr.Flags) != 2 {
			t.Fatalf("Transcript = %d messages, %d flags, %v; want 4 and 2", len(tr.Messages), len(tr.Flags), err)
		}
		if _, err := svc.Transcript(ctx, f.parent, f.kid2, c.ID); !errors.Is(err, chat.ErrNoSuchChat) {
			t.Errorf("Transcript under the wrong kid err = %v, want ErrNoSuchChat", err)
		}
	})

	t.Run("queue, alerts and review", func(t *testing.T) {
		queue, err := svc.Reviews(ctx, f.parent)
		if err != nil || len(queue) != 2 || !queue[0].Crisis {
			t.Fatalf("Reviews = %+v, %v; want the crisis first of 2", queue, err)
		}
		alerts, err := svc.Alerts(ctx, f.parent2)
		if err != nil || alerts.Unreviewed != 2 || len(alerts.Crises) != 1 || alerts.Crises[0].MemberID != f.kid.ID {
			t.Fatalf("Alerts = %+v, %v; want 2 unreviewed and Jordan's crisis", alerts, err)
		}

		if _, err := svc.MarkReviewed(ctx, f.parent, queue[0].ID); err != nil {
			t.Fatal(err)
		}
		alerts, _ = svc.Alerts(ctx, f.parent2)
		if alerts.Unreviewed != 1 || len(alerts.Crises) != 0 {
			t.Errorf("Alerts after review = %+v, want 1 unreviewed and no crisis", alerts)
		}
		queue, _ = svc.Reviews(ctx, f.parent2)
		if last := queue[len(queue)-1]; last.ReviewedBy != f.parent.ID {
			t.Errorf("the reviewed flag = %+v, want it last, reviewed by Maya", last)
		}
	})

	t.Run("another family", func(t *testing.T) {
		other := f.otherParent
		queue, _ := svc.Reviews(ctx, other)
		if len(queue) != 0 {
			t.Errorf("another family's queue = %+v, want empty", queue)
		}
		flags, _ := f.st.ReviewQueue(ctx, f.kid.FamilyID, 10)
		if _, err := svc.MarkReviewed(ctx, other, flags[0].ID); !errors.Is(err, chat.ErrNoSuchFlag) {
			t.Errorf("another family's review err = %v, want ErrNoSuchFlag", err)
		}
		if err := svc.Delete(ctx, other, f.kid, c.ID); !errors.Is(err, chat.ErrNoSuchKid) {
			t.Errorf("another family's delete err = %v, want ErrNoSuchKid", err)
		}
	})

	t.Run("delete removes flags from the queue", func(t *testing.T) {
		if err := svc.Delete(ctx, f.parent, f.kid, c.ID); err != nil {
			t.Fatal(err)
		}
		if list, _ := svc.List(ctx, f.kid); len(list) != 0 {
			t.Errorf("Jordan still has %d conversations", len(list))
		}
		if queue, _ := svc.Reviews(ctx, f.parent); len(queue) != 0 {
			t.Errorf("queue after delete = %+v, want empty", queue)
		}
		if err := svc.Delete(ctx, f.parent, f.kid, c.ID); !errors.Is(err, chat.ErrNoSuchChat) {
			t.Errorf("second delete err = %v, want ErrNoSuchChat", err)
		}
	})
}
