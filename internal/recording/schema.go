package recording

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed migrations/0001_init.sql
var migration1 string

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("recording: reading schema version: %w", err)
	}

	if version > 1 {
		return fmt.Errorf("recording: database schema version %d is newer than supported version 1", version)
	}
	if version == 1 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recording: beginning migration: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit or the returned migration error is authoritative.

	if _, err := tx.ExecContext(ctx, migration1); err != nil {
		return fmt.Errorf("recording: applying migration 1: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		return fmt.Errorf("recording: setting schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recording: committing migration 1: %w", err)
	}

	return nil
}
