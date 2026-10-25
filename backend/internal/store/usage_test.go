package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"kinlight/internal/store"
)

func TestUsageSummary(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	parent := signUp(t, s, "parent@example.com")
	if _, _, err := s.InviteToFamily(ctx, invite(parent.FamilyID, "kid@example.com", "kid-token", false)); err != nil {
		t.Fatal(err)
	}
	kid, err := s.AcceptInvite(ctx, "kid-token", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	other := signUp(t, s, "other@example.com") // a second family

	now := time.Now()
	use := func(u store.LLMUsage) {
		t.Helper()
		if err := s.InsertLLMUsage(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	tokens := func(u store.LLMUsage, prompt, completion int) store.LLMUsage {
		u.Model, u.PromptTokens, u.CompletionTokens, u.TotalTokens = "test/model", prompt, completion, prompt+completion
		return u
	}
	fam := parent.FamilyID
	// The kid describes a widget: two steps, both theirs.
	use(tokens(store.LLMUsage{FamilyID: fam, MemberID: kid.ID, ViewerID: kid.ID, Feature: "describe_widget", Step: "widget_spec"}, 100, 20))
	use(tokens(store.LLMUsage{FamilyID: fam, MemberID: kid.ID, ViewerID: kid.ID, Feature: "describe_widget", Step: "widget_items"}, 300, 80))
	// The parent opens the kid's page: the kid's articles, caused by the parent.
	use(tokens(store.LLMUsage{FamilyID: fam, MemberID: kid.ID, ViewerID: parent.ID, Feature: "articles", Step: "article_queries"}, 50, 10))
	// The parent's own briefing.
	use(tokens(store.LLMUsage{FamilyID: fam, MemberID: parent.ID, ViewerID: parent.ID, Feature: "briefing", Step: "briefing"}, 200, 40))
	// Left out of the summary: last week, another family, and unattributed.
	use(tokens(store.LLMUsage{CreatedAt: now.Add(-8 * 24 * time.Hour), FamilyID: fam, MemberID: kid.ID, ViewerID: kid.ID, Feature: "articles", Step: "article_queries"}, 999, 999))
	use(tokens(store.LLMUsage{FamilyID: other.FamilyID, MemberID: other.ID, ViewerID: other.ID, Feature: "briefing", Step: "briefing"}, 999, 999))
	use(tokens(store.LLMUsage{Feature: "unattributed", Step: "widget_spec"}, 7, 7))

	got, err := s.UsageSummary(ctx, fam, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := []store.MemberUsage{
		{MemberID: parent.ID, Feature: "briefing", Calls: 1, PromptTokens: 200, CompletionTokens: 40, TotalTokens: 240},
		{MemberID: kid.ID, Feature: "articles", Calls: 1, PromptTokens: 50, CompletionTokens: 10, TotalTokens: 60},
		{MemberID: kid.ID, Feature: "describe_widget", Calls: 2, PromptTokens: 400, CompletionTokens: 100, TotalTokens: 500},
	}
	if !slices.Equal(got, want) {
		t.Errorf("summary =\n%+v\nwant\n%+v", got, want)
	}

	// The unattributed row is kept, with no one attached.
	var unattributed int
	if err := s.DB().QueryRow(`SELECT count(*) FROM llm_usage WHERE member_id IS NULL AND family_id IS NULL AND feature = 'unattributed'`).Scan(&unattributed); err != nil {
		t.Fatal(err)
	}
	if unattributed != 1 {
		t.Errorf("unattributed rows = %d, want 1", unattributed)
	}
}
