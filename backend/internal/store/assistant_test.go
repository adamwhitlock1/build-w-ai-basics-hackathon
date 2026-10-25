package store_test

import (
	"context"
	"errors"
	"testing"

	"kinlight/internal/store"
)

func TestAssistantConversations(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	fam := newChatFamily(t, s)
	// A second parent in the same family.
	if _, _, err := s.InviteToFamily(ctx, invite(fam.parent.FamilyID, "parent2@example.com", "p2-token", true)); err != nil {
		t.Fatal(err)
	}
	parent2, err := s.AcceptInvite(ctx, "p2-token", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	p := fam.parent

	first, err := s.CreateAssistantConversation(ctx, p.FamilyID, p.ID, store.KindAnything)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateAssistantConversation(ctx, p.FamilyID, p.ID, store.KindKids)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAssistantConversation(ctx, p.FamilyID, p.ID, "gossip"); err == nil {
		t.Error("an unknown kind was accepted")
	}

	m, err := s.AddAssistantMessage(ctx, first.ID, store.RoleParent, "what does everyone have tomorrow?")
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.AddAssistantMessage(ctx, first.ID, store.RoleAgent, "Jordan has soccer.")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAssistantConversationTitle(ctx, first.ID, "Tomorrow"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddAssistantToolCalls(ctx, first.ID, reply.ID, []store.ChatToolCall{{Tool: "family_day", Detail: "everyone"}}); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListAssistantConversations(ctx, p.FamilyID, p.ID)
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[0].Title != "Tomorrow" || list[1].ID != second.ID {
		t.Errorf("list = %+v, %v; want the first (titled, most recent) then the second", list, err)
	}
	recent, _ := s.RecentAssistantMessages(ctx, first.ID, 1)
	if len(recent) != 1 || recent[0].ID != reply.ID {
		t.Errorf("recent = %+v, want the reply", recent)
	}
	if all, _ := s.ListAssistantMessages(ctx, first.ID); len(all) != 2 || all[0].ID != m.ID {
		t.Errorf("messages = %+v", all)
	}
	if calls, _ := s.ListAssistantToolCalls(ctx, first.ID); len(calls) != 1 || calls[0].MessageID != reply.ID {
		t.Errorf("tool calls = %+v", calls)
	}

	// Private: the other parent, a kid and another family find nothing.
	for _, who := range []store.User{parent2, fam.kid, fam.outsider} {
		if _, err := s.GetAssistantConversation(ctx, who.FamilyID, who.ID, first.ID); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: get err = %v, want ErrNotFound", who.Email, err)
		}
		if err := s.DeleteAssistantConversation(ctx, who.FamilyID, who.ID, first.ID); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: delete err = %v, want ErrNotFound", who.Email, err)
		}
		if l, _ := s.ListAssistantConversations(ctx, who.FamilyID, who.ID); len(l) != 0 {
			t.Errorf("%s sees %d conversations", who.Email, len(l))
		}
	}

	if err := s.DeleteAssistantConversation(ctx, p.FamilyID, p.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"assistant_conversations": 1, "assistant_messages": 0, "assistant_tool_calls": 0} {
		if n := countRows(t, s, table); n != want {
			t.Errorf("%s has %d rows, want %d", table, n, want)
		}
	}
}
