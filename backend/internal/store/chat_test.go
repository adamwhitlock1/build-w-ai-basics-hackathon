package store_test

import (
	"context"
	"errors"
	"testing"

	"kinlight/internal/store"
)

// chatFamily is a parent and a kid in one family, and a parent in another.
type chatFamily struct {
	parent, kid, outsider store.User
}

func newChatFamily(t *testing.T, s *store.Store) chatFamily {
	t.Helper()
	ctx := context.Background()
	parent := signUp(t, s, "parent@example.com")
	if _, _, err := s.InviteToFamily(ctx, invite(parent.FamilyID, "kid@example.com", "kid-token", false)); err != nil {
		t.Fatal(err)
	}
	kid, err := s.AcceptInvite(ctx, "kid-token", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	return chatFamily{parent: parent, kid: kid, outsider: signUp(t, s, "other@example.com")}
}

// addFlagged adds a kid message to c and flags it.
func addFlagged(t *testing.T, s *store.Store, c store.Conversation, text, severity string, crisis bool) store.ChatFlag {
	t.Helper()
	ctx := context.Background()
	m, err := s.AddChatMessage(ctx, c.ID, store.RoleKid, text)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.AddChatFlag(ctx, store.ChatFlag{
		MessageID: m.ID, ConversationID: c.ID, FamilyID: c.FamilyID, MemberID: c.MemberID,
		Category: "other", Severity: severity, Crisis: crisis, Reason: "because",
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestConversations(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)

	first, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeHomework)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.Mode != store.ModeHomework || first.Title != "" || first.CreatedAt.IsZero() {
		t.Errorf("created = %+v", first)
	}
	second, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, "chit-chat"); err == nil {
		t.Error("an unknown mode was accepted")
	}

	// A message in the first conversation makes it the most recent.
	for _, text := range []string{"one", "two", "three"} {
		if _, err := s.AddChatMessage(ctx, first.ID, store.RoleKid, text); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetConversationTitle(ctx, first.ID, "Counting"); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListConversations(ctx, fam.kid.FamilyID, fam.kid.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[0].Title != "Counting" || list[1].ID != second.ID {
		t.Errorf("list = %+v, want the first conversation (titled) then the second", list)
	}

	recent, err := s.RecentChatMessages(ctx, first.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].Text != "two" || recent[1].Text != "three" {
		t.Errorf("recent = %+v, want two, three", recent)
	}
	all, err := s.ListChatMessages(ctx, first.ID)
	if err != nil || len(all) != 3 || all[0].Text != "one" {
		t.Errorf("all messages = %+v, %v", all, err)
	}

	// Scoping: the wrong member or family finds nothing.
	for _, tt := range []struct {
		name           string
		family, member int64
	}{
		{"the parent", fam.parent.FamilyID, fam.parent.ID},
		{"another family", fam.outsider.FamilyID, fam.kid.ID},
	} {
		if _, err := s.GetConversation(ctx, tt.family, tt.member, first.ID); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: GetConversation err = %v, want ErrNotFound", tt.name, err)
		}
		if err := s.DeleteConversation(ctx, tt.family, tt.member, first.ID); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: DeleteConversation err = %v, want ErrNotFound", tt.name, err)
		}
	}
}

func TestReviewQueue(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)
	c, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	low := addFlagged(t, s, c, "low one", "low", false)
	crisis := addFlagged(t, s, c, "crisis one", "high", true)
	high := addFlagged(t, s, c, "high one", "high", false)
	medium := addFlagged(t, s, c, "medium one", "medium", false)

	queue, err := s.ReviewQueue(ctx, fam.parent.FamilyID, 50)
	if err != nil {
		t.Fatal(err)
	}
	var order []int64
	for _, f := range queue {
		order = append(order, f.ID)
	}
	want := []int64{crisis.ID, high.ID, medium.ID, low.ID}
	if len(order) != 4 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] || order[3] != want[3] {
		t.Errorf("queue order = %v, want %v", order, want)
	}
	if queue[0].MessageText != "crisis one" {
		t.Errorf("queue entry text = %q, want the flagged message", queue[0].MessageText)
	}

	crises, err := s.UnreviewedCrises(ctx, fam.parent.FamilyID)
	if err != nil || len(crises) != 1 || crises[0].ID != crisis.ID {
		t.Errorf("crises = %+v, %v; want the crisis flag", crises, err)
	}

	// Reviewing moves a flag to the end and records who.
	reviewed, err := s.MarkFlagReviewed(ctx, fam.parent.FamilyID, crisis.ID, fam.parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.ReviewedBy != fam.parent.ID || reviewed.ReviewedAt.IsZero() {
		t.Errorf("reviewed = %+v, want reviewed by the parent with a time", reviewed)
	}
	queue, _ = s.ReviewQueue(ctx, fam.parent.FamilyID, 50)
	if queue[len(queue)-1].ID != crisis.ID {
		t.Error("the reviewed flag isn't last")
	}
	if crises, _ := s.UnreviewedCrises(ctx, fam.parent.FamilyID); len(crises) != 0 {
		t.Errorf("crises after review = %+v, want none", crises)
	}
	if n, _ := s.CountUnreviewedFlags(ctx, fam.parent.FamilyID); n != 3 {
		t.Errorf("unreviewed = %d, want 3", n)
	}

	// Another family sees nothing and can't review.
	if q, _ := s.ReviewQueue(ctx, fam.outsider.FamilyID, 50); len(q) != 0 {
		t.Errorf("another family's queue = %+v, want empty", q)
	}
	if _, err := s.MarkFlagReviewed(ctx, fam.outsider.FamilyID, low.ID, fam.outsider.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("another family's review err = %v, want ErrNotFound", err)
	}
}

