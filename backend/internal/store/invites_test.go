package store_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"porchlight/internal/store"
)

// invite is a store.Invite to familyID that expires in a week.
func invite(familyID int64, email, token string, isAdmin bool) store.Invite {
	return store.Invite{FamilyID: familyID, Email: email, Token: token, IsAdmin: isAdmin, ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
}

func TestInviteToFamily(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	signUp(t, s, "bob@example.com") // in a family of his own

	// An expired invite for kid@ doesn't count as pending.
	if _, err := s.CreateInvite(ctx, store.Invite{FamilyID: alice.FamilyID, Email: "kid@example.com", Token: "old", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	first, created, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "kid@example.com", "tok-1", false))
	if err != nil || !created || first.ID == 0 || first.Token != "tok-1" || first.IsAdmin {
		t.Fatalf("first invite = %+v, created %v, %v; want a new kid invite", first, created, err)
	}

	tests := []struct {
		name        string
		inv         store.Invite
		wantErr     error
		wantCreated bool
		wantAdmin   bool
	}{
		{"again: the same invite back", invite(alice.FamilyID, "kid@example.com", "tok-2", false), nil, false, false},
		{"again as a parent: same invite, new role", invite(alice.FamilyID, "kid@example.com", "tok-3", true), nil, false, true},
		{"already in this family", invite(alice.FamilyID, "alice@example.com", "tok-4", false), store.ErrAlreadyMember, false, false},
		{"has an account in another family", invite(alice.FamilyID, "bob@example.com", "tok-5", false), store.ErrEmailTaken, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, created, err := s.InviteToFamily(ctx, tt.inv)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if created != tt.wantCreated || got.ID != first.ID || got.Token != first.Token || got.IsAdmin != tt.wantAdmin {
				t.Errorf("got %+v, created %v; want invite %d (token %q), admin %v", got, created, first.ID, first.Token, tt.wantAdmin)
			}
		})
	}

	// Still one pending invite for kid@, and it's a parent invite now.
	pending, err := s.ListPendingInvites(ctx, alice.FamilyID)
	if err != nil || len(pending) != 1 || pending[0].ID != first.ID || !pending[0].IsAdmin {
		t.Errorf("pending = %+v, %v; want only invite %d, as a parent", pending, err, first.ID)
	}
}

func TestAcceptInvite(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	inv, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "kid@example.com", "kid-token", false))
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetInviteByToken(ctx, "kid-token")
	if err != nil || got.ID != inv.ID || got.Email != "kid@example.com" {
		t.Fatalf("GetInviteByToken = %+v, %v; want invite %d", got, err, inv.ID)
	}

	kid, err := s.AcceptInvite(ctx, "kid-token", []byte("kid-hash"))
	if err != nil {
		t.Fatal(err)
	}
	if kid.Email != "kid@example.com" || kid.FamilyID != alice.FamilyID || kid.IsAdmin || string(kid.PasswordHash) != "kid-hash" {
		t.Errorf("new user = %+v, want a non-admin kid in alice's family", kid)
	}
	if _, err := s.GetMemberProfile(ctx, kid.ID); err != nil {
		t.Errorf("new member has no profile: %v", err)
	}
	// No layout row: the homepage starts from the default layout.
	if _, err := s.GetLayout(ctx, kid.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("new member's layout: err = %v, want ErrNotFound", err)
	}

	// The invite is used up: no longer pending, and a second accept (a
	// double-clicked button) is refused without creating anyone.
	if pending, _ := s.ListPendingInvites(ctx, alice.FamilyID); len(pending) != 0 {
		t.Errorf("pending after accepting = %+v, want none", pending)
	}
	if _, err := s.AcceptInvite(ctx, "kid-token", []byte("hash")); !errors.Is(err, store.ErrInviteExpired) {
		t.Errorf("second accept: err = %v, want ErrInviteExpired", err)
	}
	if _, err := s.GetInviteByToken(ctx, "kid-token"); !errors.Is(err, store.ErrInviteExpired) {
		t.Errorf("GetInviteByToken after accepting: err = %v, want ErrInviteExpired", err)
	}
	if n := countRows(t, s, "users"); n != 2 {
		t.Errorf("users = %d, want 2 (alice and the kid)", n)
	}

	members, err := s.ListFamilyMembers(ctx, alice.FamilyID)
	if err != nil || len(members) != 2 || members[0].ID != alice.ID || members[1].ID != kid.ID {
		t.Errorf("members = %+v, %v; want alice then the kid", members, err)
	}
}

// A double-clicked button sends the same accept twice at once. The claim
// and the new account happen in one transaction, so exactly one wins.
func TestAcceptInviteConcurrently(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	if _, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "kid@example.com", "kid-token", false)); err != nil {
		t.Fatal(err)
	}

	const tries = 8
	errs := make(chan error, tries)
	var wg sync.WaitGroup
	for range tries {
		wg.Go(func() {
			_, err := s.AcceptInvite(ctx, "kid-token", []byte("hash"))
			errs <- err
		})
	}
	wg.Wait()
	close(errs)

	accepted := 0
	for err := range errs {
		switch {
		case err == nil:
			accepted++
		case !errors.Is(err, store.ErrInviteExpired):
			t.Errorf("err = %v, want nil or ErrInviteExpired", err)
		}
	}
	if accepted != 1 {
		t.Errorf("%d accepts succeeded, want exactly 1", accepted)
	}
	if n := countRows(t, s, "users"); n != 2 {
		t.Errorf("users = %d, want 2 (alice and one kid)", n)
	}
}

