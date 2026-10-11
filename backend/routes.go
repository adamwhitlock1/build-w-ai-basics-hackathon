package main

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"

	"porchlight/internal/auth"
	"porchlight/internal/family"
	"porchlight/internal/layout"
	"porchlight/internal/respond"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// routes builds the API's HTTP handler: every endpoint, wrapped in the
// session and logging middleware. Tests call it with fakes for the LLM and
// the search sources.
func routes(logger *slog.Logger, st *store.Store, sessions *scs.SessionManager, planner widgets.Planner, srcs widgets.SourceFinder) http.Handler {
	authn := auth.New(sessions, st, logger)
	familyHandler := family.NewHandler(st, authn, logger)
	layoutHandler := layout.NewHandler(st, logger)
	widgetHandler := widgets.NewHandler(planner, srcs, st, logger)

	// private wraps a handler that needs a logged-in user; adminOnly also
	// needs that user to be a family admin. A method value such as
	// familyHandler.Me converts to http.HandlerFunc automatically.
	private := func(h http.HandlerFunc) http.Handler { return authn.RequireAuth(h) }
	adminOnly := func(h http.HandlerFunc) http.Handler { return authn.RequireAuth(auth.RequireAdmin(h)) }

	// Go 1.22 patterns put the HTTP method in the route itself, and {id}
	// is a wildcard the handler reads with r.PathValue("id").
	mux := http.NewServeMux()

	// Public.
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/signup", familyHandler.Signup)
	mux.HandleFunc("POST /api/login", familyHandler.Login)
	mux.HandleFunc("POST /api/logout", familyHandler.Logout)

	// Logged in.
	mux.Handle("GET /api/me", private(familyHandler.Me))
	mux.Handle("GET /api/me/homepage", private(layoutHandler.Homepage))
	mux.Handle("POST /api/widgets/from-prompt", private(widgetHandler.FromPrompt))
	mux.Handle("GET /api/widgets/custom/{id}", private(widgetHandler.GetCustom))

	// Family admins.
	mux.Handle("POST /api/families", adminOnly(familyHandler.CreateFamily))

	// The first wrapper listed runs first: log, recover from panics, turn
	// away forged cross-site writes, then load the session (and save it as
	// the response is written).
	return logRequests(logger, recoverPanics(logger, rejectCrossSite(sessions.LoadAndSave(mux))))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
