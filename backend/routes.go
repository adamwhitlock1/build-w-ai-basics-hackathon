package main

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"

	"kinlight/internal/assistant"
	"kinlight/internal/auth"
	"kinlight/internal/chat"
	"kinlight/internal/family"
	"kinlight/internal/layout"
	"kinlight/internal/respond"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// deps is everything routes needs to build the API. main fills it once;
// tests fill it with fakes for the LLM (Planner, Curator and Briefer), the
// search sources and the calendar fetcher. Named fields keep call sites
// readable where a long list of positional arguments would not.
type deps struct {
	Logger    *slog.Logger
	Store     *store.Store
	Sessions  *scs.SessionManager
	Planner   widgets.Planner
	Curator   widgets.Curator
	Briefer   widgets.Briefer
	Sources   widgets.SourceFinder
	Calendars widgets.CalendarFetcher
	// Chat is kid chat. nil means kid chat is off (as with no kids' key):
	// routes builds a Service with no agent.
	Chat *chat.Service
	// Assistant is the parents' Assistant. nil means it's off: routes
	// builds a Service with no agents.
	Assistant *assistant.Service
}

// routes builds the API's HTTP handler: every endpoint, wrapped in the
// session and logging middleware.
func routes(d deps) http.Handler {
	logger, st, sessions, srcs := d.Logger, d.Store, d.Sessions, d.Sources
	planner, curator, briefer, calendars := d.Planner, d.Curator, d.Briefer, d.Calendars
	authn := auth.New(sessions, st, logger)
	familyHandler := family.NewHandler(st, authn, logger)
	layoutHandler := layout.NewHandler(st, logger)
	// New widgets go onto the member's saved layout, and deleted ones come
	// off it, through layoutHandler.
	widgetHandler := widgets.NewHandler(planner, srcs, st, st, layoutHandler, logger)
	curatedHandler := widgets.NewCuratedHandler(curator, srcs, st, st, logger)
	todayHandler := widgets.NewTodayHandler(calendars, st, logger)
	briefingHandler := widgets.NewBriefingHandler(briefer, srcs, st, st, logger)
	chatService := d.Chat
	if chatService == nil {
		chatService = chat.NewService(nil, st, logger)
	}
	chatHandler := chat.NewHandler(chatService, logger)
	assistantService := d.Assistant
	if assistantService == nil {
		assistantService = assistant.NewService(nil, nil, st, logger)
	}
	assistantHandler := assistant.NewHandler(assistantService, logger)

	// private wraps a handler that needs a logged-in user; adminOnly also
	// needs that user to be a family admin. A method value such as
	// familyHandler.Me converts to http.HandlerFunc automatically.
	private := func(h http.HandlerFunc) http.Handler { return authn.RequireAuth(h) }
	adminOnly := func(h http.HandlerFunc) http.Handler { return authn.RequireAuth(auth.RequireAdmin(h)) }
	// parentsOnly is adminOnly with a 403 that says what only parents can do.
	parentsOnly := func(message string, h http.HandlerFunc) http.Handler {
		return authn.RequireAuth(auth.RequireAdminWithMessage(message, "Ask a parent in your family.", h))
	}
	// asParent serves h on a parent's behalf for the family member in the
	// path's {id}. Each wrapper does one job, outermost first: RequireAuth
	// finds who's asking, the admin check turns away anyone who isn't a
	// parent, and RequireMember looks up member {id} (404 unless they're in
	// the parent's family) and makes them the request's subject, which h
	// reads with auth.SubjectFrom.
	//
	// The admin check comes first, so a kid gets a 403 even for their own
	// id: these routes are how parents see other people's pages, and a kid
	// already has their own page through the routes under "Logged in".
	asParent := func(h http.HandlerFunc) http.Handler {
		return authn.RequireAuth(auth.RequireAdminWithMessage(
			"Only parents can see other family members' pages.", "Ask a parent in your family.",
			authn.RequireMember(h)))
	}

	// Go 1.22 patterns put the HTTP method in the route itself, and {id}
	// is a wildcard the handler reads with r.PathValue("id").
	mux := http.NewServeMux()

	// Public.
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/signup", familyHandler.Signup)
	mux.HandleFunc("POST /api/login", familyHandler.Login)
	mux.HandleFunc("POST /api/logout", familyHandler.Logout)
	// Invite links work for people who don't have an account yet. The
	// token in the path is the secret; logRequests keeps it out of the log.
	mux.HandleFunc("GET /api/invites/{token}", familyHandler.ShowInvite)
	mux.HandleFunc("POST /api/invites/{token}/accept", familyHandler.AcceptInvite)

	// Logged in.
	mux.Handle("GET /api/me", private(familyHandler.Me))
	mux.Handle("GET /api/me/family", private(familyHandler.Family))
	mux.Handle("GET /api/me/homepage", private(layoutHandler.Homepage))
	mux.Handle("PUT /api/me/homepage", private(layoutHandler.SaveLayout))
	mux.Handle("PUT /api/me/profile", private(familyHandler.UpdateProfile))
	mux.Handle("GET /api/widgets/articles", private(curatedHandler.Articles))
	mux.Handle("GET /api/widgets/events", private(curatedHandler.Events))
	mux.Handle("GET /api/widgets/today", private(todayHandler.Today))
	mux.Handle("GET /api/widgets/briefing", private(briefingHandler.Briefing))
	// The calendar link travels in the body, never the path, so the request
	// log can't record it.
	mux.Handle("PUT /api/me/calendar", private(todayHandler.SetCalendar))
	mux.Handle("DELETE /api/me/calendar", private(todayHandler.RemoveCalendar))
	mux.Handle("POST /api/widgets/from-prompt", private(widgetHandler.FromPrompt))
	mux.Handle("GET /api/widgets/custom/{id}", private(widgetHandler.GetCustom))
	mux.Handle("POST /api/widgets/custom/{id}/refresh", private(widgetHandler.RefreshCustom))
	mux.Handle("DELETE /api/widgets/custom/{id}", private(widgetHandler.DeleteCustom))
	// Kid chat. The service refuses parents (the parents' assistant comes
	// later) and keeps each kid to their own conversations.
	mux.Handle("GET /api/me/chats", private(chatHandler.List))
	mux.Handle("POST /api/me/chats", private(chatHandler.Start))
	mux.Handle("GET /api/me/chats/{id}", private(chatHandler.Get))
	mux.Handle("POST /api/me/chats/{id}/messages", private(chatHandler.Send))
	// The parents' Assistant. The service refuses kids, and keeps each
	// parent to their own conversations (the other parent's included).
	mux.Handle("GET /api/me/assistant", private(assistantHandler.List))
	mux.Handle("POST /api/me/assistant", private(assistantHandler.Start))
	mux.Handle("GET /api/me/assistant/{id}", private(assistantHandler.Get))
	mux.Handle("DELETE /api/me/assistant/{id}", private(assistantHandler.Delete))
	mux.Handle("POST /api/me/assistant/{id}/messages", private(assistantHandler.Send))

	// A parent looking at someone in their family, and setting their
	// interests and location. These reuse the handlers above, which act on
	// the request's subject: the logged-in user there, member {id} here.
	// There are deliberately no member routes for changing a layout or
	// widgets: parents look, and only edit interests, location and calendar.
	mux.Handle("GET /api/members/{id}/homepage", asParent(layoutHandler.Homepage))
	mux.Handle("PUT /api/members/{id}/profile", asParent(familyHandler.UpdateProfile))
	mux.Handle("GET /api/members/{id}/widgets/articles", asParent(curatedHandler.Articles))
	mux.Handle("GET /api/members/{id}/widgets/events", asParent(curatedHandler.Events))
	mux.Handle("GET /api/members/{id}/widgets/today", asParent(todayHandler.Today))
	mux.Handle("GET /api/members/{id}/widgets/briefing", asParent(briefingHandler.Briefing))
	mux.Handle("PUT /api/members/{id}/calendar", asParent(todayHandler.SetCalendar))
	mux.Handle("DELETE /api/members/{id}/calendar", asParent(todayHandler.RemoveCalendar))
	// A parent reading and deleting a kid's chats, and setting a kid's
	// birth year (which picks the agent's persona; kids can't set it).
	mux.Handle("GET /api/members/{id}/chats", asParent(chatHandler.ListFor))
	mux.Handle("GET /api/members/{id}/chats/{chatId}", asParent(chatHandler.Transcript))
	mux.Handle("DELETE /api/members/{id}/chats/{chatId}", asParent(chatHandler.Delete))
	mux.Handle("GET /api/members/{id}/birth-year", asParent(chatHandler.BirthYear))
	mux.Handle("PUT /api/members/{id}/birth-year", asParent(chatHandler.SetBirthYear))

	// Family admins.
	mux.Handle("POST /api/families", adminOnly(familyHandler.CreateFamily))
	mux.Handle("POST /api/families/{id}/invites", parentsOnly("Only parents can invite family members.", familyHandler.CreateInvite))
	mux.Handle("DELETE /api/families/{id}/invites/{inviteId}", parentsOnly("Only parents can manage invites.", familyHandler.RevokeInvite))
	// The family's review queue and alerts for kid chat.
	mux.Handle("GET /api/family/reviews", parentsOnly("Only parents can review kids' chats.", chatHandler.Reviews))
	mux.Handle("POST /api/family/reviews/{flagId}/reviewed", parentsOnly("Only parents can review kids' chats.", chatHandler.MarkReviewed))
	mux.Handle("GET /api/family/alerts", parentsOnly("Only parents can review kids' chats.", chatHandler.Alerts))

	// The first wrapper listed runs first: log, recover from panics, turn
	// away forged cross-site requests, then load the session (and save it as
	// the response is written).
	return logRequests(logger, recoverPanics(logger, rejectCrossSite(sessions.LoadAndSave(mux))))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
