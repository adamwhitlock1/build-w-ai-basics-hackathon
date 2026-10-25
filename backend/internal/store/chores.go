package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrChoreTitle = errors.New("Use a chore title between 1 and 120 characters.")
var ErrChoreLimit = errors.New("Each member can have up to 30 active chores.")

type Chore struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}
type ChoreProgress struct {
	UserID         int64  `json:"userId"`
	Email          string `json:"email"`
	CompletedCount int    `json:"completedCount"`
	TotalCount     int    `json:"totalCount"`
}

func (s *Store) ListChores(ctx context.Context, family, member int64, date string) ([]Chore, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.title,EXISTS(SELECT 1 FROM chore_completions d WHERE d.chore_id=c.id AND d.local_date=?) FROM chores c WHERE c.family_id=? AND c.assignee_user_id=? AND c.archived_at IS NULL ORDER BY c.created_at,c.id`, date, family, member)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Chore{}
	for rows.Next() {
		var c Chore
		if err := rows.Scan(&c.ID, &c.Title, &c.Completed); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// MutateChore serializes the scope check and write with BEGIN IMMEDIATE.
// dateAtWrite runs after the lock is held, so a waiting request cannot write yesterday.
func (s *Store) MutateChore(ctx context.Context, family, member, actor, id int64, operation, title string, completed bool, dateAtWrite func() (string, error)) error {
	title = strings.TrimSpace(title)
	if operation == "create" || operation == "rename" {
		if !utf8.ValidString(title) || strings.ContainsRune(title, '\x00') || utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 120 {
			return ErrChoreTitle
		}
	}
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=? AND family_id=?)`, member, family).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if operation != "create" {
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chores WHERE id=? AND family_id=? AND assignee_user_id=? AND archived_at IS NULL)`, id, family, member).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		switch operation {
		case "create":
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM chores WHERE family_id=? AND assignee_user_id=? AND archived_at IS NULL`, family, member).Scan(&count); err != nil {
				return err
			}
			if count >= 30 {
				return ErrChoreLimit
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO chores(family_id,assignee_user_id,title,created_by_user_id) VALUES(?,?,?,?)`, family, member, title, actor)
			return err
		case "rename":
			_, err := tx.ExecContext(ctx, `UPDATE chores SET title=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND family_id=? AND assignee_user_id=?`, title, id, family, member)
			return err
		case "remove":
			_, err := tx.ExecContext(ctx, `UPDATE chores SET archived_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=? AND family_id=? AND assignee_user_id=?`, id, family, member)
			return err
		case "completion":
			date, err := dateAtWrite()
			if err != nil {
				return err
			}
			if completed {
				_, err = tx.ExecContext(ctx, `INSERT INTO chore_completions(chore_id,local_date,completed_by_user_id) VALUES(?,?,?) ON CONFLICT(chore_id,local_date) DO UPDATE SET completed_by_user_id=excluded.completed_by_user_id,completed_at=CURRENT_TIMESTAMP`, id, date, actor)
			} else {
				_, err = tx.ExecContext(ctx, `DELETE FROM chore_completions WHERE chore_id=? AND local_date=?`, id, date)
			}
			return err
		default:
			return errors.New("unknown chore operation")
		}
	})
}
func (s *Store) ChoreSummary(ctx context.Context, family int64, date string) ([]ChoreProgress, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.email,COUNT(d.chore_id),COUNT(c.id) FROM users u LEFT JOIN chores c ON c.assignee_user_id=u.id AND c.family_id=u.family_id AND c.archived_at IS NULL LEFT JOIN chore_completions d ON d.chore_id=c.id AND d.local_date=? WHERE u.family_id=? GROUP BY u.id ORDER BY u.is_admin DESC,u.created_at,u.id`, date, family)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChoreProgress{}
	for rows.Next() {
		var p ChoreProgress
		if err := rows.Scan(&p.UserID, &p.Email, &p.CompletedCount, &p.TotalCount); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
