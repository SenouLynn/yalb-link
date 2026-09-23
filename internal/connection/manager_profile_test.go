package connection

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"yalb.gcs/internal/codec"
)

// stubStore is an in-memory Store with injectable errors, for exercising
// Manager's profile-driven paths independently of the filesystem.
type stubStore struct {
	mu        sync.Mutex
	profiles  []Profile
	loadErr   error
	saveErr   error
	intentErr error
	deleteErr error
}

func (s *stubStore) Load() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return append([]Profile(nil), s.profiles...), nil
}

func (s *stubStore) Save(p Profile) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return Profile{}, s.saveErr
	}
	if p.ID == "" {
		p.ID = newProfileID()
		p.Intent = IntentIdle
		s.profiles = append(s.profiles, p)
		return p, nil
	}
	for i, existing := range s.profiles {
		if existing.ID == p.ID {
			p.Intent = existing.Intent
			s.profiles[i] = p
			return p, nil
		}
	}
	return Profile{}, ErrProfileNotFound
}

func (s *stubStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i, p := range s.profiles {
		if p.ID == id {
			s.profiles = append(s.profiles[:i], s.profiles[i+1:]...)
			return nil
		}
	}
	return ErrProfileNotFound
}

func (s *stubStore) UpdateIntent(id, intent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.intentErr != nil {
		return s.intentErr
	}
	for i, p := range s.profiles {
		if p.ID == id {
			s.profiles[i].Intent = intent
			return nil
		}
	}
	return ErrProfileNotFound
}

func waitEntryState(t *testing.T, m *Manager, id, state string) Status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range m.Connections() {
			if s.ID == id && s.State == state {
				return s
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("id %s: wanted state %s; got %+v", id, state, m.Connections())
	return Status{}
}

func newFakeOpener(opens *int) Opener {
	return func(Device, Settings, string) (codec.FrameSource, error) {
		*opens++
		return &fakeSource{events: make(chan gomavlib.Event)}, nil
	}
}

func TestStartupResolvesConnectedMissingAmbiguousIdleAndReleased(t *testing.T) {
	connected := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "SN-A", VID: "1", PID: "1"}
	dup1 := Device{ID: "serial:b1", Kind: "serial", Path: "b1", SerialNumber: "SN-B", VID: "2", PID: "2"}
	dup2 := Device{ID: "serial:b2", Kind: "serial", Path: "b2", SerialNumber: "SN-B", VID: "2", PID: "2"}
	devices := []Device{connected, dup1, dup2}

	store := &stubStore{profiles: []Profile{
		{ID: "profile-connected", Name: "Bench", Device: connected, Settings: Settings{BaudRate: 57600}, Intent: IntentConnected},
		{ID: "profile-missing", Name: "Field", Device: Device{SerialNumber: "SN-GONE", VID: "9", PID: "9"}, Settings: Settings{BaudRate: 57600}, Intent: IntentConnected},
		{ID: "profile-ambiguous", Name: "Dup", Device: Device{SerialNumber: "SN-B", VID: "2", PID: "2"}, Settings: Settings{BaudRate: 57600}, Intent: IntentConnected},
		{ID: "profile-idle", Name: "Never connected", Device: Device{SerialNumber: "SN-IDLE", VID: "3", PID: "3"}, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle},
		{ID: "profile-released", Name: "Handed off", Device: Device{SerialNumber: "SN-REL", VID: "4", PID: "4"}, Settings: Settings{BaudRate: 57600}, Intent: IntentReleased},
	}}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return devices, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	waitEntryState(t, m, "profile-connected", Awaiting)
	waitEntryState(t, m, "profile-missing", DeviceMissing)
	waitEntryState(t, m, "profile-ambiguous", Ambiguous)
	waitEntryState(t, m, "profile-idle", Idle)
	waitEntryState(t, m, "profile-released", Released)
	if opens != 1 {
		t.Fatalf("opened %d times, want exactly the one resolvable profile", opens)
	}
}

