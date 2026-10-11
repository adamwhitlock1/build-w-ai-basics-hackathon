package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Invite is an invitation for an email address to join a family. The Token
// is the secret part of the invite link.
type Invite struct {
	ID        int64
	FamilyID  int64
	Email     string
	Token     string
	IsAdmin   bool // joins as an admin (a parent) rather than a kid
	ExpiresAt time.Time
}

const inviteColumns = `id, family_id, email, token, is_admin, expires_at`

// inviteIsUsable is the SQL condition for an invite that can still be
// accepted: nobody revoked or accepted it, and it hasn't expired. Every
// query that needs it uses this one definition, so they can't drift apart.
//
// SQLite has no real date type. expires_at is text in the same form as
// CURRENT_TIMESTAMP ("2026-10-25 18:30:00", UTC; formatTime writes it that
// way), so comparing the two as text compares them as times.
const inviteIsUsable = `revoked_at IS NULL AND accepted_at IS NULL AND expires_at > CURRENT_TIMESTAMP`

func scanInvite(row scanner) (Invite, error) {
	var inv Invite
	err := row.Scan(&inv.ID, &inv.FamilyID, &inv.Email, &inv.Token, &inv.IsAdmin, &inv.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrNotFound
	}
	return inv, err
}

// CreateInvite records a pending invite and returns it as saved: with its
// ID set, and ExpiresAt as stored (UTC, whole seconds). It doesn't check
// who already has an account or an invite; InviteToFamily does.
func (s *Store) CreateInvite(ctx context.Context, inv Invite) (Invite, error) {
	return createInvite(ctx, s.db, inv)
}

func createInvite(ctx context.Context, q dbtx, inv Invite) (Invite, error) {
	// RETURNING every column gives back the row exactly as later reads will
	// see it, so a new invite and a re-read one look the same to clients.
	saved, err := scanInvite(q.QueryRowContext(ctx,
		`INSERT INTO invites (family_id, email, token, is_admin, expires_at) VALUES (?, ?, ?, ?, ?)
		 RETURNING `+inviteColumns,
		inv.FamilyID, inv.Email, inv.Token, inv.IsAdmin, formatTime(inv.ExpiresAt)))
	if err != nil {
		return Invite{}, fmt.Errorf("insert invite: %w", err)
	}
	return saved, nil
}

// InviteToFamily invites inv.Email to join inv.FamilyID, as an admin if
// inv.IsAdmin. It returns the new invite with created set to true.
//
// If that email already has a usable invite to this family, InviteToFamily
// returns that invite instead (created is false), so the same person never
// holds two links. Its role is switched to inv.IsAdmin if that changed;
// inv.Token and inv.ExpiresAt go unused.
//
// It returns ErrAlreadyMember if the email belongs to someone in this
// family, and ErrEmailTaken if it belongs to someone in another family.
func (s *Store) InviteToFamily(ctx context.Context, inv Invite) (saved Invite, created bool, err error) {
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		// Is the email someone's already? One account belongs to one family.
		var theirFamily int64
		err := tx.QueryRowContext(ctx, `SELECT family_id FROM users WHERE email = ?`, inv.Email).Scan(&theirFamily)
		switch {
		case err == nil && theirFamily == inv.FamilyID:
			return ErrAlreadyMember
		case err == nil:
			return ErrEmailTaken
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("look up email: %w", err)
		}

		// Reuse their pending invite, if they have one. (Normally there is at
		// most one; the newest wins if not.)
		saved, err = scanInvite(tx.QueryRowContext(ctx,
			`SELECT `+inviteColumns+` FROM invites
			 WHERE family_id = ? AND email = ? AND `+inviteIsUsable+`
			 ORDER BY id DESC LIMIT 1`,
			inv.FamilyID, inv.Email))
		switch {
		case errors.Is(err, ErrNotFound):
			saved, err = createInvite(ctx, tx, inv)
			created = err == nil
			return err
		case err != nil:
			return fmt.Errorf("find pending invite: %w", err)
		}

		if saved.IsAdmin != inv.IsAdmin {
			if _, err := tx.ExecContext(ctx, `UPDATE invites SET is_admin = ? WHERE id = ?`, inv.IsAdmin, saved.ID); err != nil {
				return fmt.Errorf("change invite role: %w", err)
			}
			saved.IsAdmin = inv.IsAdmin
		}
		return nil
	})
	if err != nil {
		return Invite{}, false, fmt.Errorf("invite to family: %w", err)
	}
	return saved, created, nil
}

// ListPendingInvites returns the family's invites that can still be
// accepted (not accepted yet, not expired), oldest first.
func (s *Store) ListPendingInvites(ctx context.Context, familyID int64) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+inviteColumns+` FROM invites WHERE family_id = ? AND `+inviteIsUsable+` ORDER BY id`, familyID)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, fmt.Errorf("list invites: %w", err)
		}
		invites = append(invites, inv)
	}
	// rows.Next returns false on error too; rows.Err tells the two apart.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	return invites, nil
}

