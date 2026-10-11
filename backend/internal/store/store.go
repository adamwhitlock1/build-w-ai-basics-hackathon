// Package store is Porchlight's SQLite access layer. It owns the schema
// (applied from the migrations package at startup) and exposes one typed
// method per query, so no SQL lives outside this package.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	// Importing modernc.org/sqlite also registers its pure-Go "sqlite"
	// driver with database/sql (in the package's init function).
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"porchlight/migrations"
)

// Errors returned by Store methods. Callers match them with errors.Is.
var (
	// ErrNotFound means the row doesn't exist (or isn't visible to the
	// caller, e.g. another user's widget).
	ErrNotFound = errors.New("not found")
	// ErrEmailTaken means a user with that email already exists.
	ErrEmailTaken = errors.New("email already has an account")
	// ErrFamilyAlreadySetUp means the family already has a name, so the
	// create-family intro has been completed before.
	ErrFamilyAlreadySetUp = errors.New("family is already set up")
)

// timeFormat matches SQLite's CURRENT_TIMESTAMP ("2026-10-09 18:30:00", UTC).
// Times we write ourselves use it too, so SQL can compare them as text.
const timeFormat = "2006-01-02 15:04:05"

// Store reads and writes Porchlight's data. It is safe for concurrent use:
// *sql.DB is a connection pool, not a single connection.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and brings
// its schema up to date.
func Open(ctx context.Context, path string) (*Store, error) {
	// The _pragma parameters run on every new connection in the pool:
	// enforce foreign keys, use write-ahead logging so reads don't block
	// writes, and wait up to 5s for a lock instead of failing at once.
	//
	// "file:" makes SQLite read the path as a URI, where '?' would start
	// the options, '#' would end the path, and '%' starts an escape, so
	// those three are escaped to keep any file name working.
	escaped := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
	dsn := "file:" + escaped + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	// sql.Open only checks its arguments; Ping actually opens the file.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := migrate(ctx, db, migrations.FS); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// DB returns the underlying connection pool, for libraries that need a
// *sql.DB of their own (the session store).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// dbtx is the set of methods *sql.DB and *sql.Tx have in common. Query
// helpers that accept a dbtx work both on their own and inside a
// transaction.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// scanner is satisfied by both *sql.Row and *sql.Rows, so one scan function
// can read a row from either.
type scanner interface {
	Scan(dest ...any) error
}

// withTx runs fn inside a transaction, committing if fn returns nil and
// rolling back otherwise.
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback after a successful Commit does nothing, so deferring it
	// covers every early return (and panics) without extra bookkeeping.
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// isUniqueViolation reports whether err is SQLite rejecting a duplicate
// value in a UNIQUE column.
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

// formatTime converts t to the text form SQLite uses for timestamps.
func formatTime(t time.Time) string { return t.UTC().Format(timeFormat) }