func TestAcceptInviteRefusals(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")

	if _, err := s.CreateInvite(ctx, store.Invite{FamilyID: alice.FamilyID, Email: "late@example.com", Token: "expired", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	// taken@ signs up on their own after being invited.
	if _, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "taken@example.com", "taken", false)); err != nil {
		t.Fatal(err)
	}
	signUp(t, s, "taken@example.com")

	tests := []struct {
		token   string
		wantErr error
	}{
		{"no-such-token", store.ErrNotFound},
		{"expired", store.ErrInviteExpired},
		{"taken", store.ErrEmailTaken},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			if _, err := s.AcceptInvite(ctx, tt.token, []byte("hash")); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	// The refused accept for taken@ rolled back entirely: no extra user, and
	// the invite is still pending (a parent can revoke it).
	if n := countRows(t, s, "users"); n != 2 {
		t.Errorf("users = %d, want 2", n)
	}
	if _, err := s.GetInviteByToken(ctx, "taken"); err != nil {
		t.Errorf("taken@'s invite after the refused accept: err = %v, want it still usable", err)
	}
}

func TestRevokeInvite(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	bob := signUp(t, s, "bob@example.com")
	pending, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "kid@example.com", "kid-token", false))
	if err != nil {
		t.Fatal(err)
	}
	accepted, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "co@example.com", "co-token", true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvite(ctx, "co-token", []byte("hash")); err != nil {
		t.Fatal(err)
	}

	// Bob's family can't revoke Alice's invite, and an accepted invite
	// can't be revoked: both read as missing.
	if err := s.RevokeInvite(ctx, bob.FamilyID, pending.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoke from another family: err = %v, want ErrNotFound", err)
	}
	if err := s.RevokeInvite(ctx, alice.FamilyID, accepted.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoke an accepted invite: err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetInviteByToken(ctx, "kid-token"); err != nil {
		t.Fatalf("after the refused revokes, the kid's invite: err = %v, want it still usable", err)
	}

	if err := s.RevokeInvite(ctx, alice.FamilyID, pending.ID); err != nil {
		t.Fatal(err)
	}
	// A revoked invite's link reads as never having existed.
	if _, err := s.GetInviteByToken(ctx, "kid-token"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoked invite: err = %v, want ErrNotFound", err)
	}
	if err := s.RevokeInvite(ctx, alice.FamilyID, pending.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoking twice: err = %v, want ErrNotFound", err)
	}
}

// Revoking the newest invite mustn't free its id for the next one, or a
// repeated revoke (a retried request, a stale list on another device) would
// quietly revoke the new invite instead.
func TestRevokeDoesNotReuseIDs(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")

	old, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "old@example.com", "old-token", false))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeInvite(ctx, alice.FamilyID, old.ID); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "new@example.com", "new-token", false))
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == old.ID {
		t.Fatalf("the new invite got the revoked invite's id %d", old.ID)
	}

	if err := s.RevokeInvite(ctx, alice.FamilyID, old.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoking the old id again: err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetInviteByToken(ctx, "new-token"); err != nil {
		t.Errorf("the new invite after that: err = %v, want it still usable", err)
	}
	// The revoked invite doesn't count as pending, so inviting old@ again
	// makes a new invite rather than handing back the revoked one.
	again, created, err := s.InviteToFamily(ctx, invite(alice.FamilyID, "old@example.com", "old-again", false))
	if err != nil || !created || again.ID == old.ID || again.Token != "old-again" {
		t.Errorf("inviting old@ again = %+v, created %v, %v; want a new invite", again, created, err)
	}
}

func TestListFamilyMembersPutsAdminsFirst(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	signUp(t, s, "bob@example.com") // another family: not listed

	// The kid joins before the co-parent, but admins come first.
	for _, inv := range []store.Invite{
		invite(alice.FamilyID, "kid@example.com", "kid-token", false),
		invite(alice.FamilyID, "co@example.com", "co-token", true),
	} {
		if _, _, err := s.InviteToFamily(ctx, inv); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AcceptInvite(ctx, inv.Token, []byte("hash")); err != nil {
			t.Fatal(err)
		}
	}

	members, err := s.ListFamilyMembers(ctx, alice.FamilyID)
	if err != nil {
		t.Fatal(err)
	}
	var emails []string
	for _, m := range members {
		emails = append(emails, m.Email)
	}
	if want := []string{"alice@example.com", "co@example.com", "kid@example.com"}; !slices.Equal(emails, want) {
		t.Errorf("members = %v, want %v", emails, want)
	}
}
