package recording

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newTestStore(t *testing.T, modify func(*Config)) *Store {
	t.Helper()
	cfg := Config{Path: t.TempDir() + "/recordings.db"}
	if modify != nil {
		modify(&cfg)
	}
	store, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return store
}
