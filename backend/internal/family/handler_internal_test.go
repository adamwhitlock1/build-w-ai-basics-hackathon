package family

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A query cancelled because the browser gave up on the request is no fault
// of ours: no ERROR line and no 500. The same error with the request still
// live is a real failure.
func TestServerErrorWhenTheBrowserGivesUp(t *testing.T) {
	tests := []struct {
		name          string
		clientGone    bool
		wantStatus    int
		wantErrorLine bool
	}{
		{"client gone", true, http.StatusOK, false}, // nothing written: the recorder's default
		{"client still waiting", false, http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.clientGone {
				cancel()
			}
			var logs bytes.Buffer
			h := &Handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/family", nil).WithContext(ctx)
			h.serverError(rec, req, "load family", fmt.Errorf("get family 10: %w", context.Canceled))

			if rec.Code != tt.wantStatus || (tt.clientGone && rec.Body.Len() != 0) {
				t.Errorf("status = %d, body %q; want %d", rec.Code, rec.Body, tt.wantStatus)
			}
			if got := strings.Contains(logs.String(), "level=ERROR"); got != tt.wantErrorLine {
				t.Errorf("ERROR line logged = %v, want %v; logs:\n%s", got, tt.wantErrorLine, logs.String())
			}
		})
	}
}