func TestDeleteConversationCascades(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)
	c, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	addFlagged(t, s, c, "flagged", "medium", false)
	if err := s.InsertLLMUsage(ctx, store.LLMUsage{FamilyID: fam.kid.FamilyID, MemberID: fam.kid.ID, Feature: "kid_chat", Step: "kid_chat_reply", Model: "m", TotalTokens: 5}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteConversation(ctx, fam.kid.FamilyID, fam.kid.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"chat_conversations": 0, "chat_messages": 0, "chat_flags": 0, "llm_usage": 1} {
		if n := countRows(t, s, table); n != want {
			t.Errorf("%s has %d rows, want %d", table, n, want)
		}
	}
}

func TestBirthYear(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)

	if _, ok, err := s.GetBirthYear(ctx, fam.kid.ID); err != nil || ok {
		t.Errorf("before setting: ok = %v, err = %v; want none", ok, err)
	}
	if err := s.UpsertMemberProfile(ctx, store.MemberProfile{UserID: fam.kid.ID, Interests: "soccer", Location: "Denver"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBirthYear(ctx, fam.kid.ID, 2011); err != nil {
		t.Fatal(err)
	}
	if year, ok, err := s.GetBirthYear(ctx, fam.kid.ID); err != nil || !ok || year != 2011 {
		t.Errorf("birth year = %d, %v, %v; want 2011", year, ok, err)
	}
	// Saving the profile leaves the birth year alone, and the reverse.
	if err := s.UpsertMemberProfile(ctx, store.MemberProfile{UserID: fam.kid.ID, Interests: "art", Location: "Denver"}); err != nil {
		t.Fatal(err)
	}
	if year, _, _ := s.GetBirthYear(ctx, fam.kid.ID); year != 2011 {
		t.Errorf("birth year after a profile save = %d, want 2011", year)
	}
	if p, _ := s.GetMemberProfile(ctx, fam.kid.ID); p.Interests != "art" {
		t.Errorf("interests = %q, want art", p.Interests)
	}
	if err := s.SetBirthYear(ctx, fam.kid.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.GetBirthYear(ctx, fam.kid.ID); ok {
		t.Error("birth year still set after clearing")
	}
}

func TestChatToolCalls(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)
	c, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateConversation(ctx, fam.kid.FamilyID, fam.kid.ID, store.ModeExplore)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddChatMessage(ctx, c.ID, store.RoleAgent, "It's sunny.")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddChatToolCalls(ctx, c.ID, reply.ID, []store.ChatToolCall{{Tool: "weather"}, {Tool: "search", Detail: "volcano facts"}}); err != nil {
		t.Fatal(err)
	}
	// A failed reply's calls are kept with no message.
	if err := s.AddChatToolCalls(ctx, c.ID, 0, []store.ChatToolCall{{Tool: "search", Detail: "lava"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddChatToolCalls(ctx, other.ID, 0, []store.ChatToolCall{{Tool: "my_day"}}); err != nil {
		t.Fatal(err)
	}

	calls, err := s.ListChatToolCalls(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0].Tool != "weather" || calls[0].MessageID != reply.ID ||
		calls[1].Detail != "volcano facts" || calls[2].MessageID != 0 || calls[2].Detail != "lava" {
		t.Errorf("calls = %+v, want weather and the search on the reply, then the unattached search", calls)
	}

	if err := s.DeleteConversation(ctx, fam.kid.FamilyID, fam.kid.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, "chat_tool_calls"); n != 1 {
		t.Errorf("%d tool calls left, want only the other conversation's", n)
	}
}
