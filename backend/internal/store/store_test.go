package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"porchlight/internal/store"
)

// openTestStore opens a fresh database in a temporary directory that the
// testing package deletes when the test ends.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenOddFileNames(t *testing.T) {
	// SQLite reads the path as a URI, where these characters mean something.
	for _, name := range []string{"with space.db", "q?x.db", "hash#1.db", "pct%41.db"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := store.Open(context.Background(), filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			var foreignKeys int
			if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
				t.Fatal(err)
			}
			if foreignKeys != 1 {
				t.Errorf("foreign_keys = %d, want 1 (the connection options were lost)", foreignKeys)
			}
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("database not created under its own name: %v", err)
			}
		})
	}
}

func signUp(t *testing.T, s *store.Store, email string) store.User {
	t.Helper()
	u, err := s.CreateFamilyWithAdmin(context.Background(), email, []byte("hash"))
	if err != nil {
		t.Fatalf("CreateFamilyWithAdmin(%q): %v", email, err)
	}
	return u
}

func countRows(t *testing.T, s *store.Store, table string) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOpenIsRepeatable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := range 2 { // the second Open must find the schema already applied
		s, err := store.Open(context.Background(), path)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		if n := countRows(t, s, "schema_migrations"); n != 1 {
			t.Errorf("open #%d: schema_migrations has %d rows, want 1", i+1, n)
		}
		s.Close()
	}
}

func TestCreateFamilyWithAdmin(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	u := signUp(t, s, "parent@example.com")
	if !u.IsAdmin || u.Email != "parent@example.com" || u.ID == 0 || u.CreatedAt.IsZero() {
		t.Errorf("user = %+v, want an admin with an id and created_at", u)
	}

	fam, err := s.GetFamily(ctx, u.FamilyID)
	if err != nil {
		t.Fatal(err)
	}
	if fam.Name != "" {
		t.Errorf("family name = %q, want empty until setup", fam.Name)
	}

	profile, err := s.GetMemberProfile(ctx, u.ID)
	if err != nil {
		t.Fatalf("profile not created: %v", err)
	}
	if profile.Interests != "" || profile.Location != "" {
		t.Errorf("profile = %+v, want empty", profile)
	}

	got, err := s.GetUserByEmail(ctx, "parent@example.com")
	if err != nil || got.ID != u.ID || string(got.PasswordHash) != "hash" {
		t.Errorf("GetUserByEmail = %+v, %v; want user %d", got, err, u.ID)
	}
}

func TestCreateFamilyWithAdminDuplicateEmail(t *testing.T) {
	s := openTestStore(t)
	signUp(t, s, "parent@example.com")

	_, err := s.CreateFamilyWithAdmin(context.Background(), "parent@example.com", []byte("hash"))
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
	// The transaction rolled back, so the second family was never kept.
	if n := countRows(t, s, "families"); n != 1 {
		t.Errorf("families = %d, want 1", n)
	}
}

