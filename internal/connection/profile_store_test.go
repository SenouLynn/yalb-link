package connection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileStoreSaveReloadDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	store := NewFileStore(path)

	profiles, err := store.Load()
	if err != nil || profiles != nil {
		t.Fatalf("fresh store: %+v %v", profiles, err)
	}

	device := Device{ID: "serial:/dev/ttyUSB0", Kind: "serial", SerialNumber: "SN1", VID: "2341", PID: "0043"}
	saved, err := store.Save(Profile{Name: "Bench controller", Device: device, Settings: Settings{BaudRate: 57600}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.Intent != IntentIdle || saved.CreatedAtMs == 0 || saved.UpdatedAtMs == 0 {
		t.Fatalf("unexpected saved profile: %+v", saved)
	}

	reloaded, err := store.Load()
	if err != nil || len(reloaded) != 1 || reloaded[0] != saved {
		t.Fatalf("reload mismatch: %+v %v", reloaded, err)
	}

	// Reopening the same path in a fresh Store instance proves durability.
	fresh := NewFileStore(path)
	reloaded, err = fresh.Load()
	if err != nil || len(reloaded) != 1 || reloaded[0].Name != "Bench controller" {
		t.Fatalf("durable reload mismatch: %+v %v", reloaded, err)
	}

	updated, err := store.Save(Profile{ID: saved.ID, Name: "Bench controller v2", Device: device, Settings: Settings{BaudRate: 115200}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CreatedAtMs != saved.CreatedAtMs {
		t.Fatalf("update must preserve CreatedAtMs: %+v", updated)
	}
	if updated.Intent != IntentIdle {
		t.Fatalf("update must preserve Intent: %+v", updated)
	}

	if err := store.UpdateIntent(saved.ID, IntentConnected); err != nil {
		t.Fatal(err)
	}
	reloaded, _ = store.Load()
	if reloaded[0].Intent != IntentConnected {
		t.Fatalf("intent not persisted: %+v", reloaded)
	}
	if err := store.UpdateIntent("missing", IntentConnected); err != ErrProfileNotFound {
		t.Fatal(err)
	}

	if _, err := store.Save(Profile{ID: "missing", Name: "x", Device: device, Settings: Settings{BaudRate: 57600}}); err != ErrProfileNotFound {
		t.Fatal(err)
	}

	if err := store.Delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err = store.Load()
	if err != nil || len(reloaded) != 0 {
		t.Fatalf("after delete: %+v %v", reloaded, err)
	}
	if err := store.Delete(saved.ID); err != ErrProfileNotFound {
		t.Fatal(err)
	}
}

func TestFileStoreAtomicReplaceLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store := NewFileStore(path)
	device := Device{ID: "serial:test", Kind: "serial", SerialNumber: "SN", VID: "v", PID: "p"}
	for range 5 {
		if _, err := store.Save(Profile{Name: "profile", Device: device, Settings: Settings{BaudRate: 57600}}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "profiles.json" {
		t.Fatalf("stray files left behind: %+v", entries)
	}
}

func TestFileStoreInvalidVersionMismatchedAndTruncatedFiles(t *testing.T) {
	for name, contents := range map[string]string{
		"truncated":        `{"version":1,"profiles":[{"id":"p1","na`,
		"empty":            ``,
		"not_json":         `not json at all`,
		"version_mismatch": `{"version":99,"profiles":[]}`,
		"missing_id":       `{"version":1,"profiles":[{"name":"x"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewFileStore(path)
			if _, err := store.Load(); err == nil {
				t.Fatal("expected a load error")
			}
			// A corrupt file must never be silently repaired/replaced by a read.
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != contents {
				t.Fatalf("bad file was modified: %q", raw)
			}
		})
	}
}

func TestFileStoreInjectedWriteFailureDoesNotReplaceGoodData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store := NewFileStore(path)
	device := Device{ID: "serial:test", Kind: "serial", SerialNumber: "SN", VID: "v", PID: "p"}
	saved, err := store.Save(Profile{Name: "good", Device: device, Settings: Settings{BaudRate: 57600}})
	if err != nil {
		t.Fatal(err)
	}

	// Make the directory read-only so the next persist's CreateTemp fails.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if _, err := store.Save(Profile{ID: saved.ID, Name: "corrupted-by-failed-write", Device: device, Settings: Settings{BaudRate: 9600}}); err == nil {
		t.Fatal("expected the injected write failure to surface")
	}

	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Load()
	if err != nil || len(reloaded) != 1 || reloaded[0].Name != "good" {
		t.Fatalf("good data was replaced after a failed write: %+v %v", reloaded, err)
	}
}

func TestFileStoreConcurrentWritersNeverCorruptTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	device := Device{ID: "serial:test", Kind: "serial", SerialNumber: "SN", VID: "v", PID: "p"}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		// Two independent Store instances over the same path stand in for two
		// concurrent backend writers; FileStore only serializes its own
		// instance, so this exercises atomic replace's guarantee that the
		// file itself is never torn, independently of any in-process lock.
		store := NewFileStore(path)
		go func(i int) {
			defer wg.Done()
			_, _ = store.Save(Profile{Name: "writer", Device: device, Settings: Settings{BaudRate: 57600 + i}})
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc profileDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("file was corrupted by concurrent writers: %v\n%s", err, data)
	}
}

func TestResolveProfilesPath(t *testing.T) {
	if got, err := ResolveProfilesPath("/custom/profiles.json", true); err != nil || got != "/custom/profiles.json" {
		t.Fatalf("%q %v", got, err)
	}
	got, err := ResolveProfilesPath("", false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "connection-profiles.json" {
		t.Fatalf("unexpected default path: %q", got)
	}
}
