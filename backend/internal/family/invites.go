package family

// The family endpoints: who's in your family, inviting new members and
// revoking invites (parents only), and the public invite link that turns an
// invite into an account.
//
// There is no email sending: an invite is a link (/invite/<token>) that a
// parent copies from the sidebar and shares however they like. The token is
// the only secret in it, so it's 32 random bytes.

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/respond"
	"kinlight/internal/store"
)

// FamilyResponse is the response body of GET /api/me/family.
type FamilyResponse struct {
	Family MeFamily `json:"family"`
	// Members is everyone in the family for an admin, admins first; anyone
	// else sees only themselves. Never null.
	Members []Member `json:"members"`
	// PendingInvites are the invites that can still be accepted, oldest
	// first. Only admins see them; for anyone else it's empty, never null.
	PendingInvites []PendingInvite `json:"pendingInvites"`
}

// Member is one person in the family.
type Member struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"isAdmin"`
	IsYou   bool   `json:"isYou"` // this is the logged-in user
}

// PendingInvite is an invite that hasn't been accepted yet.
type PendingInvite struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"isAdmin"` // they'll join as a parent, not a kid
	// URL is the invite link as a path, "/invite/<token>". The frontend
	// puts its own origin in front, since only the browser knows which
	// address the app is being served from.
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// InviteResponse is the response body of CreateInvite.
type InviteResponse struct {
	Invite PendingInvite `json:"invite"`
}

// InviteDetails is the response body of GET /api/invites/{token}: what the
// set-your-password page shows.
type InviteDetails struct {
	Email      string `json:"email"`
	FamilyName string `json:"familyName"`
	IsAdmin    bool   `json:"isAdmin"`
}

func toPendingInvite(inv store.Invite) PendingInvite {
	return PendingInvite{
		ID:        inv.ID,
		Email:     inv.Email,
		IsAdmin:   inv.IsAdmin,
		URL:       "/invite/" + inv.Token, // base64url tokens are safe in a path as-is
		ExpiresAt: inv.ExpiresAt,
	}
}

func toMember(u store.User, you int64) Member {
	return Member{ID: u.ID, Email: u.Email, IsAdmin: u.IsAdmin, IsYou: u.ID == you}
}

// Family handles GET /api/me/family: the logged-in user's family, its
// members, and its pending invites. Admins see everyone and every pending
// invite; anyone else sees only themselves (prd.md > Family Management). It
// must sit behind auth.RequireAuth.
func (h *Handler) Family(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	family, err := h.store.GetFamily(ctx, user.FamilyID)
	if err != nil {
		h.serverError(w, r, "load family", err)
		return
	}

	// Empty slices, not nil ones, so the JSON says [] rather than null.
	resp := FamilyResponse{
		Family:         MeFamily{ID: family.ID, Name: family.Name},
		Members:        []Member{},
		PendingInvites: []PendingInvite{},
	}
	if !user.IsAdmin {
		resp.Members = append(resp.Members, toMember(user, user.ID))
		respond.JSON(w, http.StatusOK, resp)
		return
	}

	members, err := h.store.ListFamilyMembers(ctx, user.FamilyID)
	if err != nil {
		h.serverError(w, r, "list family members", err)
		return
	}
	for _, m := range members {
		resp.Members = append(resp.Members, toMember(m, user.ID))
	}
	invites, err := h.store.ListPendingInvites(ctx, user.FamilyID)
	if err != nil {
		h.serverError(w, r, "list invites", err)
		return
	}
	for _, inv := range invites {
		resp.PendingInvites = append(resp.PendingInvites, toPendingInvite(inv))
	}
	respond.JSON(w, http.StatusOK, resp)
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // "parent" or "kid"
}

const inviteExample = `Send JSON like {"email": "kid@example.com", "role": "kid"}.`

// CreateInvite handles POST /api/families/{id}/invites: it invites an email
// to join the family as a parent (an admin) or a kid, and responds with the
// invite, whose link the parent copies and shares.
//
// Inviting an email that already has a pending invite gives back that same
// invite (200, with the new role if it changed) rather than a second link;
// a brand new invite is 201. An email that already has an account, in this
// family or another, gets a 409 instead, as does inviting anyone before the
// create-family intro is done.
//
// It must sit behind auth.RequireAuth and an admin check, and {id} must be
// the caller's own family.
func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := h.ownFamily(w, r)
	if !ok {
		return
	}
	// Invites wait for the intro: a kid who joined an unnamed family would
	// be sent to an intro only a parent can finish. (A family's name never
	// goes back to "", so checking it outside the invite's transaction is
	// safe.)
	family, err := h.store.GetFamily(r.Context(), user.FamilyID)
	if err != nil {
		h.serverError(w, r, "load family", err)
		return
	}
	if family.Name == "" {
		respond.Error(w, http.StatusConflict, "Finish setting up your family first.", "Name your family, then you can invite people.")
		return
	}

	var req inviteRequest
	if !decodeJSON(w, r, &req, inviteExample) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "That doesn't look like an email address.", "Check it for typos, like a missing @ or dot.")
		return
	}
	var isAdmin bool
	switch strings.ToLower(strings.TrimSpace(req.Role)) {
	case "parent":
		isAdmin = true
	case "kid":
		isAdmin = false
	default:
		respond.Error(w, http.StatusBadRequest, "Choose whether they're a parent or a kid.", `Send "role": "parent" or "kid".`)
		return
	}

	// The token is only used if this turns out to be a new invite.
	token, err := newInviteToken()
	if err != nil {
		h.serverError(w, r, "create invite token", err)
		return
	}
	ctx := r.Context()
	inv, created, err := h.store.InviteToFamily(ctx, store.Invite{
		FamilyID:  user.FamilyID,
		Email:     email,
		Token:     token,
		IsAdmin:   isAdmin,
		ExpiresAt: time.Now().Add(inviteLifetime),
	})
	switch {
	case errors.Is(err, store.ErrAlreadyMember):
		respond.Error(w, http.StatusConflict, "Already a member.", "They can log in with that email.")
		return
	case errors.Is(err, store.ErrEmailTaken):
		writeEmailTaken(w)
		return
	case err != nil:
		h.serverError(w, r, "create invite", err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
		h.logger.InfoContext(ctx, "invite created", "family_id", user.FamilyID, "invite_id", inv.ID, "is_admin", inv.IsAdmin)
	}
	respond.JSON(w, status, InviteResponse{Invite: toPendingInvite(inv)})
}