// GetInviteByToken returns the invite whose link carries token, if it can
// still be accepted. It returns ErrNotFound for a token that matches no
// invite (a typo, or a revoked invite) and ErrInviteExpired for one that
// expired or was already accepted.
//
// Errors never include the token: it's a secret, and errors get logged.
func (s *Store) GetInviteByToken(ctx context.Context, token string) (Invite, error) {
	inv, err := scanInvite(s.db.QueryRowContext(ctx,
		`SELECT `+inviteColumns+` FROM invites WHERE token = ? AND `+inviteIsUsable, token))
	if errors.Is(err, ErrNotFound) {
		err = inviteMissingOrUsed(ctx, s.db, token)
	}
	if err != nil {
		return Invite{}, fmt.Errorf("get invite: %w", err)
	}
	return inv, nil
}

// AcceptInvite turns the invite whose link carries token into an account.
// In one transaction it marks the invite accepted, creates the user (with
// passwordHash) in the invite's family, as an admin if the invite says so,
// and gives them an empty profile. They have no saved layout, so their
// homepage starts from the default one.
//
// It returns ErrNotFound for an unknown (or revoked) token,
// ErrInviteExpired for one that expired or was already accepted, and
// ErrEmailTaken if the invited email has an account by now. After any
// error, nothing has changed.
func (s *Store) AcceptInvite(ctx context.Context, token string, passwordHash []byte) (User, error) {
	var user User
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		// Checking that the invite is usable and claiming it is one UPDATE,
		// so it's a single atomic step: if the same link is accepted twice
		// at once (a double-clicked button), only the first UPDATE matches
		// a row, and the second finds the invite already accepted.
		// RETURNING hands back the claimed row's columns, like a SELECT.
		var inv Invite
		err := tx.QueryRowContext(ctx,
			`UPDATE invites SET accepted_at = CURRENT_TIMESTAMP
			 WHERE token = ? AND `+inviteIsUsable+`
			 RETURNING family_id, email, is_admin`,
			token).Scan(&inv.FamilyID, &inv.Email, &inv.IsAdmin)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return inviteMissingOrUsed(ctx, tx, token)
		case err != nil:
			return fmt.Errorf("claim invite: %w", err)
		}

		user, err = scanUser(tx.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, family_id, is_admin) VALUES (?, ?, ?, ?)
			 RETURNING `+userColumns,
			inv.Email, passwordHash, inv.FamilyID, inv.IsAdmin))
		switch {
		case isUniqueViolation(err):
			// Someone signed up with this email after the invite was sent.
			// The deferred rollback also un-claims the invite.
			return ErrEmailTaken
		case err != nil:
			return fmt.Errorf("insert user: %w", err)
		}

		return upsertMemberProfile(ctx, tx, MemberProfile{UserID: user.ID})
	})
	if err != nil {
		return User{}, fmt.Errorf("accept invite: %w", err)
	}
	return user, nil
}

// inviteMissingOrUsed explains why no usable invite has token: there is no
// such invite at all, or it was revoked (ErrNotFound: a revoked link reads
// as one that never existed), or it expired or was already accepted
// (ErrInviteExpired).
func inviteMissingOrUsed(ctx context.Context, q dbtx, token string) error {
	var exists bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM invites WHERE token = ? AND revoked_at IS NULL)`, token).Scan(&exists)
	switch {
	case err != nil:
		return fmt.Errorf("check invite: %w", err)
	case !exists:
		return ErrNotFound
	default:
		return ErrInviteExpired
	}
}

// RevokeInvite revokes familyID's pending invite inviteID, so its link
// stops working. An invite from another family, one already accepted, or
// one already revoked is reported as ErrNotFound, exactly like a missing
// one, and left alone.
//
// The invite is marked revoked rather than deleted, so its id is never
// given to a later invite (see migrations/002_invite_revoked_at.sql): a
// repeated revoke for this id can't hit someone else's invite.
func (s *Store) RevokeInvite(ctx context.Context, familyID, inviteID int64) error {
	// The family, "not accepted" and "not revoked" checks are part of the
	// UPDATE itself, so nothing can change between checking and revoking.
	res, err := s.db.ExecContext(ctx,
		`UPDATE invites SET revoked_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND family_id = ? AND accepted_at IS NULL AND revoked_at IS NULL`, inviteID, familyID)
	if err != nil {
		return fmt.Errorf("revoke invite %d: %w", inviteID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke invite %d: %w", inviteID, err)
	}
	if n == 0 {
		return fmt.Errorf("revoke invite %d: %w", inviteID, ErrNotFound)
	}
	return nil
}
