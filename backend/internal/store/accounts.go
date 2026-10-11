package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is one Porchlight account. Every user belongs to exactly one family.
type User struct {
	ID           int64
	Email        string
	PasswordHash []byte // bcrypt; never sent to clients or logged
	FamilyID     int64
	IsAdmin      bool
	CreatedAt    time.Time
}

// Family is a family unit. A family with an empty Name hasn't finished the
// create-family intro yet.
type Family struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// MemberProfile is what a member wants on their page: freeform interests
// and a freeform location.
type MemberProfile struct {
	UserID    int64
	Interests string
	Location  string
}

// Invite is an invitation for an email address to join a family. The Token
// is the secret part of the invite link.
type Invite struct {
	ID        int64
	FamilyID  int64
	Email     string
	Token     string
	IsAdmin   bool
	ExpiresAt time.Time
}

// FamilySetup is everything the create-family intro saves at once.
type FamilySetup struct {
	FamilyID int64
	Name     string
	Profile  MemberProfile // the admin filling in the intro
	Invites  []Invite      // co-admin invites; FamilyID is filled in for you
}

const userColumns = `id, email, password_hash, family_id, is_admin, created_at`

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FamilyID, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateFamilyWithAdmin signs up a new user: in one transaction it creates
// an unnamed family, the user as that family's admin, and an empty profile.
// It returns ErrEmailTaken if the email already has an account, in which
// case nothing is created.
func (s *Store) CreateFamilyWithAdmin(ctx context.Context, email string, passwordHash []byte) (User, error) {
	var user User
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		// users.family_id is NOT NULL, so the family has to exist first.
		var familyID int64
		err := tx.QueryRowContext(ctx, `INSERT INTO families (name) VALUES ('') RETURNING id`).Scan(&familyID)
		if err != nil {
			return fmt.Errorf("insert family: %w", err)
		}

		user, err = scanUser(tx.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, family_id, is_admin) VALUES (?, ?, ?, 1)
			 RETURNING `+userColumns,
			email, passwordHash, familyID))
		switch {
		case isUniqueViolation(err):
			return ErrEmailTaken // the deferred rollback also removes the family
		case err != nil:
			return fmt.Errorf("insert user: %w", err)
		}

		return upsertMemberProfile(ctx, tx, MemberProfile{UserID: user.ID})
	})
	if err != nil {
		return User{}, fmt.Errorf("create family with admin: %w", err)
	}
	return user, nil
}

// GetUser returns the user with the given id, or ErrNotFound.
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if err != nil {
		return User{}, fmt.Errorf("get user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByEmail returns the user with the given (already lowercased)
// email, or ErrNotFound.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ?`, email))
	if err != nil {
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetFamily returns the family with the given id, or ErrNotFound.
func (s *Store) GetFamily(ctx context.Context, id int64) (Family, error) {
	var f Family
	err := s.db.QueryRowContext(ctx, `SELECT id, name, created_at FROM families WHERE id = ?`, id).
		Scan(&f.ID, &f.Name, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return Family{}, fmt.Errorf("get family %d: %w", id, err)
	}
	return f, nil
}

// CompleteFamilySetup saves the create-family intro in one transaction: it
// names the family, saves the admin's profile, and records the co-admin
// invites. It returns ErrFamilyAlreadySetUp if the family already has a
// name, and ErrNotFound if there is no such family.
func (s *Store) CompleteFamilySetup(ctx context.Context, setup FamilySetup) error {
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		// "AND name = ''" makes checking and naming one atomic step, so two
		// racing requests can't both complete the setup.
		res, err := tx.ExecContext(ctx, `UPDATE families SET name = ? WHERE id = ? AND name = ''`,
			setup.Name, setup.FamilyID)
		if err != nil {
			return fmt.Errorf("name family: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return fmt.Errorf("name family: %w", err)
		} else if n == 0 {
			return familyMissingOrSetUp(ctx, tx, setup.FamilyID)
		}

		if err := upsertMemberProfile(ctx, tx, setup.Profile); err != nil {
			return err
		}
		for _, inv := range setup.Invites {
			inv.FamilyID = setup.FamilyID
			if _, err := createInvite(ctx, tx, inv); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("complete family setup: %w", err)
	}
	return nil
}

// familyMissingOrSetUp explains why naming a family changed no rows.
func familyMissingOrSetUp(ctx context.Context, q dbtx, familyID int64) error {
	var exists bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM families WHERE id = ?)`, familyID).Scan(&exists)
	switch {
	case err != nil:
		return fmt.Errorf("check family: %w", err)
	case !exists:
		return ErrNotFound
	default:
		return ErrFamilyAlreadySetUp
	}
}

// GetMemberProfile returns the profile for userID, or ErrNotFound.
func (s *Store) GetMemberProfile(ctx context.Context, userID int64) (MemberProfile, error) {
	p := MemberProfile{UserID: userID}
	err := s.db.QueryRowContext(ctx, `SELECT interests, location FROM member_profiles WHERE user_id = ?`, userID).
		Scan(&p.Interests, &p.Location)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return MemberProfile{}, fmt.Errorf("get profile for user %d: %w", userID, err)
	}
	return p, nil
}

// UpsertMemberProfile creates or replaces the profile for p.UserID.
func (s *Store) UpsertMemberProfile(ctx context.Context, p MemberProfile) error {
	return upsertMemberProfile(ctx, s.db, p)
}

func upsertMemberProfile(ctx context.Context, q dbtx, p MemberProfile) error {
	// ON CONFLICT turns the INSERT into an UPDATE when the row exists;
	// "excluded" is the row we tried to insert.
	_, err := q.ExecContext(ctx, `
		INSERT INTO member_profiles (user_id, interests, location) VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			interests  = excluded.interests,
			location   = excluded.location,
			updated_at = CURRENT_TIMESTAMP`,
		p.UserID, p.Interests, p.Location)
	if err != nil {
		return fmt.Errorf("upsert profile for user %d: %w", p.UserID, err)
	}
	return nil
}

// CreateInvite records a pending invite and returns it with its ID set.
func (s *Store) CreateInvite(ctx context.Context, inv Invite) (Invite, error) {
	return createInvite(ctx, s.db, inv)
}

func createInvite(ctx context.Context, q dbtx, inv Invite) (Invite, error) {
	err := q.QueryRowContext(ctx,
		`INSERT INTO invites (family_id, email, token, is_admin, expires_at) VALUES (?, ?, ?, ?, ?)
		 RETURNING id`,
		inv.FamilyID, inv.Email, inv.Token, inv.IsAdmin, formatTime(inv.ExpiresAt)).Scan(&inv.ID)
	if err != nil {
		return Invite{}, fmt.Errorf("insert invite: %w", err)
	}
	return inv, nil
}

// ListPendingInvites returns the family's invites that haven't been
// accepted yet (including expired ones), oldest first.
func (s *Store) ListPendingInvites(ctx context.Context, familyID int64) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, family_id, email, token, is_admin, expires_at
		FROM invites
		WHERE family_id = ? AND accepted_at IS NULL
		ORDER BY id`, familyID)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.ID, &inv.FamilyID, &inv.Email, &inv.Token, &inv.IsAdmin, &inv.ExpiresAt); err != nil {
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
