package main

import (
	"context"
	"kinlight/internal/auth"
	"kinlight/internal/hosteddemo"
	"kinlight/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedSeedAndReset(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "kinlight-demo")
	os.Mkdir(dir, 0700)
	ctx := context.Background()
	t.Setenv("SEED_CALENDAR_MAYA", "https://private.example/secret")
	if err = provisionHosted(ctx, dir, false, "", "parent-demo-secret", "child-demo-secret"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, filepath.Join(dir, hosteddemo.ContentName))
	if err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"maya@rivera.test", "sam@rivera.test", "jordan@rivera.test", "lily@rivera.test"} {
		u, e := st.GetUserByEmail(ctx, email)
		if e != nil {
			t.Fatal(e)
		}
		expected := "child-demo-secret"
		if u.IsAdmin {
			expected = "parent-demo-secret"
		}
		if !auth.CheckPassword(expected, u.PasswordHash) {
			t.Fatal("wrong role password")
		}
		var n int
		if e = st.DB().QueryRow("SELECT count(*) FROM chores WHERE assignee_user_id=?", u.ID).Scan(&n); e != nil || n != 2 {
			t.Fatalf("chores %d: %v", n, e)
		}
	}
	var calendars string
	if err = st.DB().QueryRow("SELECT group_concat(calendar_url) FROM member_profiles").Scan(&calendars); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(calendars, "private.example") {
		t.Fatal("imported private feed")
	}
	if _, err = st.DB().Exec("INSERT INTO sessions(token,data,expiry) VALUES('test',X'00',9999999999)"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if err = provisionHosted(ctx, dir, true, dir, "parent-demo-secret", "child-demo-secret"); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(ctx, filepath.Join(dir, hosteddemo.ContentName))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var n int
	if err = st.DB().QueryRow("SELECT count(*) FROM sessions").Scan(&n); err != nil || n != 0 {
		t.Fatalf("sessions %d: %v", n, err)
	}
}
