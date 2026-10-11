// Package layout serves a member's homepage: their profile, how their page
// is arranged (the layout), and their custom widgets. It also keeps the
// layout in step with their widgets, so a new widget lands on the page and
// a missing one never leaves a hole.
package layout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"porchlight/internal/auth"
	"porchlight/internal/respond"
	"porchlight/internal/store"
	"porchlight/internal/widgets"
)

// maxBodyBytes caps a PUT /api/me/homepage body. A full 50-card layout is
// a few kilobytes, so this leaves plenty of room for whatever extra props
// Puck adds (which are dropped before saving).
const maxBodyBytes = 256 << 10

// saveExample is the 400 message for a body that isn't a layout at all.
const saveExample = `Send JSON like {"layout": {"root": {"props": {"blocks": [...]}}}}.`

// Store is the database access the homepage needs. *store.Store satisfies
// it.
type Store interface {
	GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
	ListCustomWidgets(ctx context.Context, userID int64) ([]store.CustomWidget, error)
	GetLayout(ctx context.Context, userID int64) (json.RawMessage, error)
	UpsertLayout(ctx context.Context, userID int64, puckData json.RawMessage) error
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
	// Layout is how the page is arranged. It is never null: someone who
	// hasn't arranged their page gets Default.
	Layout        Data                  `json:"layout"`
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
	widgetIDs := make([]int64, 0, len(saved))
	for _, cw := range saved {
		widget, err := widgets.FromStore(cw)
		if err != nil {
			// One unreadable widget shouldn't take the whole page down.
			h.logger.WarnContext(ctx, "skipping unreadable widget", "widget_id", cw.ID, "err", err)
			continue
		}
		page.CustomWidgets = append(page.CustomWidgets, widget)
		widgetIDs = append(widgetIDs, widget.ID)
	}

	// The layout is healed against the widgets being sent, so every Custom
	// card in it has its widget in customWidgets.
	page.Layout, err = h.loadLayout(ctx, user.ID, widgetIDs)
	if err != nil {
		h.serverError(w, r, "load layout", err)
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

// loadLayout returns userID's saved layout, healed to match widgetIDs
// (their saved widgets, oldest first), or Default if they have never saved
// one. Nothing is written: the default is only stored once they arrange
// their page.
func (h *Handler) loadLayout(ctx context.Context, userID int64, widgetIDs []int64) (Data, error) {
	saved, err := h.savedLayout(ctx, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return Default(widgetIDs), nil
	case errors.Is(err, errUnreadableLayout):
		// Only this package writes layouts, so this shouldn't happen; if it
		// does, the default page beats an error page.
		h.logger.WarnContext(ctx, "showing the default layout", "user_id", userID, "err", err)
		return Default(widgetIDs), nil
	case err != nil:
		return Data{}, err
	}
	return heal(saved, widgetIDs), nil
}

// errUnreadableLayout means a stored layout isn't valid JSON for Data.
var errUnreadableLayout = errors.New("saved layout is unreadable")

// savedLayout reads userID's stored layout. It returns store.ErrNotFound
// if they have none.
func (h *Handler) savedLayout(ctx context.Context, userID int64) (Data, error) {
	raw, err := h.store.GetLayout(ctx, userID)
	if err != nil {
		return Data{}, err
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		// %w twice (Go 1.20+) lets errors.Is match either error.
		return Data{}, fmt.Errorf("%w: %w", errUnreadableLayout, err)
	}
	return d, nil
}

// saveRequest is the body of PUT /api/me/homepage. Layout is a pointer so
// a missing or null "layout" (nil) can be told apart from an empty one.
type saveRequest struct {
	Layout *incomingData `json:"layout"`
}

// SaveResponse is the response body of SaveLayout.
type SaveResponse struct {
	Layout Data `json:"layout"` // what was saved, normalised
}

// SaveLayout handles PUT /api/me/homepage: it checks the layout in the body,
// saves the normalised version as the logged-in user's layout, and responds
// with it. It must sit behind auth.RequireAuth.
func (h *Handler) SaveLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	var req saveRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Layout == nil {
		respond.Error(w, http.StatusBadRequest, saveExample, "")
		return
	}

	// A Custom card may only show one of the caller's own widgets.
	saved, err := h.store.ListCustomWidgets(ctx, user.ID)
	if err != nil {
		h.serverError(w, r, "list widgets", err)
		return
	}
	owned := make(map[int64]bool, len(saved))
	for _, cw := range saved {
		owned[cw.ID] = true
	}

	layout, msg, hint := parseLayout(*req.Layout, owned)
	if msg != "" {
		respond.Error(w, http.StatusBadRequest, msg, hint)
		return
	}
	if err := h.save(ctx, user.ID, layout); err != nil {
		h.serverError(w, r, "save layout", err)
		return
	}
	respond.JSON(w, http.StatusOK, SaveResponse{Layout: layout})
}

// AddCustomWidget puts a card for a newly saved widget at the end of
// userID's saved layout. widgets.Handler calls it after saving a widget.
// Someone who has never saved a layout has nothing to update: their default
// layout already shows every widget.
//
// Reading, changing, and writing the layout aren't one atomic step, so a
// layout saved from another tab in between can overwrite the new card.
// That's harmless: GET /api/me/homepage adds a card for any widget the
// layout is missing.
func (h *Handler) AddCustomWidget(ctx context.Context, userID, widgetID int64) error {
	saved, err := h.savedLayout(ctx, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil
	case err != nil:
		return fmt.Errorf("add widget %d to layout: %w", widgetID, err)
	}
	if err := h.save(ctx, userID, withCustomWidget(saved, widgetID)); err != nil {
		return fmt.Errorf("add widget %d to layout: %w", widgetID, err)
	}
	return nil
}

// RemoveCustomWidget takes a deleted widget's card off userID's saved
// layout. widgets.Handler calls it after deleting a widget. Someone who has
// never saved a layout has nothing to update.
//
// As with AddCustomWidget, this is tidying rather than a must: GET
// /api/me/homepage drops the card of a widget that no longer exists anyway.
func (h *Handler) RemoveCustomWidget(ctx context.Context, userID, widgetID int64) error {
	saved, err := h.savedLayout(ctx, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil
	case err != nil:
		return fmt.Errorf("remove widget %d from layout: %w", widgetID, err)
	}
	if err := h.save(ctx, userID, withoutCustomWidget(saved, widgetID)); err != nil {
		return fmt.Errorf("remove widget %d from layout: %w", widgetID, err)
	}
	return nil
}

// save stores d as userID's layout.
func (h *Handler) save(ctx context.Context, userID int64, d Data) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode layout: %w", err)
	}
	return h.store.UpsertLayout(ctx, userID, raw)
}

// serverError logs err and writes a generic 500.
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}

// decodeJSON reads exactly one JSON value from the request body into dst.
// On a bad body it writes a 400 and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dst)
	if err == nil {
		// A second Decode must hit EOF, or the body had trailing junk.
		if err = dec.Decode(&struct{}{}); errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("more than one JSON value")
		}
	}
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		respond.Error(w, http.StatusBadRequest, "That layout is too big to save.", "")
		return false
	case err != nil:
		respond.Error(w, http.StatusBadRequest, saveExample, "")
		return false
	}
	return true
}
