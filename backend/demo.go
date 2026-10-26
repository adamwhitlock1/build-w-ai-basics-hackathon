package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kinlight/internal/hosteddemo"
	"kinlight/internal/respond"
)

func publicLink(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("public links must be absolute HTTPS URLs without credentials")
	}
	return u.String(), nil
}
func configureDemo(c *config) error {
	var err error
	c.demoURL, err = publicLink(os.Getenv("KINLIGHT_DEMO_URL"))
	if err != nil {
		return err
	}
	if c.demoURL != "" {
		u, _ := url.Parse(c.demoURL)
		if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("KINLIGHT_DEMO_URL must be a site origin without path, query or fragment")
		}
	}
	c.videoURL, err = publicLink(os.Getenv("KINLIGHT_VIDEO_URL"))
	if err != nil {
		return err
	}
	if os.Getenv("KINLIGHT_HOSTED_DEMO") != "1" {
		return nil
	}
	if c.publicURL == "" {
		return fmt.Errorf("hosted demo requires KINLIGHT_PUBLIC_URL")
	}
	if c.miniMaxKey != "" || c.kidsKey == "" {
		return fmt.Errorf("hosted demo needs a separate kids key and does not support MiniMax")
	}
	for _, key := range []string{"LITESTREAM_BUCKET", "LITESTREAM_ACCESS_KEY_ID", "LITESTREAM_SECRET_ACCESS_KEY"} {
		if os.Getenv(key) != "" {
			return fmt.Errorf("hosted demo must not receive backup configuration")
		}
	}
	c.demoDir = os.Getenv("KINLIGHT_DEMO_DIR")
	if err = hosteddemo.Validate(c.demoDir); err != nil {
		return err
	}
	if c.dbPath != filepath.Join(c.demoDir, hosteddemo.ContentName) {
		return fmt.Errorf("DB_PATH must point to the marked judge-content.sqlite")
	}
	c.demoModels, err = positiveEnv("KINLIGHT_DEMO_MODEL_CALLS", 100)
	if err != nil {
		return err
	}
	c.demoSearches, err = positiveEnv("KINLIGHT_DEMO_SEARCH_CALLS", 50)
	return err
}
func positiveEnv(key string, fallback int) (int, error) {
	s := os.Getenv(key)
	if s == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return n, nil
}
func publicConfig(d deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		out := struct {
			Demo     bool               `json:"demo"`
			DemoURL  string             `json:"demoUrl"`
			VideoURL string             `json:"videoUrl"`
			Quota    *hosteddemo.Status `json:"quota,omitempty"`
		}{Demo: d.DemoQuota != nil, DemoURL: d.DemoURL, VideoURL: d.VideoURL}
		if d.DemoQuota != nil {
			// Without the ledger the counts are left out, but the page still
			// learns it is the shared demo and keeps its warning banner.
			if s, err := d.DemoQuota.Status(r.Context()); err == nil {
				out.Quota = &s
			} else {
				d.Logger.WarnContext(r.Context(), "can't read the demo allowance", "err", err)
			}
		}
		respond.JSON(w, 200, out)
	}
}

// demoFixed reports whether a write changes what every judge shares in a way
// another judge can't put back: accounts, invites, calendars, a kid's birth
// year (which picks the kid agent's persona), and deleting conversations.
func demoFixed(method, p string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return false
	}
	switch {
	case p == "/api/signup", p == "/api/families", strings.HasPrefix(p, "/api/families/"),
		strings.HasPrefix(p, "/api/invites/"),
		strings.HasSuffix(p, "/calendar"),
		strings.HasSuffix(p, "/birth-year"):
		return true
	case method == http.MethodDelete:
		return strings.Contains(p, "/chats/") || strings.HasPrefix(p, "/api/me/assistant/")
	}
	return false
}

// startsAI reports whether a request is one a person sends to start live AI:
// a chat or Assistant message, or building or refreshing a card.
func startsAI(method, p string) bool {
	return method == http.MethodPost && (strings.HasSuffix(p, "/messages") || strings.HasPrefix(p, "/api/widgets/"))
}

func demoRestrictions(d deps, next http.Handler) http.Handler {
	if d.DemoQuota == nil {
		return next
	}
	slots := d.DemoSlots
	if slots == nil {
		slots = hosteddemo.NewSlots(2, 20*time.Second)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if demoFixed(r.Method, r.URL.Path) {
			respond.Error(w, 403, "That setting is fixed in the shared judge demo.", "You can still arrange pages, try AI and manage chores.")
			return
		}
		// Turn a spent allowance away before anything is saved: otherwise a
		// kid's message would be stored and flagged as unchecked.
		if startsAI(r.Method, r.URL.Path) {
			var paused error
			if s, err := d.DemoQuota.Status(r.Context()); err != nil {
				paused = hosteddemo.ErrUnavailable
			} else if s.ModelsRemaining == 0 {
				paused = &hosteddemo.LimitError{Reset: s.Reset}
			}
			if paused != nil {
				respond.JSON(w, 429, hosteddemo.ErrorBody(paused, "Existing content is still available."))
				return
			}
		}
		// Each request is one operation: it takes an AI slot on its first
		// paid dispatch (see hosteddemo.Transport) and returns it here.
		ctx := hosteddemo.WithOperation(r.Context(), slots)
		defer hosteddemo.Release(ctx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
