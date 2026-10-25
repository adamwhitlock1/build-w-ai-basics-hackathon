package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
			s, err := d.DemoQuota.Status(r.Context())
			if err != nil {
				respond.Error(w, 503, "Demo allowance unavailable.", "Live calls are paused.")
				return
			}
			out.Quota = &s
		}
		respond.JSON(w, 200, out)
	}
}
func demoRestrictions(d deps, next http.Handler) http.Handler {
	if d.DemoQuota == nil {
		return next
	}
	slots := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		write := r.Method != "GET" && r.Method != "HEAD"
		blocked := write && (p == "/api/signup" || p == "/api/families" || strings.Contains(p, "/invites") || strings.HasSuffix(p, "/calendar") || strings.Contains(p, "password"))
		if blocked {
			respond.Error(w, 403, "That setting is fixed in the shared judge demo.", "You can still arrange pages, try AI and manage chores.")
			return
		}
		// Bound complete AI operations, including streaming and tool rounds. Cached reads remain usable.
		paid := strings.Contains(p, "/widgets/") || (r.Method == "POST" && strings.HasSuffix(p, "/messages"))
		if paid {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-r.Context().Done():
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(hosteddemo.WithOperation(r.Context())))
	})
}
