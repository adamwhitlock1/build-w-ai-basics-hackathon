// Package respond writes JSON HTTP responses in the shapes the frontend
// expects, so every handler reports success and failure the same way.
package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the JSON body of every error response.
type ErrorBody struct {
	Error string `json:"error"` // what went wrong, in plain language
	Hint  string `json:"hint"`  // what to try next; may be empty
	// Code, when set, names a failure the frontend handles on its own,
	// such as the judge demo's pauses (see internal/hosteddemo).
	Code string `json:"code,omitempty"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent, so all we can do is note it.
		slog.Warn("write json response", "err", err)
	}
}

// Error writes an ErrorBody with the given status code.
func Error(w http.ResponseWriter, status int, message, hint string) {
	JSON(w, status, ErrorBody{Error: message, Hint: hint})
}
