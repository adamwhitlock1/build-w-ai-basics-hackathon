package auth_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"kinlight/internal/auth"
)

func TestThrottle(t *testing.T) {
	now := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)
	th := auth.NewThrottle()
	th.SetClock(func() time.Time { return now })

	for i := range auth.LoginFailures {
		if th.Blocked("email:maya") {
			t.Fatalf("blocked after %d failures", i)
		}
		th.Fail("email:maya", "ip:1.2.3.4")
		now = now.Add(time.Minute)
	}
	if !th.Blocked("email:maya") || !th.Blocked("ip:1.2.3.4") || th.Blocked("email:sam") {
		t.Fatal("after five failures, the email and the address should be blocked, and only they")
	}

	// The first failure was at 8:00; at 8:15 it's out of the window.
	now = time.Date(2026, 10, 25, 8, 15, 0, 1, time.UTC)
	if th.Blocked("email:maya") {
		t.Error("still blocked once the oldest failure is 15 minutes old")
	}

	th.Fail("email:sam")
	th.Clear("email:sam")
	if th.Blocked("email:sam") {
		t.Error("Clear didn't forget")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "10.0.0.5:51234"
	r.Header.Set("CF-Connecting-IP", "203.0.113.9")
	if got := auth.ClientIP(r, false); got != "10.0.0.5" {
		t.Errorf("untrusted: %q, want the connection's address", got)
	}
	if got := auth.ClientIP(r, true); got != "203.0.113.9" {
		t.Errorf("trusted: %q, want Cloudflare's header", got)
	}
	r.Header.Set("CF-Connecting-IP", "not an address")
	if got := auth.ClientIP(r, true); got != "10.0.0.5" {
		t.Errorf("bad header: %q, want the connection's address", got)
	}
}