func TestStartupProfileLoadErrorDisablesAutoConnectOnly(t *testing.T) {
	store := &stubStore{loadErr: errors.New("corrupt profiles file")}
	var opens int
	m := New(Config{Inventory: inventoryFunc(testInventory), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	if len(m.Connections()) != 0 {
		t.Fatalf("no automatic acquisition expected from an unreadable store: %+v", m.Connections())
	}
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{BaudRate: 57600}); err != nil {
		t.Fatal("manual, non-profile connect must still work:", err)
	}
	if _, err := m.Profiles(); err == nil {
		t.Fatal("Profiles() should surface the same unreadable-store error")
	}
}

func TestConnectProfileResolvesConnectsAndPersistsIntent(t *testing.T) {
	device := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "SN", VID: "1", PID: "1"}
	store := &stubStore{profiles: []Profile{{ID: "p1", Name: "Bench", Device: device, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle}}}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{device}, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	status, err := m.ConnectProfile(context.Background(), "p1", "", Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != Awaiting || status.Settings.BaudRate != 57600 {
		t.Fatalf("expected profile's own settings as the default: %+v", status)
	}
	profiles, _ := store.Load()
	if profiles[0].Intent != IntentConnected {
		t.Fatalf("intent not persisted: %+v", profiles[0])
	}

	status2, err := m.ConnectProfile(context.Background(), "p1", "", Settings{})
	if err != nil || status2.OpenedAtMs != status.OpenedAtMs {
		t.Fatalf("expected an idempotent reconnect: %+v %v", status2, err)
	}
	if opens != 1 {
		t.Fatalf("opened %d times, want 1", opens)
	}

	if err := m.Disconnect(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	profiles, _ = store.Load()
	if profiles[0].Intent != IntentReleased {
		t.Fatalf("explicit disconnect must persist RELEASED: %+v", profiles[0])
	}
	if got := m.Connections()[0].State; got != Released {
		t.Fatal(got)
	}
}

func TestConnectProfileAmbiguousRequiresExplicitDevice(t *testing.T) {
	d1 := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "SN", VID: "1", PID: "1"}
	d2 := Device{ID: "serial:b", Kind: "serial", Path: "b", SerialNumber: "SN", VID: "1", PID: "1"}
	store := &stubStore{profiles: []Profile{{ID: "p1", Name: "Dup", Device: Device{SerialNumber: "SN", VID: "1", PID: "1"}, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle}}}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{d1, d2}, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	if _, err := m.ConnectProfile(context.Background(), "p1", "", Settings{}); !errors.Is(err, ErrAmbiguous) {
		t.Fatal(err)
	}
	if opens != 0 {
		t.Fatal("ambiguous match must never auto-select and open a device")
	}
	status, err := m.ConnectProfile(context.Background(), "p1", d2.ID, Settings{})
	if err != nil || status.Device.ID != d2.ID {
		t.Fatalf("explicit selection should disambiguate: %+v %v", status, err)
	}
}

func TestConnectProfileMissingIdentityRequiresExplicitSelection(t *testing.T) {
	store := &stubStore{profiles: []Profile{{ID: "p1", Name: "Cheap adapter", Device: Device{}, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle}}}
	known := Device{ID: "serial:a", Kind: "serial", Path: "a"}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{known}, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	if _, err := m.ConnectProfile(context.Background(), "p1", "", Settings{}); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	if _, err := m.ConnectProfile(context.Background(), "p1", known.ID, Settings{}); err != nil {
		t.Fatal(err)
	}
}

func TestConnectProfilePersistenceFailureRejectsAcquisition(t *testing.T) {
	device := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "SN", VID: "1", PID: "1"}
	store := &stubStore{profiles: []Profile{{ID: "p1", Name: "Bench", Device: device, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle}}, intentErr: errors.New("disk full")}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{device}, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	if _, err := m.ConnectProfile(context.Background(), "p1", "", Settings{}); err == nil {
		t.Fatal("expected the injected persistence failure to reject the connect")
	}
	if opens != 0 {
		t.Fatalf("device was opened despite failed intent persistence: opens=%d", opens)
	}
}

