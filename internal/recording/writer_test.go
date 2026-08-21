package recording

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRepeatedWriterFailuresBecomeTerminal(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0).UTC()
	store := newTestStore(t, func(cfg *Config) {
		cfg.Now = func() time.Time { return clock }
		cfg.BatchSize = 1
	})
	store.writeBatchFn = func(context.Context, []writeRecord) error { return errors.New("injected write failure") }
	recording, err := store.StartRecording(context.Background(), "failure")
	if err != nil {
		t.Fatal(err)
	}
	recorder := &Recorder{Store: store}
	events := deterministicEvents(clock)
	for i := 0; i < 5; i++ {
		_ = recorder.Publish(context.Background(), events[i%len(events)])
	}

	waitFor(t, func() bool { return store.Stats().WriteErrors == 5 })
	listed, err := store.ListRecordings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != recording.ID || listed[0].Status != "error" || listed[0].StopReason != "writer_error" {
		t.Fatalf("recording after failures = %+v", listed)
	}
	if err := recorder.Publish(context.Background(), events[0]); err != nil {
		t.Errorf("Publish() error = %v", err)
	}
}

func TestBlockedWriteTimesOutAndShutdownCompletes(t *testing.T) {
	store := newTestStore(t, func(cfg *Config) {
		cfg.BatchSize = 1
		cfg.OperationTimeout = 20 * time.Millisecond
		cfg.ShutdownTimeout = 500 * time.Millisecond
	})
	store.writeBatchFn = func(ctx context.Context, _ []writeRecord) error {
		<-ctx.Done()
		return ctx.Err()
	}
	started, err := store.StartRecording(context.Background(), "blocked writer")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Recorder{Store: store}).Publish(context.Background(), deterministicEvents(time.Now())[0]); err != nil {
		t.Fatal(err)
	}

	var listed []Recording
	waitFor(t, func() bool {
		var listErr error
		listed, listErr = store.ListRecordings(context.Background())
		return listErr == nil && len(listed) == 1 && listed[0].Status == "error"
	})
	if store.Stats().WriteErrors != 1 {
		t.Fatalf("write errors = %d, want 1", store.Stats().WriteErrors)
	}
	listed, err = store.ListRecordings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != started.ID || listed[0].Status != "error" || listed[0].StopReason != "writer_error" {
		t.Fatalf("recording after blocked write = %+v", listed)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := store.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("condition was not reached")
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}