// RevokeInvite handles DELETE /api/families/{id}/invites/{inviteId}: it
// revokes a pending invite, so its link stops working, and responds 204 No
// Content. An invite that's already accepted or revoked, or belongs to
// another family, gets the same 404 as a missing one.
//
// It must sit behind auth.RequireAuth and an admin check, and {id} must be
// the caller's own family.
func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := h.ownFamily(w, r)
	if !ok {
		return
	}
	inviteID, err := strconv.ParseInt(r.PathValue("inviteId"), 10, 64)
	if err != nil {
		writeInviteIDNotFound(w)
		return
	}

	ctx := r.Context()
	err = h.store.RevokeInvite(ctx, user.FamilyID, inviteID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeInviteIDNotFound(w)
		return
	case err != nil:
		h.serverError(w, r, "revoke invite", err)
		return
	}
	h.logger.InfoContext(ctx, "invite revoked", "family_id", user.FamilyID, "invite_id", inviteID)
	w.WriteHeader(http.StatusNoContent)
}

// ShowInvite handles GET /api/invites/{token}, which is public: the
// set-your-password page loads it to say whose family the invite is for.
// An unknown or revoked token is a 404; an expired or already accepted one
// is a 410 Gone.
func (h *Handler) ShowInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inv, err := h.store.GetInviteByToken(ctx, r.PathValue("token"))
	if !h.inviteUsable(w, r, err) {
		return
	}
	family, err := h.store.GetFamily(ctx, inv.FamilyID)
	if err != nil {
		h.serverError(w, r, "load family", err)
		return
	}
	respond.JSON(w, http.StatusOK, InviteDetails{Email: inv.Email, FamilyName: family.Name, IsAdmin: inv.IsAdmin})
}

type acceptRequest struct {
	Password string `json:"password"`
}

// AcceptInvite handles POST /api/invites/{token}/accept, which is public:
// it creates the invited member's account with the password they chose,
// logs them in, and responds 201 with Me. Their homepage starts from the
// default layout.
//
// Anyone already logged in on this browser is logged out of their own
// account by this: the session now belongs to the new member.
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req acceptRequest
	if !decodeJSON(w, r, &req, `Send JSON like {"password": "..."}.`) {
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

	// The store re-checks the invite inside the transaction that creates
	// the account, so a link accepted twice at once makes one account.
	ctx := r.Context()
	user, err := h.store.AcceptInvite(ctx, r.PathValue("token"), hash)
	if errors.Is(err, store.ErrEmailTaken) {
		writeEmailTaken(w)
		return
	}
	if !h.inviteUsable(w, r, err) {
		return
	}

	// LogIn renews the session token, as it does at every login, then
	// points the session at the new member.
	if err := h.sessions.LogIn(ctx, user.ID); err != nil {
		h.logger.ErrorContext(ctx, "log in after accepting invite", "user_id", user.ID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Your account was created, but we couldn't log you in.", "Try logging in.")
		return
	}
	h.logger.InfoContext(ctx, "invite accepted", "user_id", user.ID, "family_id", user.FamilyID, "is_admin", user.IsAdmin)
	h.writeMe(w, r, http.StatusCreated, user)
}

// ownFamily checks that the {id} in the path is the logged-in user's own
// family and returns that user. Any other id, even one that isn't a number,
// gets the same 404 as a family that doesn't exist, so family ids can't be
// probed; it writes that response and returns false.
func (h *Handler) ownFamily(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return store.User{}, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id != user.FamilyID {
		respond.Error(w, http.StatusNotFound, "That family doesn't exist.", "")
		return store.User{}, false
	}
	return user, true
}

// inviteUsable answers an error from looking up or accepting an invite by
// its token. It reports true for a nil err; otherwise it writes the right
// response (404, 410, or 500) and reports false.
func (h *Handler) inviteUsable(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, store.ErrNotFound):
		respond.Error(w, http.StatusNotFound, "That invite link doesn't work.", "Ask whoever invited you for a fresh link.")
	case errors.Is(err, store.ErrInviteExpired):
		// 410 Gone: the invite existed, but can't be used any more.
		respond.Error(w, http.StatusGone, "That invite has expired or was already used.", "If you already set a password, log in instead.")
	default:
		h.serverError(w, r, "look up invite", err)
	}
	return false
}

func writeInviteIDNotFound(w http.ResponseWriter) {
	respond.Error(w, http.StatusNotFound, "That invite doesn't exist.", "It may have been accepted or revoked already.")
}

// writeEmailTaken answers an invite for an email that already has an
// account somewhere else.
func writeEmailTaken(w http.ResponseWriter) {
	respond.Error(w, http.StatusConflict, "That email already has a Kinlight account.", "Each account belongs to one family for now.")
}
