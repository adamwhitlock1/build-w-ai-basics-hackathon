// Package family serves the account and family endpoints: sign up, log in,
// log out, "who am I", the create-family intro, and editing your interests
// and location.
package family

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"porchlight/internal/auth"
	"porchlight/internal/respond"
	"porchlight/internal/store"
)

// Limits on what the create-family intro accepts.
const (
	maxFamilyNameRunes = 80
	maxProfileRunes    = 500 // each of interests and location
	maxCoAdmins        = 10
	inviteLifetime     = 14 * 24 * time.Hour
)

// maxBodyBytes caps request bodies; these forms are small.
const maxBodyBytes = 16 << 10

// Store is the database access these handlers need. *store.Store satisfies
// it; declaring the interface here, where it's used, keeps it as small as
// this package's needs.
type Store interface {
	CreateFamilyWithAdmin(ctx context.Context, email string, passwordHash []byte) (store.User, error)
	GetUserByEmail(ctx context.Context, email string) (store.User, error)
	GetFamily(ctx context.Context, id int64) (store.Family, error)
	GetMemberProfile(ctx context.Context, userID int64) (store.MemberProfile, error)
	UpsertMemberProfile(ctx context.Context, p store.MemberProfile) error
	CompleteFamilySetup(ctx context.Context, setup store.FamilySetup) error
}

// Sessions logs users in and out. *auth.Authenticator satisfies it.
type Sessions interface {
	LogIn(ctx context.Context, userID int64) error
	LogOut(ctx context.Context) error
}

// Handler serves the account and family endpoints.
type Handler struct {
	store    Store
	sessions Sessions
	logger   *slog.Logger
}

// NewHandler returns a Handler backed by st and sessions.
func NewHandler(st Store, sessions Sessions, logger *slog.Logger) *Handler {
	return &Handler{store: st, sessions: sessions, logger: logger}
}

// Me describes the logged-in user, their family, and their profile. Signup,
// Login, Me, and CreateFamily all respond with it.
type Me struct {
	User   MeUser   `json:"user"`
	Family MeFamily `json:"family"`
	// NeedsFamilySetup is true until the create-family intro is done.
	NeedsFamilySetup bool      `json:"needsFamilySetup"`
	Profile          MeProfile `json:"profile"`
}

