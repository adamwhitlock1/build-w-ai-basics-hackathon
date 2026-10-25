package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Login throttling: after LoginFailures failed logins within LoginWindow,
// for one email or from one client address, further logins for it are
// refused until the oldest of those failures is LoginWindow old. It's held
// in memory, so a restart forgets it: enough for one household.
const (
	LoginFailures = 5
	LoginWindow   = 15 * time.Minute
)

// Throttle counts failed logins by key ("email:…", "ip:…"). It is safe
// for concurrent use.
type Throttle struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	now      func() time.Time
}

// NewThrottle returns an empty Throttle on the real clock.
func NewThrottle() *Throttle {
	return &Throttle{failures: map[string][]time.Time{}, now: time.Now}
}

// SetClock replaces the throttle's clock, for tests.
func (t *Throttle) SetClock(now func() time.Time) { t.now = now }

// recent returns key's failures still inside the window, dropping the rest.
// The caller holds t.mu.
func (t *Throttle) recent(key string) []time.Time {
	cutoff := t.now().Add(-LoginWindow)
	kept := t.failures[key][:0]
	for _, at := range t.failures[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(t.failures, key)
		return nil
	}
	t.failures[key] = kept
	return kept
}

// Blocked reports whether any of keys has reached the limit.
func (t *Throttle) Blocked(keys ...string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, key := range keys {
		if len(t.recent(key)) >= LoginFailures {
			return true
		}
	}
	return false
}

// Fail records a failed login against each of keys.
func (t *Throttle) Fail(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for _, key := range keys {
		t.failures[key] = append(t.recent(key), now)
	}
}

// Clear forgets key's failures (a successful login clears its email).
func (t *Throttle) Clear(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, key)
}

// ClientIP is the address a request came from: Cloudflare's
// CF-Connecting-IP when trustCloudflare (only when a Cloudflare Tunnel is
// the sole way in, since anyone can send the header), otherwise the
// connection's own address.
func ClientIP(r *http.Request, trustCloudflare bool) string {
	if trustCloudflare {
		if ip := net.ParseIP(r.Header.Get("CF-Connecting-IP")); ip != nil {
			return ip.String()
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
