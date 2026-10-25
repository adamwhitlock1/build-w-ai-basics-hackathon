package auth_test

import (
	"bytes"
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

	"kinlight/internal/auth"
	"kinlight/internal/store"
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

func TestRequireMember(t *testing.T) {
	parent := store.User{ID: 1, Email: "parent@example.com", FamilyID: 10, IsAdmin: true}
	kid := store.User{ID: 2, Email: "kid@example.com", FamilyID: 10}
	stranger := store.User{ID: 3, Email: "stranger@example.com", FamilyID: 20, IsAdmin: true}
	users := fakeUsers{users: map[int64]store.User{parent.ID: parent, kid.ID: kid, stranger.ID: stranger}}

	tests := []struct {
		name        string
		id          string // the path's {id}
		users       fakeUsers
		wantStatus  int
		wantSubject int64
	}{
		{"a member of the family", "2", users, http.StatusOK, kid.ID},
		{"their own id", "1", users, http.StatusOK, parent.ID},
		{"someone in another family", "3", users, http.StatusNotFound, 0},
		{"nobody", "99", users, http.StatusNotFound, 0},
		{"not a number", "kid", users, http.StatusNotFound, 0},
		{"database down", "2", fakeUsers{err: errors.New("disk on fire")}, http.StatusInternalServerError, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			authn := auth.New(auth.NewSessionManager(memstore.New(), logger), tt.users, logger)

			var subject, viewer store.User
			handler := authn.RequireMember(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				subject, _ = auth.SubjectFrom(r.Context())
				viewer, _ = auth.UserFrom(r.Context())
			}))

			// The parent is logged in (as RequireAuth would have arranged),
			// and the router has matched {id}.
			req := httptest.NewRequest(http.MethodGet, "/api/members/"+tt.id+"/homepage", nil)
			req = req.WithContext(auth.WithUser(req.Context(), parent))
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if rec.Code == http.StatusOK && (subject.ID != tt.wantSubject || viewer.ID != parent.ID) {
				t.Errorf("handler saw subject %d and viewer %d, want subject %d and viewer %d", subject.ID, viewer.ID, tt.wantSubject, parent.ID)
			}
			// Another family's member gets exactly the same answer as nobody.
			if rec.Code == http.StatusNotFound && !strings.Contains(rec.Body.String(), `"error":"That family member doesn't exist."`) {
				t.Errorf("404 body = %s", rec.Body)
			}
		})
	}
}

// Without RequireMember, the subject is the logged-in user: that's what
// lets the /api/me handlers read SubjectFrom and behave as before.
func TestSubjectFallsBackToTheLoggedInUser(t *testing.T) {
	parent := store.User{ID: 1, FamilyID: 10, IsAdmin: true}
	kid := store.User{ID: 2, FamilyID: 10}

	if _, ok := auth.SubjectFrom(context.Background()); ok {
		t.Error("SubjectFrom found a subject in an empty context")
	}
	ctx := auth.WithUser(context.Background(), parent)
	if got, ok := auth.SubjectFrom(ctx); !ok || got.ID != parent.ID {
		t.Errorf("SubjectFrom = %d, %v; want the logged-in user %d", got.ID, ok, parent.ID)
	}
	ctx = auth.WithSubject(ctx, kid)
	if got, ok := auth.SubjectFrom(ctx); !ok || got.ID != kid.ID {
		t.Errorf("SubjectFrom = %d, %v; want the member %d", got.ID, ok, kid.ID)
	}
	if got, _ := auth.UserFrom(ctx); got.ID != parent.ID {
		t.Errorf("UserFrom = %d, want the logged-in user %d still", got.ID, parent.ID)
	}
}

// A browser that gives up on a request (a router preload, a StrictMode
// remount) cancels its context mid-query. That's no fault of ours: no
// ERROR line and no 500. The same error with the request still live is a
// real failure, and is still logged.
func TestRequireAuthWhenTheBrowserGivesUp(t *testing.T) {
	kid := store.User{ID: 3, Email: "kid@example.com"}
	cancelled := fakeUsers{err: fmt.Errorf("get user %d: %w", kid.ID, context.Canceled)}

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
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			sessions := auth.NewSessionManager(memstore.New(), logger)
			authn := auth.New(sessions, cancelled, logger)

			reached := false
			handler := authn.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
			}))

			ctx, err := sessions.Load(context.Background(), "")
			if err != nil {
				t.Fatal(err)
			}
			if err := authn.LogIn(ctx, kid.ID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if tt.clientGone {
				cancel()
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil).WithContext(ctx))

			if reached {
				t.Error("the handler ran without a logged-in user")
			}
			if rec.Code != tt.wantStatus || (tt.clientGone && rec.Body.Len() != 0) {
				t.Errorf("status = %d, body %q; want %d", rec.Code, rec.Body, tt.wantStatus)
			}
			if got := strings.Contains(logs.String(), "level=ERROR"); got != tt.wantErrorLine {
				t.Errorf("ERROR line logged = %v, want %v; logs:\n%s", got, tt.wantErrorLine, logs.String())
			}
		})
	}
}