// MeUser is the public part of a user. It has no password hash, by
// construction: only fields listed here are ever sent.
type MeUser struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"isAdmin"`
}

// MeFamily is the user's family.
type MeFamily struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// MeProfile is the user's interests and location.
type MeProfile struct {
	Interests string `json:"interests"`
	Location  string `json:"location"`
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const credentialsExample = `Send JSON like {"email": "you@example.com", "password": "..."}.`

// Signup handles POST /api/signup: it creates a family, makes the new user
// its admin, logs them in, and responds 201 with Me.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if !decodeJSON(w, r, &req, credentialsExample) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "That doesn't look like an email address.", "Check it for typos, like a missing @ or dot.")
		return
	}
	if msg, hint := passwordProblem(req.Password); msg != "" {
		respond.Error(w, http.StatusBadRequest, msg, hint)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.serverError(w, r, "hash password", err)
		return
	}
	ctx := r.Context()
	user, err := h.store.CreateFamilyWithAdmin(ctx, email, hash)
	switch {
	case errors.Is(err, store.ErrEmailTaken):
		respond.Error(w, http.StatusConflict, "That email already has an account.", "Try logging in instead.")
		return
	case err != nil:
		h.serverError(w, r, "create account", err)
		return
	}

	// Log in before writing the response: the session cookie is sent with
	// the response headers.
	if err := h.sessions.LogIn(ctx, user.ID); err != nil {
		h.logger.ErrorContext(ctx, "log in after signup", "user_id", user.ID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Your account was created, but we couldn't log you in.", "Try logging in.")
		return
	}
	h.logger.InfoContext(ctx, "signed up", "user_id", user.ID, "family_id", user.FamilyID)
	h.writeMe(w, r, http.StatusCreated, user)
}

// Login handles POST /api/login. A wrong email and a wrong password get the
// same 401, so the response doesn't reveal which emails have accounts.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if !decodeJSON(w, r, &req, credentialsExample) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		respond.Error(w, http.StatusBadRequest, "Enter your email and password.", "")
		return
	}

	ctx := r.Context()
	user, err := h.store.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.serverError(w, r, "look up user", err)
		return
	}
	// For an unknown email, user is the zero User: its nil PasswordHash
	// makes CheckPassword spend the same time and return false.
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		respond.Error(w, http.StatusUnauthorized, "That email and password don't match.", "")
		return
	}

	if err := h.sessions.LogIn(ctx, user.ID); err != nil {
		h.serverError(w, r, "log in", err)
		return
	}
	h.writeMe(w, r, http.StatusOK, user)
}

// Logout handles POST /api/logout. It always succeeds with 204, logged in
// or not.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.LogOut(r.Context()); err != nil {
		h.serverError(w, r, "log out", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/me. It must sit behind auth.RequireAuth, which
// answers 401 for anyone not logged in.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	h.writeMe(w, r, http.StatusOK, user)
}

type createFamilyRequest struct {
	Name          string   `json:"name"`
	CoAdminEmails []string `json:"coAdminEmails"`
	Interests     string   `json:"interests"`
	Location      string   `json:"location"`
}

// CreateFamily handles POST /api/families, the create-family intro: it
// names the caller's family, saves their interests and location, and
// records a pending admin invite for each co-admin email. It must sit
// behind auth.RequireAuth and auth.RequireAdmin.
func (h *Handler) CreateFamily(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	var req createFamilyRequest
	if !decodeJSON(w, r, &req, `Send JSON like {"name": "The Robinsons", "coAdminEmails": [], "interests": "...", "location": "..."}.`) {
		return
	}

	name := strings.TrimSpace(req.Name)
	switch {
	case name == "":
		respond.Error(w, http.StatusBadRequest, "Give your family a name.", `For example, "The Robinsons".`)
		return
	case utf8.RuneCountInString(name) > maxFamilyNameRunes:
		respond.Error(w, http.StatusBadRequest, "That family name is too long.", fmt.Sprintf("Keep it under %d characters.", maxFamilyNameRunes))
		return
	}
	profile, msg, hint := cleanProfile(req.Interests, req.Location)
	if msg != "" {
		respond.Error(w, http.StatusBadRequest, msg, hint)
		return
	}

	coAdmins, bad := cleanEmails(req.CoAdminEmails, user.Email)
	switch {
	case bad != "":
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("%q doesn't look like an email address.", bad), "Separate co-admin emails with commas.")
		return
	case len(coAdmins) > maxCoAdmins:
		respond.Error(w, http.StatusBadRequest, "That's more co-admins than we can invite at once.", fmt.Sprintf("Add up to %d now.", maxCoAdmins))
		return
	}

	expires := time.Now().Add(inviteLifetime)
	invites := make([]store.Invite, 0, len(coAdmins))
	for _, email := range coAdmins {
		token, err := newInviteToken()
		if err != nil {
			h.serverError(w, r, "create invite token", err)
			return
		}
		invites = append(invites, store.Invite{Email: email, Token: token, IsAdmin: true, ExpiresAt: expires})
	}

	err := h.store.CompleteFamilySetup(r.Context(), store.FamilySetup{
		FamilyID: user.FamilyID,
		Name:     name,
		Profile:  store.MemberProfile{UserID: user.ID, Interests: profile.Interests, Location: profile.Location},
		Invites:  invites,
	})
	switch {
	case errors.Is(err, store.ErrFamilyAlreadySetUp):
		respond.Error(w, http.StatusConflict, "Your family is already set up.", "")
		return
	case err != nil:
		h.serverError(w, r, "complete family setup", err)
		return
	}
	h.logger.InfoContext(r.Context(), "family set up", "family_id", user.FamilyID, "co_admin_invites", len(invites))
	h.writeMe(w, r, http.StatusOK, user)
}

// profileRequest is the body of PUT /api/me/profile. The fields are
// pointers so a missing one (nil) can be told apart from one sent blank
// (""): a PUT replaces both, and a forgotten field mustn't quietly erase
// what was saved.
type profileRequest struct {
	Interests *string `json:"interests"`
	Location  *string `json:"location"`
}

const profileExample = `Send JSON like {"interests": "space exploration, soccer", "location": "Denver, CO"}.`

// ProfileResponse is the response body of UpdateProfile.
type ProfileResponse struct {
	Profile MeProfile `json:"profile"`
}

// UpdateProfile handles PUT /api/me/profile: it replaces the logged-in
// user's interests and location (both must be sent; either may be blank)
// and responds with what was saved. The Articles and Local Events widgets read them on their
// next request. It must sit behind auth.RequireAuth.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	var req profileRequest
	if !decodeJSON(w, r, &req, profileExample) {
		return
	}
	if req.Interests == nil || req.Location == nil {
		respond.Error(w, http.StatusBadRequest, profileExample, `Send both fields; use "" to clear one.`)
		return
	}
	profile, msg, hint := cleanProfile(*req.Interests, *req.Location)
	if msg != "" {
		respond.Error(w, http.StatusBadRequest, msg, hint)
		return
	}

	err := h.store.UpsertMemberProfile(r.Context(), store.MemberProfile{
		UserID:    user.ID,
		Interests: profile.Interests,
		Location:  profile.Location,
	})
	if err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	h.logger.InfoContext(r.Context(), "profile updated", "user_id", user.ID)
	respond.JSON(w, http.StatusOK, ProfileResponse{Profile: profile})
}

// writeMe loads user's family and profile and writes them as Me.
func (h *Handler) writeMe(w http.ResponseWriter, r *http.Request, status int, user store.User) {
	ctx := r.Context()
	family, err := h.store.GetFamily(ctx, user.FamilyID)
	if err != nil {
		h.serverError(w, r, "load family", err)
		return
	}
	profile, err := h.store.GetMemberProfile(ctx, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) { // no profile yet reads as an empty one
		h.serverError(w, r, "load profile", err)
		return
	}

	respond.JSON(w, status, Me{
		User:             MeUser{ID: user.ID, Email: user.Email, IsAdmin: user.IsAdmin},
		Family:           MeFamily{ID: family.ID, Name: family.Name},
		NeedsFamilySetup: family.Name == "",
		Profile:          MeProfile{Interests: profile.Interests, Location: profile.Location},
	})
}

// serverError logs err and writes a generic 500. what says what we were
// doing, e.g. "load family".
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.ErrorContext(r.Context(), what+" failed", "err", err)
	respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
}

// decodeJSON reads exactly one JSON value from the request body into dst.
// On a bad body it writes a 400 whose message is example and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, example string) bool {
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
		respond.Error(w, http.StatusBadRequest, "That's more than this form can take.", "Shorten what you typed and try again.")
		return false
	case err != nil:
		respond.Error(w, http.StatusBadRequest, example, "")
		return false
	}
	return true
}
