package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePublicURL(t *testing.T) {
	for raw, want := range map[string]string{
		"":                               "",
		"https://kinlight.codestuff.io":  "https://kinlight.codestuff.io",
		"https://kinlight.codestuff.io/": "https://kinlight.codestuff.io",
		" https://example.test:8443 ":    "https://example.test:8443",
	} {
		got, err := parsePublicURL(raw)
		if err != nil || got != want {
			t.Errorf("parsePublicURL(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"http://kinlight.codestuff.io", "kinlight.codestuff.io", "https://kinlight.codestuff.io/app", "https://", "https://x.test/?a=1"} {
		if _, err := parsePublicURL(raw); err == nil {
			t.Errorf("parsePublicURL(%q) passed, want an error", raw)
		}
	}
}

func TestDevOnlySettings(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }
	if on := devOnlySettings(getenv); len(on) != 0 {
		t.Fatalf("nothing set, got %v", on)
	}
	for _, kv := range [][2]string{
		{"KINLIGHT_DEV_FIXTURES", "1"},
		{"KINLIGHT_KIDS_ALLOW_RETENTION", "1"},
		{"GENKIT_ENV", "dev"},
		{"GENKIT_REFLECTION_ENABLED", "true"},
	} {
		env = map[string]string{kv[0]: kv[1]}
		on := devOnlySettings(getenv)
		if len(on) != 1 || !strings.Contains(on[0], kv[0]) {
			t.Errorf("%s=%s: got %v, want it named", kv[0], kv[1], on)
		}
	}
	env = map[string]string{"GENKIT_ENV": "prod", "KINLIGHT_DEV_FIXTURES": "0"}
	if on := devOnlySettings(getenv); len(on) != 0 {
		t.Errorf("off values counted: %v", on)
	}
}

func TestSecureCookieOnlyInProduction(t *testing.T) {
	for _, tt := range []struct {
		name, publicURL string
		secure          bool
	}{
		{"development", "", false},
		{"production", "https://kinlight.example.test", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := startServerOpts(t, filepath.Join(t.TempDir(), "test.db"), func(d *deps) { d.PublicURL = tt.publicURL })
			resp := newClient(t, srv).do("POST", "/api/signup", creds{"maya@example.com", "correct horse"}, http.StatusCreated, nil)
			cookie := resp.Header.Get("Set-Cookie")
			if !strings.Contains(cookie, "kinlight_session=") || strings.Contains(cookie, "Secure") != tt.secure {
				t.Errorf("Set-Cookie = %q, want Secure=%v", cookie, tt.secure)
			}
			for _, attr := range []string{"HttpOnly", "SameSite=Lax"} {
				if !strings.Contains(cookie, attr) {
					t.Errorf("Set-Cookie = %q, missing %s", cookie, attr)
				}
			}
		})
	}
}

func TestSecurityHeadersOnlyInProduction(t *testing.T) {
	for _, tt := range []struct {
		name, publicURL string
	}{{"development", ""}, {"production", "https://kinlight.example.test"}} {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := startServerOpts(t, filepath.Join(t.TempDir(), "test.db"), func(d *deps) { d.PublicURL = tt.publicURL })
			resp := newClient(t, srv).do("GET", "/api/health", nil, http.StatusOK, nil)
			csp := resp.Header.Get("Content-Security-Policy")
			if (csp != "") != (tt.publicURL != "") {
				t.Errorf("Content-Security-Policy = %q in %s", csp, tt.name)
			}
			if tt.publicURL != "" {
				for _, h := range []string{"Strict-Transport-Security", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy"} {
					if resp.Header.Get(h) == "" {
						t.Errorf("missing %s", h)
					}
				}
			}
		})
	}
}
