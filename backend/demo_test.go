package main

import (
	"context"
	"encoding/json"
	"kinlight/internal/auth"
	"kinlight/internal/hosteddemo"
	"kinlight/internal/respond"
	"kinlight/internal/store"
	"net/http"
	"path/filepath"
	"testing"
)

func TestDemoRestrictionsAndPublicConfig(t *testing.T) {
	q, err := hosteddemo.OpenQuota(t.TempDir(), 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	srv, st := startServerOpts(t, filepath.Join(t.TempDir(), "test.db"), func(d *deps) {
		d.DemoQuota = q
		d.DemoURL = "https://demo.example"
		d.VideoURL = "https://video.example/watch"
	})
	hash, _ := auth.HashPassword("test-password")
	u, err := st.CreateFamilyWithAdmin(context.Background(), "judge@example.test", hash)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CompleteFamilySetup(context.Background(), store.FamilySetup{FamilyID: u.FamilyID, Name: "Judges", Profile: store.MemberProfile{UserID: u.ID}}); err != nil {
		t.Fatal(err)
	}
	c := newClient(t, srv)
	c.do("POST", "/api/login", map[string]string{"email": u.Email, "password": "test-password"}, 200, nil)
	for _, path := range []string{"/api/signup", "/api/families", "/api/families/1/invites", "/api/invites/token/accept"} {
		c.do("POST", path, map[string]string{}, 403, nil)
	}
	for _, path := range []string{"/api/me/calendar", "/api/members/1/calendar"} {
		c.do("PUT", path, map[string]string{"url": "https://secret.example"}, 403, nil)
	}
	c.do("GET", "/api/me", nil, 200, nil)
	var out map[string]any
	c.do("GET", "/api/public-config", nil, 200, &out)
	raw, _ := json.Marshal(out)
	if out["demo"] != true || len(out) != 4 {
		t.Fatalf("bad public config %s", raw)
	}
	for _, path := range []string{"/api/me/homepage", "/api/family/chores"} {
		c.do(http.MethodGet, path, nil, 200, nil)
	}
	// Shared things another judge can't put back stay fixed.
	c.do("PUT", "/api/members/2/birth-year", map[string]int{"birthYear": 2008}, 403, nil)
	c.do("DELETE", "/api/members/2/chats/1", nil, 403, nil)
	c.do("DELETE", "/api/me/assistant/1", nil, 403, nil)

	// A spent allowance turns new AI away before anything is saved, with a
	// code the page reads rather than the wording.
	for range 3 {
		_ = q.Reserve(context.Background(), "model")
	}
	for _, path := range []string{"/api/me/chats/1/messages", "/api/me/assistant/1/messages", "/api/widgets/from-prompt"} {
		var body respond.ErrorBody
		c.do("POST", path, map[string]string{"text": "hi"}, 429, &body)
		if body.Code != hosteddemo.CodeLimit {
			t.Errorf("POST %s code %q, want %q", path, body.Code, hosteddemo.CodeLimit)
		}
	}
	c.do("GET", "/api/me/homepage", nil, 200, nil)

	// Without the ledger the page still learns it's the shared demo.
	q.Close()
	out = nil
	c.do("GET", "/api/public-config", nil, 200, &out)
	if out["demo"] != true || out["quota"] != nil {
		t.Fatalf("public config without ledger: %v", out)
	}
}
func TestPublicLinks(t *testing.T) {
	for _, raw := range []string{"javascript:alert(1)", "//evil.example", "http://example.com", "https://user:secret@example.com"} {
		if _, err := publicLink(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"", "https://demo.example", "https://www.youtube.com/watch?v=123"} {
		if _, err := publicLink(raw); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHostedConfigFailsClosed(t *testing.T) {
	for _, key := range []string{"KINLIGHT_DEMO_URL", "KINLIGHT_VIDEO_URL", "KINLIGHT_DEMO_DIR", "LITESTREAM_BUCKET", "LITESTREAM_ACCESS_KEY_ID", "LITESTREAM_SECRET_ACCESS_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("KINLIGHT_HOSTED_DEMO", "1")
	c := config{}
	if configureDemo(&c) == nil {
		t.Fatal("accepted demo without production URL")
	}
	c.publicURL = "https://demo.example"
	c.kidsKey = "demo-kids"
	if configureDemo(&c) == nil {
		t.Fatal("accepted unmarked database")
	}
	t.Setenv("LITESTREAM_BUCKET", "household")
	if configureDemo(&c) == nil {
		t.Fatal("accepted household backups")
	}
	t.Setenv("KINLIGHT_HOSTED_DEMO", "")
	t.Setenv("KINLIGHT_DEMO_URL", "https://demo.example/login?x=1")
	if configureDemo(&c) == nil {
		t.Fatal("accepted non-origin demo URL")
	}
}
