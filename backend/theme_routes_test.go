package main

import (
	"net/http"
	"strings"
	"testing"

	"kinlight/internal/family"
)

func TestTheme(t *testing.T) {
	srv, _ := newTestServer(t)
	maya := newClient(t, srv)
	if me := maya.signup("maya@example.com"); me.User.Theme != "auto" {
		t.Fatalf("a new member's theme = %q, want auto", me.User.Theme)
	}
	sam := newClient(t, srv)
	sam.signup("sam@example.com")

	t.Run("choosing a theme", func(t *testing.T) {
		var me family.Me
		maya.do("PUT", "/api/me/theme", map[string]string{"theme": "dark"}, http.StatusOK, &me)
		if me.User.Theme != "dark" || me.User.Email != "maya@example.com" {
			t.Errorf("PUT answered %+v, want Maya with dark", me.User)
		}
		maya.do("GET", "/api/me", nil, http.StatusOK, &me)
		if me.User.Theme != "dark" {
			t.Errorf("GET /api/me theme = %q, want dark", me.User.Theme)
		}
	})

	t.Run("other members keep theirs", func(t *testing.T) {
		var me family.Me
		sam.do("GET", "/api/me", nil, http.StatusOK, &me)
		if me.User.Theme != "auto" {
			t.Errorf("Sam's theme = %q after Maya chose hers, want auto", me.User.Theme)
		}
	})

	t.Run("an unknown theme is refused", func(t *testing.T) {
		for _, body := range []any{map[string]string{"theme": "sepia"}, map[string]string{"theme": ""}, map[string]string{}} {
			maya.wantError("PUT", "/api/me/theme", body, http.StatusBadRequest, family.ThemeExample)
		}
		var me family.Me
		maya.do("GET", "/api/me", nil, http.StatusOK, &me)
		if me.User.Theme != "dark" {
			t.Errorf("a refused theme changed Maya's to %q", me.User.Theme)
		}
	})

	t.Run("a cross-site request is refused", func(t *testing.T) {
		req, err := http.NewRequest("PUT", srv.URL+"/api/me/theme", strings.NewReader(`{"theme":"light"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.net")
		resp, err := maya.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
		var me family.Me
		maya.do("GET", "/api/me", nil, http.StatusOK, &me)
		if me.User.Theme != "dark" {
			t.Errorf("a cross-site request changed Maya's theme to %q", me.User.Theme)
		}
	})

	t.Run("logged out", func(t *testing.T) {
		newClient(t, srv).wantError("PUT", "/api/me/theme", map[string]string{"theme": "dark"}, http.StatusUnauthorized, "Please log in.")
	})
}
