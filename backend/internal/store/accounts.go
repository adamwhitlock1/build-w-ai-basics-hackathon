package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is one Kinlight account. Every user belongs to exactly one family.
type User struct {
	ID           int64
	Email        string
	PasswordHash []byte // bcrypt; never sent to clients or logged
	FamilyID     int64
	IsAdmin      bool
	Theme        string // ThemeAuto, ThemeLight or ThemeDark
	CreatedAt    time.Time
}

// The themes a member can choose: follow the device, or always Daybreak
// (light) or Porch light (dark).
const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"
)

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

// FamilySetup is everything the create-family intro saves at once.
type FamilySetup struct {
	FamilyID int64
	Name     string
	Profile  MemberProfile // the admin filling in the intro
	Invites  []Invite      // co-admin invites; FamilyID is filled in for you
}

const userColumns = `id, email, password_hash, family_id, is_admin, theme, created_at`

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FamilyID, &u.IsAdmin, &u.Theme, &u.CreatedAt)
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

// ListFamilyMembers returns everyone in familyID: the admins first, then
// everyone else, each group in the order they joined.
func (s *Store) ListFamilyMembers(ctx context.Context, familyID int64) ([]User, error) {
	// is_admin is stored as 1 or 0, so DESC puts the admins first. id breaks
	// ties between people who joined in the same second (created_at only
	// counts whole seconds).
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE family_id = ? ORDER BY is_admin DESC, created_at, id`, familyID)
	if err != nil {
		return nil, fmt.Errorf("list family members: %w", err)
	}
	defer rows.Close()

	var members []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list family members: %w", err)
		}
		members = append(members, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list family members: %w", err)
	}
	return members, nil
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
// name, and ErrNotFound if there is no such family. If a co-admin email
// already has an account, it returns an *EmailTakenError naming it. After
// any error, nothing has changed.
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
			// The same rule as InviteToFamily: an email that already has an
			// account can never accept an invite (each account belongs to
			// one family), so refuse it rather than keep a link that can't
			// work. The deferred rollback undoes the naming above too.
			var taken bool
			err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = ?)`, inv.Email).Scan(&taken)
			if err != nil {
				return fmt.Errorf("look up co-admin email: %w", err)
			}
			if taken {
				return &EmailTakenError{Email: inv.Email}
			}
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

// SetTheme saves userID's theme, one of the Theme constants, and returns the
// updated user. The database refuses any other value. It returns
// ErrNotFound if there's no such user.
func (s *Store) SetTheme(ctx context.Context, userID int64, theme string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`UPDATE users SET theme = ? WHERE id = ? RETURNING `+userColumns, theme, userID))
	if errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	if err != nil {
		return User{}, fmt.Errorf("set theme of user %d: %w", userID, err)
	}
	return u, nil
}
