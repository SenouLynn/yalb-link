package recording

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDeleteRecordingRemovesEventsAndMetadata(t *testing.T) {
	store := newTestStore(t, nil)
	started, err := store.StartRecording(context.Background(), "doomed")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range deterministicEvents(time.Now()) {
		if err := (&Recorder{Store: store}).Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRecording(context.Background(), started.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetRecording(context.Background(), started.ID); !errors.Is(err, ErrRecordingNotFound) {
		t.Fatalf("GetRecording() error = %v", err)
	}
	events, err := store.Replay(context.Background(), started.ID)
	if err != nil || len(events) != 0 {
		t.Fatalf("Replay() = %d events, %v", len(events), err)
	}
}

func TestDeleteRecordingRefusesActiveAndUnknown(t *testing.T) {
	store := newTestStore(t, nil)
	started, err := store.StartRecording(context.Background(), "active")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRecording(context.Background(), started.ID); !errors.Is(err, ErrRecordingInUse) {
		t.Fatalf("active delete error = %v", err)
	}
	if err := store.DeleteRecording(context.Background(), started.ID+1); !errors.Is(err, ErrRecordingNotFound) {
		t.Fatalf("unknown delete error = %v", err)
	}
}

func TestDeleteRecordingUsesMultipleChunks(t *testing.T) {
	store := newTestStore(t, nil)
	result, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
		VALUES ('large', 1, 2, 'stopped')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for seq := 1; seq <= deleteChunkSize+1; seq++ {
		if _, err := tx.Exec(`INSERT INTO recording_events(recording_id, seq, kind, occurred_at, payload)
			VALUES (?, ?, 'fleet', 1, x'00')`, id, seq); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRecording(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM recording_events WHERE recording_id = ?", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("event count = %d", count)
	}
}

func TestLiveBytesFallsAfterDelete(t *testing.T) {
	store := newTestStore(t, nil)
	result, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
		VALUES ('large', 1, 2, 'stopped')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	if _, err := store.db.Exec(`INSERT INTO recording_events(recording_id, seq, kind, occurred_at, payload)
		VALUES (?, 1, 'fleet', 1, zeroblob(1048576))`, id); err != nil {
		t.Fatal(err)
	}
	before, err := store.liveBytes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRecording(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	after, err := store.liveBytes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after >= before {
		t.Fatalf("live bytes after delete = %d, before = %d", after, before)
	}
}

func TestSweepDeletesOldestUntilCountBound(t *testing.T) {
	store := newTestStore(t, func(cfg *Config) {
		cfg.MaxTotalBytes = -1
		cfg.MaxTotalAge = -1
		cfg.MaxRecordings = -1
	})
	for id := 1; id <= 3; id++ {
		if _, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
			VALUES (?, ?, ?, 'stopped')`, fmt.Sprint(id), id, id); err != nil {
			t.Fatal(err)
		}
	}
	store.maxRecordings = 2
	deleted, err := store.sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d", deleted)
	}
	listed, err := store.ListRecordings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Name != "2" || listed[1].Name != "3" {
		t.Fatalf("remaining = %+v", listed)
	}
}

func TestSweepAppliesAgeBoundIndependently(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	store := newTestStore(t, func(cfg *Config) {
		cfg.Now = func() time.Time { return now }
		cfg.MaxTotalBytes = -1
		cfg.MaxTotalAge = -1
		cfg.MaxRecordings = -1
	})
	for _, started := range []time.Time{now.Add(-2 * time.Hour), now.Add(-30 * time.Minute)} {
		if _, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
			VALUES ('age', ?, ?, 'stopped')`, started.UnixMilli(), now.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	store.maxTotalAge = time.Hour
	deleted, err := store.sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d", deleted)
	}
}

func TestSweepAppliesLiveSizeBoundIndependently(t *testing.T) {
	store := newTestStore(t, func(cfg *Config) {
		cfg.MaxTotalBytes = -1
		cfg.MaxTotalAge = -1
		cfg.MaxRecordings = -1
	})
	result, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
		VALUES ('large', 1, 2, 'stopped')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	if _, err := store.db.Exec(`INSERT INTO recording_events(recording_id, seq, kind, occurred_at, payload)
		VALUES (?, 1, 'fleet', 1, zeroblob(1048576))`, id); err != nil {
		t.Fatal(err)
	}
	before, err := store.liveBytes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store.maxTotalBytes = before - 1
	deleted, err := store.sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d", deleted)
	}
}

func TestSweepFiresOnInjectedTimer(t *testing.T) {
	sweepElapsed := make(chan time.Time, 1)
	never := make(chan time.Time)
	store := newTestStore(t, func(cfg *Config) {
		cfg.MaxTotalBytes = -1
		cfg.MaxTotalAge = -1
		cfg.MaxRecordings = -1
		cfg.SweepInterval = time.Minute
		cfg.After = func(duration time.Duration) <-chan time.Time {
			if duration == time.Minute {
				return sweepElapsed
			}
			return never
		}
	})
	for id := 1; id <= 2; id++ {
		if _, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
			VALUES (?, ?, ?, 'stopped')`, fmt.Sprint(id), id, id); err != nil {
			t.Fatal(err)
		}
	}
	store.maxRecordings = 1
	sweepElapsed <- time.Now()
	waitFor(t, func() bool {
		listed, err := store.ListRecordings(context.Background())
		return err == nil && len(listed) == 1 && listed[0].Name == "2"
	})
}

func TestOpenSweepsExistingDatabase(t *testing.T) {
	path := t.TempDir() + "/retention.db"
	store, err := Open(Config{Path: path, MaxTotalBytes: -1, MaxTotalAge: -1, MaxRecordings: -1})
	if err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 2; id++ {
		if _, err := store.db.Exec(`INSERT INTO recordings(name, started_at, stopped_at, status)
			VALUES (?, ?, ?, 'stopped')`, fmt.Sprint(id), id, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(Config{Path: path, MaxTotalBytes: -1, MaxTotalAge: -1, MaxRecordings: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	listed, err := reopened.ListRecordings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != "2" {
		t.Fatalf("remaining = %+v", listed)
	}
}