func TestGetMissing(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tests := []struct {
		name string
		call func() error
	}{
		{"user", func() error { _, err := s.GetUser(ctx, 99); return err }},
		{"user by email", func() error { _, err := s.GetUserByEmail(ctx, "nobody@example.com"); return err }},
		{"family", func() error { _, err := s.GetFamily(ctx, 99); return err }},
		{"profile", func() error { _, err := s.GetMemberProfile(ctx, 99); return err }},
		{"custom widget", func() error { _, err := s.GetCustomWidget(ctx, 1, 99); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, store.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestCompleteFamilySetup(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	u := signUp(t, s, "parent@example.com")

	expires := time.Now().Add(14 * 24 * time.Hour).Truncate(time.Second)
	setup := store.FamilySetup{
		FamilyID: u.FamilyID,
		Name:     "The Robinsons",
		Profile:  store.MemberProfile{UserID: u.ID, Interests: "gardening, rockets", Location: "Portland, OR"},
		Invites: []store.Invite{
			{Email: "coparent@example.com", Token: "tok-1", IsAdmin: true, ExpiresAt: expires},
		},
	}
	if err := s.CompleteFamilySetup(ctx, setup); err != nil {
		t.Fatal(err)
	}

	fam, _ := s.GetFamily(ctx, u.FamilyID)
	if fam.Name != "The Robinsons" {
		t.Errorf("family name = %q", fam.Name)
	}
	profile, _ := s.GetMemberProfile(ctx, u.ID)
	if profile != setup.Profile {
		t.Errorf("profile = %+v, want %+v", profile, setup.Profile)
	}
	invites, err := s.ListPendingInvites(ctx, u.FamilyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(invites) != 1 {
		t.Fatalf("invites = %+v, want 1", invites)
	}
	inv := invites[0]
	if inv.Email != "coparent@example.com" || inv.Token != "tok-1" || !inv.IsAdmin || inv.FamilyID != u.FamilyID || !inv.ExpiresAt.Equal(expires) {
		t.Errorf("invite = %+v", inv)
	}

	// A second setup is refused and changes nothing.
	setup.Name = "Someone Else"
	if err := s.CompleteFamilySetup(ctx, setup); !errors.Is(err, store.ErrFamilyAlreadySetUp) {
		t.Errorf("second setup err = %v, want ErrFamilyAlreadySetUp", err)
	}
	if fam, _ := s.GetFamily(ctx, u.FamilyID); fam.Name != "The Robinsons" {
		t.Errorf("family renamed to %q", fam.Name)
	}

	if err := s.CompleteFamilySetup(ctx, store.FamilySetup{FamilyID: 999, Name: "Ghosts"}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing family err = %v, want ErrNotFound", err)
	}
}

func TestCustomWidgets(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	bob := signUp(t, s, "bob@example.com")

	spec := json.RawMessage(`{"title":"SpaceX Launches","archetype":"list","items":[]}`)
	first, err := s.CreateCustomWidget(ctx, store.CustomWidget{
		UserID: alice.ID, Name: "SpaceX Launches", Source: "events_search", Query: "spacex launches", RenderSpec: spec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.CreatedAt.IsZero() {
		t.Errorf("created widget = %+v, want id and created_at set", first)
	}
	second, _ := s.CreateCustomWidget(ctx, store.CustomWidget{UserID: alice.ID, Name: "Quote", Source: "web_search", Query: "quote", RenderSpec: spec})
	_, _ = s.CreateCustomWidget(ctx, store.CustomWidget{UserID: bob.ID, Name: "Bob's", Source: "web_search", Query: "bob", RenderSpec: spec})

	got, err := s.GetCustomWidget(ctx, alice.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != first.Name || got.Source != first.Source || got.Query != first.Query || string(got.RenderSpec) != string(spec) || !got.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("round trip: got %+v, want %+v", got, first)
	}

	// Bob can't read Alice's widget.
	if _, err := s.GetCustomWidget(ctx, bob.ID, first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other user's widget: err = %v, want ErrNotFound", err)
	}

	list, err := s.ListCustomWidgets(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Errorf("alice's widgets = %+v, want [%d %d] oldest first", list, first.ID, second.ID)
	}

	none, err := s.ListCustomWidgets(ctx, 999)
	if err != nil || len(none) != 0 {
		t.Errorf("unknown user's widgets = %v, %v; want none", none, err)
	}
}

func TestProfilesAndInvites(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	u := signUp(t, s, "parent@example.com")

	want := store.MemberProfile{UserID: u.ID, Interests: "birds", Location: "Lisbon"}
	if err := s.UpsertMemberProfile(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetMemberProfile(ctx, u.ID); got != want {
		t.Errorf("profile = %+v, want %+v", got, want)
	}

	inv, err := s.CreateInvite(ctx, store.Invite{FamilyID: u.FamilyID, Email: "kid@example.com", Token: "tok", ExpiresAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if inv.ID == 0 {
		t.Error("invite has no id")
	}
	if _, err := s.CreateInvite(ctx, store.Invite{FamilyID: u.FamilyID, Email: "other@example.com", Token: "tok", ExpiresAt: time.Now()}); err == nil {
		t.Error("reused an invite token; want the UNIQUE constraint to refuse it")
	}
	if _, err := s.CreateInvite(ctx, store.Invite{FamilyID: 999, Email: "x@example.com", Token: "tok-2", ExpiresAt: time.Now()}); err == nil {
		t.Error("invite for a missing family; want the foreign key to refuse it")
	}
	invites, err := s.ListPendingInvites(ctx, u.FamilyID)
	if err != nil || len(invites) != 1 || invites[0].ID != inv.ID || invites[0].IsAdmin {
		t.Errorf("ListPendingInvites = %+v, %v; want just the kid's (non-admin) invite", invites, err)
	}
}