func TestConnectProfileUnknownProfile(t *testing.T) {
	store := &stubStore{}
	m := New(Config{Inventory: inventoryFunc(testInventory), Profiles: store})
	defer m.Close()
	if _, err := m.ConnectProfile(context.Background(), "missing", "", Settings{}); !errors.Is(err, ErrProfileNotFound) {
		t.Fatal(err)
	}
	m2 := New(Config{Inventory: inventoryFunc(testInventory)})
	defer m2.Close()
	if _, err := m2.ConnectProfile(context.Background(), "p1", "", Settings{}); !errors.Is(err, ErrProfilesDisabled) {
		t.Fatal(err)
	}
}

func TestSaveProfileCapturesInventoryNotCallerIdentity(t *testing.T) {
	real := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "REAL-SN", VID: "1", PID: "1", Description: "Real radio"}
	store := &stubStore{}
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{real}, nil }), Profiles: store})
	defer m.Close()

	faked := Device{ID: "serial:a", SerialNumber: "FAKE-SN", VID: "9", PID: "9", Description: "Invented"}
	profile, err := m.SaveProfile(context.Background(), Profile{Name: "Bench", Device: faked, Settings: Settings{BaudRate: 57600}})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Device != real {
		t.Fatalf("caller-invented identity leaked through: %+v", profile.Device)
	}

	if _, err := m.SaveProfile(context.Background(), Profile{Name: "x", Device: Device{ID: "serial:missing"}, Settings: Settings{BaudRate: 57600}}); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	if _, err := m.SaveProfile(context.Background(), Profile{Name: "  ", Device: real, Settings: Settings{BaudRate: 57600}}); !errors.Is(err, ErrProfileInvalid) {
		t.Fatal(err)
	}
	if _, err := m.SaveProfile(context.Background(), Profile{Name: "x", Device: real, Settings: Settings{}}); !errors.Is(err, ErrSettings) {
		t.Fatal(err)
	}
}

func TestSaveProfileRejectsUDPDevice(t *testing.T) {
	store := &stubStore{}
	m := New(Config{Inventory: inventoryFunc(testInventory), Profiles: store})
	defer m.Close()
	source := &fakeSource{events: make(chan gomavlib.Event)}
	if err := m.AddUDP("127.0.0.1:0", source); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SaveProfile(context.Background(), Profile{Name: "x", Device: Device{ID: "udp:127.0.0.1:0"}, Settings: Settings{BaudRate: 57600}}); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
}

func TestDeleteProfileRemovesPlaceholderAndDetachesLiveEntry(t *testing.T) {
	device := Device{ID: "serial:a", Kind: "serial", Path: "a", SerialNumber: "SN", VID: "1", PID: "1"}
	store := &stubStore{profiles: []Profile{
		{ID: "idle-profile", Name: "Never connected", Device: Device{SerialNumber: "SN-IDLE", VID: "9", PID: "9"}, Settings: Settings{BaudRate: 57600}, Intent: IntentIdle},
	}}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{device}, nil }), Profiles: store, Open: newFakeOpener(&opens)})
	defer m.Close()

	waitEntryState(t, m, "idle-profile", Idle)
	if err := m.DeleteProfile("idle-profile"); err != nil {
		t.Fatal(err)
	}
	for _, s := range m.Connections() {
		if s.ID == "idle-profile" {
			t.Fatalf("deleted idle placeholder should be gone: %+v", s)
		}
	}

	live, err := m.profiles.Save(Profile{Name: "Live", Device: device, Settings: Settings{BaudRate: 57600}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ConnectProfile(context.Background(), live.ID, "", Settings{}); err != nil {
		t.Fatal(err)
	}
	waitEntryState(t, m, live.ID, Awaiting)
	if err := m.DeleteProfile(live.ID); err != nil {
		t.Fatal(err)
	}
	if got := m.Connections()[0].State; got != Awaiting {
		t.Fatalf("deleting a profile must not disturb its live connection: %s", got)
	}
	// The entry is now detached: disconnecting it must not fail trying to
	// persist intent against a profile that no longer exists.
	if err := m.Disconnect(context.Background(), live.ID); err != nil {
		t.Fatal(err)
	}
}
