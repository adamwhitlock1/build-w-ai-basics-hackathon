package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kinlight/internal/store"
	"kinlight/migrations"
)

// TestMigrateFromBeforeFreshness opens a database built by migrations 001
// and 002 only, holding a widget saved back then, and checks that Open
// brings it up to date without losing the widget.
func TestMigrateFromBeforeFreshness(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")

	// Build the old database by hand: the two old migrations, recorded the
	// way store.migrate records them, plus one user and one widget.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_init.sql", "002_invite_revoked_at.sql"} {
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, strings.TrimSuffix(name, ".sql")); err != nil {
			t.Fatal(err)
		}
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO families (id, name) VALUES (1, 'Old')`)
	mustExec(`INSERT INTO users (id, email, password_hash, family_id, is_admin) VALUES (1, 'old@example.com', x'00', 1, 1)`)
	mustExec(`INSERT INTO custom_widgets (id, user_id, name, source, query, render_spec) VALUES (7, 1, 'Old widget', 'web_search', 'old', '{"title":"Old widget","archetype":"list","items":[]}')`)
	db.Close()

	s, err := store.Open(ctx, path)
	if err != nil {
		t.Fatalf("open old database: %v", err)
	}
	defer s.Close()

	w, err := s.GetCustomWidget(ctx, 1, 7)
	if err != nil {
		t.Fatalf("old widget after migrating: %v", err)
	}
	if w.Prompt != "" || !w.RefreshedAt.IsZero() {
		t.Errorf("old widget: prompt %q, refreshed at %v; want empty and zero", w.Prompt, w.RefreshedAt)
	}
	if _, err := s.GetCuratedResult(ctx, 1, "articles"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("curated result in the migrated database: err = %v, want ErrNotFound (the table exists and is empty)", err)
	}
}

func TestCustomWidgetPromptAndRefresh(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	bob := signUp(t, s, "bob@example.com")

	created, err := s.CreateCustomWidget(ctx, store.CustomWidget{
		UserID: alice.ID, Name: "Launches", Source: "web_search", Query: "spacex launches",
		Prompt:     "show me upcoming SpaceX launches",
		RenderSpec: json.RawMessage(`{"title":"Launches","archetype":"list","items":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Prompt != "show me upcoming SpaceX launches" || !created.RefreshedAt.IsZero() {
		t.Errorf("created: prompt %q, refreshed at %v; want the prompt and zero", created.Prompt, created.RefreshedAt)
	}

	newSpec := json.RawMessage(`{"title":"Launches","archetype":"list","items":[{"title":"Starship"}]}`)
	at := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

	// Bob can't refresh Alice's widget, and it's left alone.
	if _, err := s.UpdateCustomWidgetItems(ctx, bob.ID, created.ID, newSpec, at); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("refreshing another user's widget: err = %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateCustomWidgetItems(ctx, alice.ID, 999, newSpec, at); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("refreshing a missing widget: err = %v, want ErrNotFound", err)
	}

	updated, err := s.UpdateCustomWidgetItems(ctx, alice.ID, created.ID, newSpec, at)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated.RenderSpec) != string(newSpec) || !updated.RefreshedAt.Equal(at) {
		t.Errorf("updated: spec %s, refreshed at %v; want %s and %v", updated.RenderSpec, updated.RefreshedAt, newSpec, at)
	}
	got, _ := s.GetCustomWidget(ctx, alice.ID, created.ID)
	if string(got.RenderSpec) != string(newSpec) || !got.RefreshedAt.Equal(at) || got.Prompt != created.Prompt || !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("read back after refresh = %+v", got)
	}
}

func TestCuratedResults(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	alice := signUp(t, s, "alice@example.com")
	bob := signUp(t, s, "bob@example.com")

	if _, err := s.GetCuratedResult(ctx, alice.ID, "articles"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nothing remembered yet: err = %v, want ErrNotFound", err)
	}

	first := store.CuratedResult{
		UserID: alice.ID, Kind: "articles", Inputs: "space",
		Payload:   json.RawMessage(`{"items":[1]}`),
		UpdatedAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
	}
	if err := s.PutCuratedResult(ctx, first); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCuratedResult(ctx, alice.ID, "articles")
	if err != nil {
		t.Fatal(err)
	}
	if got.Inputs != first.Inputs || string(got.Payload) != string(first.Payload) || !got.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("round trip: got %+v, want %+v", got, first)
	}

	// Putting again replaces it.
	second := first
	second.Inputs, second.Payload, second.UpdatedAt = "soccer", json.RawMessage(`{"items":[2]}`), first.UpdatedAt.Add(time.Hour)
	if err := s.PutCuratedResult(ctx, second); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetCuratedResult(ctx, alice.ID, "articles")
	if got.Inputs != "soccer" || string(got.Payload) != `{"items":[2]}` || !got.UpdatedAt.Equal(second.UpdatedAt) {
		t.Errorf("after replacing: got %+v, want %+v", got, second)
	}
	if n := countRows(t, s, "curated_widget_results"); n != 1 {
		t.Errorf("rows after replacing = %d, want 1", n)
	}

	// Kinds and people are kept apart.
	if _, err := s.GetCuratedResult(ctx, alice.ID, "events"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("alice's events: err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetCuratedResult(ctx, bob.ID, "articles"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("bob's articles: err = %v, want ErrNotFound", err)
	}
}
