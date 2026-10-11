// Package layout serves a member's homepage: their profile, their saved
// page layout, and their custom widgets.
package layout

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"porchlight/internal/auth"
	"porchlight/internal/respond"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// Store is the database access the homepage needs. *store.Store satisfies
// it.
type Store interface {
	GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
	ListCustomWidgets(ctx context.Context, userID int64) ([]store.CustomWidget, error)
}

// Handler serves the homepage endpoints.
type Handler struct {
	store  Store
	logger *slog.Logger
}

// NewHandler returns a Handler backed by st.
func NewHandler(st Store, logger *slog.Logger) *Handler {
	return &Handler{store: st, logger: logger}
}

// Homepage is the response body of GET /api/me/homepage.
type Homepage struct {
	Profile Profile `json:"profile"`
	// Layout is the saved page arrangement. Nothing saves one yet, so it is
	// always nil, which encodes as JSON null.
	Layout        json.RawMessage       `json:"layout"`
	CustomWidgets []widgets.SavedWidget `json:"customWidgets"` // oldest first; never null
}

// Profile is a member's interests and location.
type Profile struct {
	Interests string `json:"interests"`
	Location  string `json:"location"`
}

// Homepage handles GET /api/me/homepage for the logged-in user. It must sit
// behind auth.RequireAuth.
func (h *Handler) Homepage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}

	profile, err := h.store.GetMemberProfile(ctx, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) { // no profile yet reads as an empty one
		h.serverError(w, r, "load profile", err)
		return
	}
	saved, err := h.store.ListCustomWidgets(ctx, user.ID)
	if err != nil {
		h.serverError(w, r, "list widgets", err)
		return
	}

	page := Homepage{
		Profile:       Profile{Interests: profile.Interests, Location: profile.Location},
		CustomWidgets: make([]widgets.SavedWidget, 0, len(saved)),
	}
	for _, cw := range saved {
		widget, err := widgets.FromStore(cw)
		if err != nil {
			// One unreadable widget shouldn't take the whole page down.
			h.logger.WarnContext(ctx, "skipping unreadable widget", "widget_id", cw.ID, "err", err)
			continue
		}
		page.CustomWidgets = append(page.CustomWidgets, widget)
	}
	respond.JSON(w, http.StatusOK, page)
}

// serverError logs err and writes a generic 500.
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}
