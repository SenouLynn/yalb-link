package recording

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLifecycle(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0).UTC()
	store := newTestStore(t, func(cfg *Config) { cfg.Now = func() time.Time { return clock } })

	started, err := store.StartRecording(context.Background(), "flight one")
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	if started.Status != "active" || !started.StartedAt.Equal(clock) {
		t.Fatalf("started = %+v", started)
	}
	if _, err := store.StartRecording(context.Background(), "flight two"); !errors.Is(err, ErrRecordingActive) {
		t.Fatalf("second StartRecording() error = %v, want ErrRecordingActive", err)
	}

	clock = clock.Add(time.Minute)
	stopped, err := store.StopRecording(context.Background())
	if err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}
	if stopped.Status != "stopped" || stopped.StopReason != "requested" || stopped.StoppedAt == nil || !stopped.StoppedAt.Equal(clock) {
		t.Fatalf("stopped = %+v", stopped)
	}
	if _, err := store.StopRecording(context.Background()); !errors.Is(err, ErrNoRecording) {
		t.Fatalf("second StopRecording() error = %v, want ErrNoRecording", err)
	}
}

func TestLimitClosesAtCommittedBatchBoundary(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0).UTC()
	store := newTestStore(t, func(cfg *Config) {
		cfg.Now = func() time.Time { return clock }
		cfg.BatchSize = 100
		cfg.MaxEvents = 2
	})
	recording, err := store.StartRecording(context.Background(), "bounded")
	if err != nil {
		t.Fatal(err)
	}
	events := deterministicEvents(clock)
	recorder := &Recorder{Store: store}
	for _, event := range events[:2] {
		_ = recorder.Publish(context.Background(), event)
	}

	stopped, err := store.StopRecording(context.Background())
	if err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}
	if stopped.ID != recording.ID || stopped.Status != "limit_reached" || stopped.StopReason != "event_limit" {
		t.Fatalf("stopped = %+v", stopped)
	}
	if stopped.EventCount != 2 {
		t.Errorf("event count = %d, want 2", stopped.EventCount)
	}
}

func TestPublishNeverBlocksWhenQueueIsFull(t *testing.T) {
	store := &Store{items: make(chan writerItem, 1), log: discardLogger()}
	store.current = &activeRecording{id: 1}
	recorder := &Recorder{Store: store}
	events := deterministicEvents(time.Unix(1_700_000_000, 0))

	if err := recorder.Publish(context.Background(), events[0]); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- recorder.Publish(context.Background(), events[1]) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Publish() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Publish() blocked on a full queue")
	}
	if got := store.Stats().Dropped; got != 1 {
		t.Errorf("dropped = %d, want 1", got)
	}
	<-store.items
	if err := recorder.Publish(context.Background(), events[2]); err != nil {
		t.Fatal(err)
	}
	item := <-store.items
	if item.record.seq != 3 {
		t.Errorf("sequence after dropped event = %d, want 3", item.record.seq)
	}
}

func TestOpenMarksInterruptedRecordingAsError(t *testing.T) {
	path := t.TempDir() + "/interrupted.db"
	clock := time.Unix(1_700_000_000, 0).UTC()
	store, err := Open(Config{Path: path, Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(),
		"INSERT INTO recordings(name, started_at, status) VALUES ('interrupted', ?, 'active')", clock.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	clock = clock.Add(time.Minute)
	reopened, err := Open(Config{Path: path, Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	listed, err := reopened.ListRecordings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Status != "error" || listed[0].StopReason != "process_restart" {
		t.Fatalf("recovered recording = %+v", listed)
	}
}
