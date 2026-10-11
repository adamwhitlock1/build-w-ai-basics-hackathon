package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// GetLayout returns userID's saved homepage layout: Puck's data, as JSON.
// It returns ErrNotFound if they have never saved one. The store doesn't
// look inside the JSON; the layout package owns its shape.
func (s *Store) GetLayout(ctx context.Context, userID int64) (json.RawMessage, error) {
	var data string // TEXT column; database/sql can't scan it into json.RawMessage directly
	err := s.db.QueryRowContext(ctx, `SELECT puck_data FROM homepage_layouts WHERE user_id = ?`, userID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get layout for user %d: %w", userID, err)
	}
	return json.RawMessage(data), nil
}

// UpsertLayout creates or replaces userID's homepage layout. Callers check
// puckData first: the store saves whatever it is given.
func (s *Store) UpsertLayout(ctx context.Context, userID int64, puckData json.RawMessage) error {
	// Same ON CONFLICT pattern as upsertMemberProfile. Passing the JSON as a
	// string stores it as TEXT rather than a BLOB.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO homepage_layouts (user_id, puck_data) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			puck_data  = excluded.puck_data,
			updated_at = CURRENT_TIMESTAMP`,
		userID, string(puckData))
	if err != nil {
		return fmt.Errorf("upsert layout for user %d: %w", userID, err)
	}
	return nil
}
