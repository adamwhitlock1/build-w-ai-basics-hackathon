package main

import (
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"porchlight/internal/respond"
)

// Middleware in Go is just a function that takes an http.Handler and returns
// another one that does some work before and/or after calling it.

// logRequests logs one line per request with its status and duration.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", loggedPath(r.URL.Path),
			"status", rec.status,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

// invitePathPrefix starts every path that carries an invite token.
const invitePathPrefix = "/api/invites/"

// loggedPath is path as logRequests writes it to the log. An invite path
// carries the invite's secret token, and anyone who could read the log
// shouldn't be able to accept someone else's invite with it, so the token
// is swapped for "<token>":
//
//	/api/invites/abc123/accept -> /api/invites/<token>/accept
func loggedPath(path string) string {
	rest, ok := strings.CutPrefix(path, invitePathPrefix)
	if !ok || rest == "" {
		return path
	}
	if _, after, found := strings.Cut(rest, "/"); found {
		return invitePathPrefix + "<token>/" + after
	}
	return invitePathPrefix + "<token>"
}

// statusRecorder remembers the status code a handler wrote. Embedding
// http.ResponseWriter gives it every ResponseWriter method for free; we
// override only WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader records only the first status: that's the one the client
// gets, and net/http ignores (and logs) any later call.
func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the wrapped ResponseWriter. Embedding an interface only
// promotes that interface's methods, so a statusRecorder has no Flush or
// SetWriteDeadline of its own even when the real writer does.
// http.ResponseController looks for Unwrap and follows it down to the real
// writer, so a handler can still stream with rc.Flush() (Go 1.20+).
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// rejectCrossSite turns away requests that another website could have sent
// with this browser's session cookie (cross-site request forgery). The
// cookie's SameSite=Lax setting can't do this alone: browsers ignore ports
// when deciding what is "same-site", so a page served from localhost:5173
// may send requests here with the cookie attached.
//
// Reads need guarding as well as writes: GET /api/widgets/articles and
// /events spend LLM and search credit on every call, so even an <img> tag
// on another page pointed at them would cost money.
//
// Three checks:
//   - Current browsers label every request with Sec-Fetch-Site. If it says
//     the request came from another origin ("cross-site", or "same-site",
//     which covers another port on localhost), refuse it, whatever the
//     method. Without the header (curl, an older browser) the request goes
//     on to the checks below.
//   - Browsers send an Origin header with every cross-origin POST. If it
//     names a different host than the request went to, refuse it.
//   - A request body must be JSON. A page on another origin can only send
//     Content-Type: application/json after a CORS "preflight" check, which
//     this server never approves.
//
// The Vite dev proxy passes the browser's Sec-Fetch-Site through and keeps
// its Host header (it doesn't set changeOrigin), so requests from the app
// itself ("same-origin") pass every check.
func rejectCrossSite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Sec-Fetch-Site") {
		case "cross-site", "same-site":
			respond.Error(w, http.StatusForbidden, "That request came from another site.", "")
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r) // no body to check
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !sameHost(origin, r.Host) {
			respond.Error(w, http.StatusForbidden, "That request came from another site.", "")
			return
		}
		// ContentLength is 0 for no body and -1 when the length is unknown.
		if r.ContentLength != 0 && !isJSON(r.Header.Get("Content-Type")) {
			respond.Error(w, http.StatusUnsupportedMediaType, "Send the request body as JSON.", "Set the header Content-Type: application/json.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameHost reports whether origin (like "http://localhost:5174") names host
// (like "localhost:5174").
func sameHost(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, host)
}

// isJSON reports whether a Content-Type header value is application/json,
// with or without parameters such as "; charset=utf-8".
func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

// recoverPanics turns a panic in a handler into a logged 500 instead of a
// dropped connection.
func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { // net/http's deliberate "abort quietly" panic
				panic(v)
			}
			logger.Error("panic in handler", "panic", v, "path", r.URL.Path, "stack", string(debug.Stack()))
			respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
		}()
		next.ServeHTTP(w, r)
	})
}
