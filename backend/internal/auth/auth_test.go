package auth_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2/memstore"

	"porchlight/internal/auth"
	"porchlight/internal/store"
)

func TestPasswords(t *testing.T) {
	hash, err := auth.HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		plain string
		hash  []byte
		want  bool
	}{
		{"right password", "correct horse", hash, true},
		{"wrong password", "wrong horse", hash, false},
		{"no such user", "correct horse", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.CheckPassword(tt.plain, tt.hash); got != tt.want {
				t.Errorf("CheckPassword = %v, want %v", got, tt.want)
			}
		})
	}

	if _, err := auth.HashPassword(strings.Repeat("a", auth.MaxPasswordBytes+1)); err == nil {
		t.Error("HashPassword accepted a password longer than bcrypt can check")
	}
}

// fakeUsers is an auth.UserGetter backed by a map.
type fakeUsers struct {
	users map[int64]store.User
	err   error
}

func (f fakeUsers) GetUser(_ context.Context, id int64) (store.User, error) {
	if f.err != nil {
		return store.User{}, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return store.User{}, fmt.Errorf("get user %d: %w", id, store.ErrNotFound)
	}
	return u, nil
}

func TestRequireAuthAndAdmin(t *testing.T) {
	parent := store.User{ID: 1, Email: "parent@example.com", IsAdmin: true}
	kid := store.User{ID: 2, Email: "kid@example.com"}
	users := fakeUsers{users: map[int64]store.User{parent.ID: parent, kid.ID: kid}}

	tests := []struct {
		name       string
		loggedInAs int64 // 0 = not logged in
		users      fakeUsers
		adminOnly  bool
		wantStatus int
	}{
		{"not logged in", 0, users, false, http.StatusUnauthorized},
		{"logged in", kid.ID, users, false, http.StatusOK},
		{"account deleted since login", 99, users, false, http.StatusUnauthorized},
		{"database down", kid.ID, fakeUsers{err: errors.New("disk on fire")}, false, http.StatusInternalServerError},
		{"admin route, admin", parent.ID, users, true, http.StatusOK},
		{"admin route, not admin", kid.ID, users, true, http.StatusForbidden},
		{"admin route, not logged in", 0, users, true, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			sessions := auth.NewSessionManager(memstore.New(), logger)
			authn := auth.New(sessions, tt.users, logger)

			var seen store.User
			var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen, _ = auth.UserFrom(r.Context())
			})
			if tt.adminOnly {
				handler = auth.RequireAdmin(handler)
			}
			handler = authn.RequireAuth(handler)

			// Load attaches an empty session to the context, as the
			// LoadAndSave middleware does for a real request.
			ctx, err := sessions.Load(context.Background(), "")
			if err != nil {
				t.Fatal(err)
			}
			if tt.loggedInAs != 0 {
				if err := authn.LogIn(ctx, tt.loggedInAs); err != nil {
					t.Fatal(err)
				}
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil).WithContext(ctx))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if rec.Code == http.StatusOK && seen.ID != tt.loggedInAs {
				t.Errorf("handler saw user %d, want %d", seen.ID, tt.loggedInAs)
			}
			if rec.Code == http.StatusUnauthorized && !strings.Contains(rec.Body.String(), `"error":"Please log in."`) {
				t.Errorf("401 body = %s", rec.Body)
			}
		})
	}
}
