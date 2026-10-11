package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
)

// migrate applies every .sql file in fsys that the database hasn't seen,
// in filename order. Applied files are recorded in schema_migrations by
// name (without ".sql"), so each one runs exactly once per database.
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(fsys, "*.sql") // sorted by name
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	for _, name := range names {
		if err := applyMigration(ctx, db, fsys, name); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration runs one migration file and records it, in a single
// transaction: either the whole file applies and is recorded, or nothing
// changes.
func applyMigration(ctx context.Context, db *sql.DB, fsys fs.FS, name string) error {
	version := strings.TrimSuffix(name, ".sql")
	return withTx(ctx, db, func(tx *sql.Tx) error {
		var applied bool
		err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)`, version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			return nil
		}

		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		// One Exec runs every statement in the file.
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		return nil
	})
}
