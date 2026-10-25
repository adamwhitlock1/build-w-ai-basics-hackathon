package main

import (
	"io"
	"net/http"
	"os"
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

func TestServesTheWebApp(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<!doctype html><title>Kinlight</title>")
	write("assets/index-abc123.js", "console.log('app')")
	write("favicon.svg", "<svg></svg>")
	srv, _ := startServerOpts(t, filepath.Join(t.TempDir(), "test.db"), func(d *deps) { d.WebDir = dir })

	get := func(path string) (*http.Response, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp, string(body)
	}

	resp, body := get("/assets/index-abc123.js")
	if resp.StatusCode != http.StatusOK || body != "console.log('app')" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q cache %q", resp.StatusCode, body, resp.Header.Get("Cache-Control"))
	}
	resp, body = get("/favicon.svg")
	if resp.StatusCode != http.StatusOK || body != "<svg></svg>" || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("file: %d %q cache %q", resp.StatusCode, body, resp.Header.Get("Cache-Control"))
	}
	for _, path := range []string{"/", "/invite/abc", "/chat/3", "/assets/missing.js"} {
		resp, body = get(path)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, "<title>Kinlight</title>") || resp.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d %q cache %q, want the app's page", path, resp.StatusCode, body, resp.Header.Get("Cache-Control"))
		}
	}
	resp, body = get("/api/nothing-here")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(resp.Header.Get("Content-Type"), "json") || strings.Contains(body, "Kinlight</title>") {
		t.Errorf("/api/nothing-here: %d %s %q, want the API's JSON 404", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	resp, _ = get("/api/health")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/api/health: %d", resp.StatusCode)
	}
}

func TestNoWebDirServesOnlyTheAPI(t *testing.T) {
	srv, _ := startServerOpts(t, filepath.Join(t.TempDir(), "test.db"), nil)
	resp, err := http.Get(srv.URL + "/invite/abc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("/invite/abc without a web dir: %d, want 404", resp.StatusCode)
	}
}
