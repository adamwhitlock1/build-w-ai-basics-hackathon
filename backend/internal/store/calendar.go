package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetCalendarLink returns userID's calendar link, or "" if they haven't
// connected one (or have no profile yet). The link is a secret: callers
// must not log it or send it to a browser.
func (s *Store) GetCalendarLink(ctx context.Context, userID int64) (string, error) {
	var link string
	err := s.db.QueryRowContext(ctx, `SELECT calendar_url FROM member_profiles WHERE user_id = ?`, userID).Scan(&link)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get calendar link of user %d: %w", userID, err)
	}
	return link, nil
}

// SetCalendarLink saves link as userID's calendar link ("" removes it). It
// leaves their interests and location alone, just as UpsertMemberProfile
// leaves the link alone.
func (s *Store) SetCalendarLink(ctx context.Context, userID int64, link string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO member_profiles (user_id, calendar_url) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			calendar_url = excluded.calendar_url,
			updated_at   = CURRENT_TIMESTAMP`,
		userID, link)
	if err != nil {
		// The error names the user, never the link.
		return fmt.Errorf("set calendar link of user %d: %w", userID, err)
	}
	return nil
}

// DeleteCuratedResult forgets userID's remembered result of kind, if any.
func (s *Store) DeleteCuratedResult(ctx context.Context, userID int64, kind string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM curated_widget_results WHERE user_id = ? AND kind = ?`, userID, kind)
	if err != nil {
		return fmt.Errorf("delete %s result of user %d: %w", kind, userID, err)
	}
	return nil
}
