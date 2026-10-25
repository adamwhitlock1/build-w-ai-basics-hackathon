package hosteddemo

import (
	"context"
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"path/filepath"
	"time"
)

type LimitError struct{ Reset time.Time }

func (e *LimitError) Error() string {
	return "Demo allowance reached. Live AI is paused until " + e.Reset.Format(time.RFC3339) + ". Existing content is still available."
}

type Quota struct {
	db               *sql.DB
	Models, Searches int
	Now              func() time.Time
}
type Status struct {
	ModelsRemaining   int       `json:"modelsRemaining"`
	SearchesRemaining int       `json:"searchesRemaining"`
	Reset             time.Time `json:"reset"`
}

var denver, _ = time.LoadLocation("America/Denver")

func Day(now time.Time) (string, time.Time) {
	n := now.In(denver)
	return n.Format("2006-01-02"), time.Date(n.Year(), n.Month(), n.Day()+1, 0, 0, 0, 0, denver)
}
func OpenQuota(dir string, models, searches int) (*Quota, error) {
	if models < 1 || searches < 1 {
		return nil, errors.New("demo allowances must be positive")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "quota.sqlite"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS quota(day TEXT NOT NULL,kind TEXT NOT NULL,used INTEGER NOT NULL,PRIMARY KEY(day,kind))`); err != nil {
		db.Close()
		return nil, err
	}
	return &Quota{db: db, Models: models, Searches: searches, Now: time.Now}, nil
}
func (q *Quota) Close() error { return q.db.Close() }
func (q *Quota) Reserve(ctx context.Context, kind string) (err error) {
	defer func() {
		if err != nil {
			failOperation(ctx, err)
		}
	}()
	limit := q.Models
	if kind == "search" {
		limit = q.Searches
	} else if kind != "model" {
		return errors.New("unknown quota kind")
	}
	day, reset := Day(q.Now())
	r, err := q.db.ExecContext(ctx, `INSERT INTO quota(day,kind,used) VALUES(?,?,1) ON CONFLICT(day,kind) DO UPDATE SET used=used+1 WHERE used < ?`, day, kind, limit)
	if err != nil {
		return errors.New("Demo allowance unavailable. Live calls are paused; existing content is still available.")
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return &LimitError{reset}
	}
	return nil
}
func (q *Quota) Status(ctx context.Context) (Status, error) {
	day, reset := Day(q.Now())
	s := Status{q.Models, q.Searches, reset}
	rows, err := q.db.QueryContext(ctx, `SELECT kind,used FROM quota WHERE day=?`, day)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var used int
		if err := rows.Scan(&kind, &used); err != nil {
			return s, err
		}
		if kind == "model" {
			s.ModelsRemaining = max(0, q.Models-used)
		} else {
			s.SearchesRemaining = max(0, q.Searches-used)
		}
	}
	return s, rows.Err()
}
