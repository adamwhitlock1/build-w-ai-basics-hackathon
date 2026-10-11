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
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
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

// rejectCrossSite turns away state-changing requests that another website
// could have sent with this browser's session cookie (cross-site request
// forgery). The cookie's SameSite=Lax setting can't do this alone: browsers
// ignore ports when deciding what is "same-site", so a page served from
// localhost:5173 may POST here with the cookie attached.
//
// Two checks, each enough on its own in a current browser:
//   - Browsers send an Origin header with every cross-origin POST. If it
//     names a different host than the request went to, refuse it.
//   - A request body must be JSON. A page on another origin can only send
//     Content-Type: application/json after a CORS "preflight" check, which
//     this server never approves.
//
// The Vite dev proxy keeps the browser's Host header (it doesn't set
// changeOrigin), so requests from the app itself pass the Origin check.
func rejectCrossSite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r) // these only read, so there's nothing to forge
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
