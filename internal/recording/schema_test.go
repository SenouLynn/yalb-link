package recording

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationOneToTwoPreservesRowsAndWithoutRowID(t *testing.T) {
	db, err := sql.Open("sqlite", t.TempDir()+"/migration.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, migration1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO recordings(id,name,started_at,status) VALUES(1,'old',1,'stopped')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO recording_events VALUES(1,1,'fleet',1,x'01')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM recording_events WHERE recording_id=1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("preserved rows = %d, want 1", count)
	}
	var ddl string
	if err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE name='recording_events'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(ddl), "WITHOUT ROWID") {
		t.Fatalf("table lost WITHOUT ROWID: %s", ddl)
	}
}
