package connection

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EnvProfilesPath overrides the saved-connection-profiles document location.
const EnvProfilesPath = "GCS_CONNECTION_PROFILES_PATH"

// profileSchemaVersion is bumped only alongside a migration. A document at
// any other version is treated as unreadable rather than guessed at, per
// T-047's "invalid or unreadable stored settings" requirement.
const profileSchemaVersion = 1

// ResolveProfilesPath honors EnvProfilesPath, falling back to a file under
// the OS per-user configuration directory.
func ResolveProfilesPath(value string, set bool) (string, error) {
	if set && value != "" {
		return value, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve default connection profiles path: %w", err)
	}
	return filepath.Join(dir, "yalb-gcs", "connection-profiles.json"), nil
}

type profileDocument struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// FileStore persists profiles as one versioned JSON document, replaced
// atomically (temp file, fsync, rename) so a reader never observes a
// partially written file. A single process-local mutex serializes every
// read-modify-write cycle; this assumes one GCS process owns the file,
// matching the single local observer scope (T-050). See ADR 0010.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore returns a Store backed by the document at path. The directory
// is created on first write; it need not exist yet.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

// Load reads every saved profile.
func (s *FileStore) Load() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *FileStore) load() ([]Profile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read connection profiles: %w", err)
	}
	var doc profileDocument
	if jsonErr := json.Unmarshal(data, &doc); jsonErr != nil {
		return nil, fmt.Errorf("parse connection profiles: %w", jsonErr)
	}
	if doc.Version != profileSchemaVersion {
		return nil, fmt.Errorf("connection profiles version %d is not supported (want %d)", doc.Version, profileSchemaVersion)
	}
	for _, p := range doc.Profiles {
		if p.ID == "" || p.Name == "" {
			return nil, fmt.Errorf("connection profile is missing its id or name")
		}
	}
	return doc.Profiles, nil
}

// Save inserts or updates a profile by ID.
func (s *FileStore) Save(p Profile) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.load()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UnixMilli()
	if p.ID == "" {
		p.ID = newProfileID()
		p.CreatedAtMs = now
		p.Intent = IntentIdle
		p.UpdatedAtMs = now
		profiles = append(profiles, p)
	} else {
		idx := -1
		for i, existing := range profiles {
			if existing.ID == p.ID {
				idx = i
				break
			}
		}
		if idx == -1 {
			return Profile{}, ErrProfileNotFound
		}
		p.CreatedAtMs = profiles[idx].CreatedAtMs
		p.Intent = profiles[idx].Intent
		p.UpdatedAtMs = now
		profiles[idx] = p
	}
	if err := s.persist(profiles); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Delete removes a saved profile by ID.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.load()
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range profiles {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrProfileNotFound
	}
	profiles = append(profiles[:idx], profiles[idx+1:]...)
	return s.persist(profiles)
}

// UpdateIntent persists connect/disconnect intent for one saved profile.
func (s *FileStore) UpdateIntent(id, intent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.load()
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range profiles {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrProfileNotFound
	}
	profiles[idx].Intent = intent
	profiles[idx].UpdatedAtMs = time.Now().UnixMilli()
	return s.persist(profiles)
}

// persist atomically replaces the document with profiles. A failure here
// leaves the previous good file untouched: the temp file is written and
// synced first, and only a successful rename ever touches the real path.
func (s *FileStore) persist(profiles []Profile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create connection profiles directory: %w", err)
	}
	if profiles == nil {
		profiles = []Profile{}
	}
	data, err := json.MarshalIndent(profileDocument{Version: profileSchemaVersion, Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode connection profiles: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".connection-profiles-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary connection profiles file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write connection profiles: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync connection profiles: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close connection profiles: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("set connection profiles permissions: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace connection profiles: %w", err)
	}
	committed = true
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

// newProfileID mirrors internal/command's epoch generation: a random id,
// with a time-derived fallback that keeps saving usable rather than failing
// outright on the practically-never case that crypto/rand errors.
func newProfileID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("profile-t%d", time.Now().UnixNano())
	}
	return "profile-" + hex.EncodeToString(b[:])
}
