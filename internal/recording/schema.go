package recording

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed migrations/0001_init.sql
var migration1 string

//go:embed migrations/0002_command_kind.sql
var migration2 string

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("recording: reading schema version: %w", err)
	}

	if version > 2 {
		return fmt.Errorf("recording: database schema version %d is newer than supported version 2", version)
	}
	if version == 2 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recording: beginning migration: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit or the returned migration error is authoritative.

	if version == 0 {
		if _, err := tx.ExecContext(ctx, migration1); err != nil {
			return fmt.Errorf("recording: applying migration 1: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, migration2); err != nil {
		return fmt.Errorf("recording: applying migration 2: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
		return fmt.Errorf("recording: setting schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recording: committing migrations: %w", err)
	}

	return nil
}
