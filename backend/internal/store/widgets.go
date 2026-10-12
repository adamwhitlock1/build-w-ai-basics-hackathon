package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CustomWidget is a widget a user described, as saved. RenderSpec is opaque
// JSON to the store; the widgets package owns its shape.
type CustomWidget struct {
	ID         int64
	UserID     int64
	Name       string
	Source     string
	Query      string
	Prompt     string // the sentence it was described with; "" for widgets made before prompts were kept
	RenderSpec json.RawMessage
	CreatedAt  time.Time
	// RefreshedAt is when the widget's items were last refreshed; zero if
	// they never have been, so they date from CreatedAt.
	RefreshedAt time.Time
}

const customWidgetColumns = `id, user_id, name, source, query, prompt, render_spec, created_at, refreshed_at`

func scanCustomWidget(row scanner) (CustomWidget, error) {
	var (
		w    CustomWidget
		spec string // TEXT column; database/sql can't scan it into json.RawMessage directly
		// sql.NullTime scans a NULL refreshed_at as Valid == false instead
		// of failing.
		refreshed sql.NullTime
	)
	err := row.Scan(&w.ID, &w.UserID, &w.Name, &w.Source, &w.Query, &w.Prompt, &spec, &w.CreatedAt, &refreshed)
	if errors.Is(err, sql.ErrNoRows) {
		return CustomWidget{}, ErrNotFound
	}
	if err != nil {
		return CustomWidget{}, err
	}
	w.RenderSpec = json.RawMessage(spec)
	if refreshed.Valid {
		w.RefreshedAt = refreshed.Time
	}
	return w, nil
}

// CreateCustomWidget saves w for w.UserID and returns it with ID and
// CreatedAt filled in by the database (any values passed for them are
// ignored).
func (s *Store) CreateCustomWidget(ctx context.Context, w CustomWidget) (CustomWidget, error) {
	saved, err := scanCustomWidget(s.db.QueryRowContext(ctx,
		`INSERT INTO custom_widgets (user_id, name, source, query, prompt, render_spec) VALUES (?, ?, ?, ?, ?, ?)
		 RETURNING `+customWidgetColumns,
		// Passing the spec as a string stores it as TEXT rather than a BLOB.
		w.UserID, w.Name, w.Source, w.Query, w.Prompt, string(w.RenderSpec)))
	if err != nil {
		return CustomWidget{}, fmt.Errorf("create custom widget: %w", err)
	}
	return saved, nil
}

// GetCustomWidget returns widget widgetID if it belongs to userID. Another
// user's widget is reported as ErrNotFound, exactly like a missing one, so
// callers can't learn which ids exist.
func (s *Store) GetCustomWidget(ctx context.Context, userID, widgetID int64) (CustomWidget, error) {
	w, err := scanCustomWidget(s.db.QueryRowContext(ctx,
		`SELECT `+customWidgetColumns+` FROM custom_widgets WHERE id = ? AND user_id = ?`, widgetID, userID))
	if err != nil {
		return CustomWidget{}, fmt.Errorf("get custom widget %d: %w", widgetID, err)
	}
	return w, nil
}

// UpdateCustomWidgetItems replaces the render spec of widget widgetID, if it
// belongs to userID, and records refreshedAt as when it was refreshed. It
// returns the widget as saved. Like GetCustomWidget, it reports another
// user's widget as ErrNotFound and leaves it alone.
func (s *Store) UpdateCustomWidgetItems(ctx context.Context, userID, widgetID int64, renderSpec json.RawMessage, refreshedAt time.Time) (CustomWidget, error) {
	w, err := scanCustomWidget(s.db.QueryRowContext(ctx,
		`UPDATE custom_widgets SET render_spec = ?, refreshed_at = ? WHERE id = ? AND user_id = ?
		 RETURNING `+customWidgetColumns,
		string(renderSpec), formatTime(refreshedAt), widgetID, userID))
	if err != nil {
		return CustomWidget{}, fmt.Errorf("update custom widget %d: %w", widgetID, err)
	}
	return w, nil
}

// DeleteCustomWidget deletes widget widgetID if it belongs to userID. Like
// GetCustomWidget, it reports another user's widget as ErrNotFound, exactly
// like a missing one, and leaves it alone.
func (s *Store) DeleteCustomWidget(ctx context.Context, userID, widgetID int64) error {
	// The user_id check is part of the DELETE itself, so there is no gap
	// between checking who owns the widget and deleting it.
	res, err := s.db.ExecContext(ctx, `DELETE FROM custom_widgets WHERE id = ? AND user_id = ?`, widgetID, userID)
	if err != nil {
		return fmt.Errorf("delete custom widget %d: %w", widgetID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete custom widget %d: %w", widgetID, err)
	}
	if n == 0 {
		return fmt.Errorf("delete custom widget %d: %w", widgetID, ErrNotFound)
	}
	return nil
}

// ListCustomWidgets returns userID's widgets, oldest first.
func (s *Store) ListCustomWidgets(ctx context.Context, userID int64) ([]CustomWidget, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+customWidgetColumns+` FROM custom_widgets WHERE user_id = ? ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list custom widgets: %w", err)
	}
	defer rows.Close()

	var widgets []CustomWidget
	for rows.Next() {
		w, err := scanCustomWidget(rows)
		if err != nil {
			return nil, fmt.Errorf("list custom widgets: %w", err)
		}
		widgets = append(widgets, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list custom widgets: %w", err)
	}
	return widgets, nil
}
