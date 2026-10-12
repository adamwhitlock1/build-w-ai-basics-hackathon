package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CuratedResult is a member's latest Articles or Local Events widget, as
// remembered. Payload is opaque JSON to the store; the widgets package owns
// its shape. Inputs records what it was picked for (the member's interests
// and location), so the widgets package can tell when it no longer fits.
type CuratedResult struct {
	UserID    int64
	Kind      string // "articles" or "events"
	Inputs    string
	Payload   json.RawMessage
	UpdatedAt time.Time
}

// GetCuratedResult returns userID's remembered result of kind, or
// ErrNotFound if there isn't one.
func (s *Store) GetCuratedResult(ctx context.Context, userID int64, kind string) (CuratedResult, error) {
	r := CuratedResult{UserID: userID, Kind: kind}
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT inputs, payload, updated_at FROM curated_widget_results WHERE user_id = ? AND kind = ?`,
		userID, kind).Scan(&r.Inputs, &payload, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return CuratedResult{}, fmt.Errorf("get %s result of user %d: %w", kind, userID, err)
	}
	r.Payload = json.RawMessage(payload)
	return r, nil
}

// PutCuratedResult remembers r as r.UserID's result of r.Kind, replacing
// any earlier one.
func (s *Store) PutCuratedResult(ctx context.Context, r CuratedResult) error {
	// "ON CONFLICT ... DO UPDATE" is SQLite's upsert: insert the row, or, if
	// one with the same primary key exists, update it instead.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO curated_widget_results (user_id, kind, inputs, payload, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (user_id, kind) DO UPDATE SET
		   inputs = excluded.inputs, payload = excluded.payload, updated_at = excluded.updated_at`,
		r.UserID, r.Kind, r.Inputs, string(r.Payload), formatTime(r.UpdatedAt))
	if err != nil {
		return fmt.Errorf("put %s result of user %d: %w", r.Kind, r.UserID, err)
	}
	return nil
}
