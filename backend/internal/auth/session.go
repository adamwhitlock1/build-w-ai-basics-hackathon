package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"

	"porchlight/internal/respond"
	"porchlight/internal/store"
)

const (
	// CookieName is the name of the session cookie.
	CookieName = "porchlight_session"
	// SessionLifetime is how long a login lasts.
	SessionLifetime = 30 * 24 * time.Hour

	// userIDKey is the only value Porchlight keeps in a session.
	userIDKey = "userID"
)

// NewSessionManager returns an scs session manager that keeps session data
// in sessionStore and gives the browser only a random token, in an HttpOnly
// cookie. Wrap the router in its LoadAndSave middleware.
func NewSessionManager(sessionStore scs.Store, logger *slog.Logger) *scs.SessionManager {
	sm := scs.New()
	sm.Store = sessionStore
	sm.Lifetime = SessionLifetime
	sm.Cookie.Name = CookieName
	sm.Cookie.HttpOnly = true // page JavaScript can't read it
	// Lax: not sent with POSTs from other sites. But browsers ignore ports
	// here, so every localhost port counts as the same site; main's
	// rejectCrossSite middleware covers that gap.
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = false // we serve plain http:// locally; true behind HTTPS
	sm.Cookie.Persist = true // survives closing the browser
	sm.ErrorFunc = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.ErrorContext(r.Context(), "session store failed", "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
	}
	return sm
}

// UserGetter loads a user by id. *store.Store satisfies it.
type UserGetter interface {
	GetUser(ctx context.Context, id int64) (store.User, error)
}

// Authenticator connects sessions to users. It logs users in and out, and
// its RequireAuth middleware loads the logged-in user for each request.
type Authenticator struct {
	sessions *scs.SessionManager
	users    UserGetter
	logger   *slog.Logger
}

// New returns an Authenticator that keeps logins in sessions and loads
// users from users.
func New(sessions *scs.SessionManager, users UserGetter, logger *slog.Logger) *Authenticator {
	// Pay for the dummy hash now, at startup, so the first login for an
	// unknown email isn't slower than the rest (see CheckPassword).
	dummyHash()
	return &Authenticator{sessions: sessions, users: users, logger: logger}
}

// LogIn makes the current session belong to userID. ctx must come from a
// request that went through the session manager's LoadAndSave middleware.
func (a *Authenticator) LogIn(ctx context.Context, userID int64) error {
	// A fresh token at login prevents session fixation: a token someone
	// planted in the browser beforehand is worthless afterwards.
	if err := a.sessions.RenewToken(ctx); err != nil {
		return fmt.Errorf("renew session token: %w", err)
	}
	a.sessions.Put(ctx, userIDKey, userID)

	// LoadAndSave would save the session as the response starts, after the
	// handler has already chosen its status. Saving it now means a database
	// failure comes back here, as an error the handler can answer. (Writing
	// the response still sends the cookie, saving the session once more.)
	if _, _, err := a.sessions.Commit(ctx); err != nil {
		// Best effort: drop the unsaved session, so LoadAndSave doesn't
		// retry the save while the handler writes its error response.
		_ = a.sessions.Destroy(ctx)
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

// LogOut ends the current session. It's fine to call when nobody is
// logged in.
func (a *Authenticator) LogOut(ctx context.Context) error {
	if err := a.sessions.Destroy(ctx); err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	return nil
}

// RequireAuth only lets requests from a logged-in user through, and puts
// that user in the request context (read it with UserFrom). Everyone else
// gets a 401. Every private handler goes behind it: the frontend's route
// guards are only for show.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := a.sessions.GetInt64(ctx, userIDKey) // 0 when nobody is logged in
		if userID == 0 {
			writeLoginRequired(w)
			return
		}

		user, err := a.users.GetUser(ctx, userID)
		switch {
		case errors.Is(err, store.ErrNotFound): // the session outlived its account
			writeLoginRequired(w)
			return
		case err != nil:
			a.logger.ErrorContext(ctx, "load logged-in user", "user_id", userID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(ctx, user)))
	})
}

// RequireAdmin only lets family admins through; other users get a 403. It
// reads the user that RequireAuth stored, so it must sit inside RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFrom(r.Context())
		switch {
		case !ok:
			writeLoginRequired(w)
		case !user.IsAdmin:
			respond.Error(w, http.StatusForbidden, "Only a family admin can do that.", "Ask an admin in your family.")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func writeLoginRequired(w http.ResponseWriter) {
	respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
}

// userKey is the context key for the logged-in user. Because the type is
// unexported, no other package can build the same key, so nothing else can
// read or overwrite this value by accident.
type userKey struct{}

// WithUser returns a copy of ctx that carries user. RequireAuth uses it;
// tests use it to fake a logged-in request.
func WithUser(ctx context.Context, user store.User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// UserFrom returns the logged-in user stored in ctx by RequireAuth. ok is
// false if there isn't one.
func UserFrom(ctx context.Context) (user store.User, ok bool) {
	user, ok = ctx.Value(userKey{}).(store.User)
	return user, ok
}
