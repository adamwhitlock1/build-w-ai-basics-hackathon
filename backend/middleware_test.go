package main

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
)

// Invite tokens are secrets, so request logs show "<token>" instead.
func TestLoggedPath(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/api/invites/abc123", "/api/invites/<token>"},
		{"/api/invites/abc123/accept", "/api/invites/<token>/accept"},
		{"/api/invites/", "/api/invites/"},
		{"/api/me/family", "/api/me/family"},
		{"/api/families/1/invites/2", "/api/families/1/invites/2"}, // an id, not a secret
	}
	for _, tt := range tests {
		if got := loggedPath(tt.path); got != tt.want {
			t.Errorf("loggedPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// A streamed reply (the family agent's, later) must reach the browser as it
// is written, through every middleware wrapper. Each wrapper replaces the
// ResponseWriter, and http.ResponseController finds Flush and
// SetWriteDeadline on the real one by following Unwrap methods.
func TestFlushReachesClientThroughMiddleware(t *testing.T) {
	release := make(chan struct{})
	deadlineErr := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		deadlineErr <- rc.SetWriteDeadline(time.Now().Add(time.Minute))
		io.WriteString(w, "first\n")
		if err := rc.Flush(); err != nil {
			t.Errorf("Flush: %v", err)
		}
		<-release // the client must read "first" before this returns
		io.WriteString(w, "second\n")
	})

	logger := slog.New(slog.DiscardHandler)
	stack := logRequests(logger, recoverPanics(logger, rejectCrossSite(scs.New().LoadAndSave(handler))))
	srv := httptest.NewServer(stack)
	defer srv.Close()

	// If flushing doesn't work, not even the headers arrive and http.Get
	// waits for the handler, which waits for us: give it a deadline.
	type result struct {
		resp *http.Response
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get(srv.URL)
		got <- result{resp, err}
	}()
	var resp *http.Response
	select {
	case r := <-got:
		if r.err != nil {
			t.Fatal(r.err)
		}
		resp = r.resp
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("no response while the handler was still running: the flush didn't reach the client")
	}
	defer resp.Body.Close()
	if err := <-deadlineErr; err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}

	lines := bufio.NewReader(resp.Body)
	line, err := lines.ReadString('\n')
	if err != nil || line != "first\n" {
		t.Errorf("first line = %q (err %v), want %q", line, err, "first\n")
	}
	close(release)
	if rest, _ := io.ReadAll(lines); string(rest) != "second\n" {
		t.Errorf("rest = %q, want %q", rest, "second\n")
	}
}
