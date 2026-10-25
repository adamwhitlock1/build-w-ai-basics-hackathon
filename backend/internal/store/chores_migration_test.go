package store

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"

	"kinlight/migrations"
)

func TestChoreMigrationPreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "before.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	old := fstestBeforeChores{migrations.FS}
	if err := migrate(context.Background(), db, old); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO families(id,name) VALUES(1,'Family'); INSERT INTO users(id,email,password_hash,family_id,is_admin) VALUES(1,'parent@example.test',X'1234',1,1); INSERT INTO member_profiles(user_id,interests,location,calendar_url) VALUES(1,'space','Denver','https://example.test/private.ics'); INSERT INTO homepage_layouts(user_id,puck_data) VALUES(1,'{"custom":"keep"}');`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var email, calendar, layout string
	err = st.db.QueryRow(`SELECT u.email,p.calendar_url,h.puck_data FROM users u JOIN member_profiles p ON p.user_id=u.id JOIN homepage_layouts h ON h.user_id=u.id`).Scan(&email, &calendar, &layout)
	if err != nil || email != "parent@example.test" || calendar != "https://example.test/private.ics" || layout != `{"custom":"keep"}` {
		t.Fatalf("preservation: %s %s %s %v", email, calendar, layout, err)
	}
	items, err := st.ListChores(context.Background(), 1, 1, "2026-10-25")
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
}

type fstestBeforeChores struct{ fs.FS }

func (f fstestBeforeChores) Glob(pattern string) ([]string, error) {
	names, err := fs.Glob(f.FS, pattern)
	out := []string{}
	for _, name := range names {
		if name != "010_chores.sql" {
			out = append(out, name)
		}
	}
	return out, err
}
