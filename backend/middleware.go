package main

import (
	"log/slog"
	"net/http"
	"runtime/debug"
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
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
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
