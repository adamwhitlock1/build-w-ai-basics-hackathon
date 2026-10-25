package auth

// Most requests are about the person making them: GET /api/me/homepage is
// the logged-in user's own page. A parent can also look at someone else in
// their family, through the /api/members/{id}/... routes. Rather than a
// second copy of each handler for that, the handlers that serve both ask
// for the request's *subject*: the member the request is about.
//
//   - On /api/me/... routes nothing sets a subject, so the subject is the
//     logged-in user, and those routes work exactly as they always have.
//   - On /api/members/{id}/... routes, RequireMember looks up member {id}
//     (after RequireAuth and RequireAdmin have let the request in) and
//     makes them the subject.
//
// The logged-in user, from UserFrom, is still whoever is asking: the
// *viewer*. A handler reads whose data to use from SubjectFrom, and logs
// both ids, so the log shows when a parent looked at a kid's page.

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"kinlight/internal/respond"
	"kinlight/internal/store"
)

// subjectKey is the context key for the request's subject. Like userKey,
// it's unexported so only this package can set the value.
type subjectKey struct{}

// WithSubject returns a copy of ctx whose subject is member. RequireMember
// uses it; tests use it to fake a parent looking at a member's page.
func WithSubject(ctx context.Context, member store.User) context.Context {
	return context.WithValue(ctx, subjectKey{}, member)
}

// SubjectFrom returns the member the request is about: the family member
// RequireMember stored in ctx, or else the logged-in user. ok is false if
// there's neither.
//
// Falling back to the logged-in user is what lets one handler serve both
// /api/me/... and /api/members/{id}/..., and it fails safe: a handler that
// was mounted without RequireMember acts on the caller's own data, never
// on somebody else's.
func SubjectFrom(ctx context.Context) (member store.User, ok bool) {
	if member, ok = ctx.Value(subjectKey{}).(store.User); ok {
		return member, true
	}
	return UserFrom(ctx)
}

// RequireMember makes the family member named by the path's {id} the
// request's subject (read it with SubjectFrom). {id} may be the logged-in
// user's own id. Any id that isn't someone in the logged-in user's family
// (someone in another family, nobody at all, or not even a number) gets the
// same 404, so member ids can't be probed from outside the family.
//
// It decides only *which* member, not whether the caller may see them: it
// must sit inside RequireAuth, and inside RequireAdmin (or
// RequireAdminWithMessage) wherever only parents may look.
func (a *Authenticator) RequireMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		viewer, ok := UserFrom(ctx)
		if !ok {
			writeLoginRequired(w)
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeMemberNotFound(w)
			return
		}

		// The cases are checked in order, so the family comparison only
		// runs once the member has loaded.
		member, err := a.users.GetUser(ctx, id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeMemberNotFound(w)
			return
		case err != nil:
			a.logger.ErrorContext(ctx, "load family member", "member_id", id, "viewer_id", viewer.ID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Check the backend console for details.")
			return
		case member.FamilyID != viewer.FamilyID:
			writeMemberNotFound(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithSubject(ctx, member)))
	})
}

func writeMemberNotFound(w http.ResponseWriter) {
	respond.Error(w, http.StatusNotFound, "That family member doesn't exist.", "")
}
