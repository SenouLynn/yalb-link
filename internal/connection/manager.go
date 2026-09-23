package connection

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
	"yalb.gcs/internal/codec"
)

// Opener acquires a transport synchronously and gives it a unique channel label.
type Opener func(Device, Settings, string) (codec.FrameSource, error)

// Config supplies native defaults and injectable transport/clock boundaries.
type Config struct {
	Inventory Inventory
	Open      Opener
	// OnStatus must not block or call back into Manager. Hub publication is bounded.
	OnStatus func(Status)
	Now      func() time.Time
	Silence  time.Duration
	// Profiles persists saved connection targets across restarts. Nil
	// disables profile storage entirely: Connect/Disconnect by device id
	// keep working, matching T-046 behavior, but no profile CRUD, startup
	// resolution or intent persistence occurs.
	Profiles Store
}
type session struct {
	source   codec.FrameSource
	stop     chan struct{}
	done     chan struct{}
	channels map[codec.LinkID]*gomavlib.Channel
}
type sighting struct {
	owner *entry
	link  codec.LinkID
	seen  int64
}

type entry struct {
	status  Status
	session *session
	// profileID is set when this entry's connection id is a saved profile's
	// id, so Connect/Disconnect can persist CONNECTED/RELEASED intent. Empty
	// for a connection opened directly by device id.
	profileID string
}

// Manager multiplexes dynamic transports into one authoritative bridge/fleet
// fold. A child ending never closes the shared Events channel.
type Manager struct {
	inventory  Inventory
	open       Opener
	onStatus   func(Status)
	now        func() time.Time
	silence    time.Duration
	profiles   Store
	mu         sync.Mutex
	ops        sync.Mutex
	sightings  map[VehicleKey]sighting
	entries    map[string]*entry
	links      map[codec.LinkID]*session
	subs       map[chan Status]struct{}
	events     chan gomavlib.Event
	closing    chan struct{}
	done       chan struct{}
	once       sync.Once
	wg         sync.WaitGroup
	generation uint64
	closed     bool
}

// New starts acquisition supervision; no port opens until Connect or AddUDP.
func New(cfg Config) *Manager {
	if cfg.Inventory == nil {
		cfg.Inventory = NativeInventory{}
	}
	if cfg.Open == nil {
		cfg.Open = OpenSerial
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Silence <= 0 {
		cfg.Silence = codec.HeartbeatTTL
	}
	m := &Manager{inventory: cfg.Inventory, open: cfg.Open, onStatus: cfg.OnStatus, now: cfg.Now, silence: cfg.Silence, profiles: cfg.Profiles,
		sightings: map[VehicleKey]sighting{}, entries: map[string]*entry{}, links: map[codec.LinkID]*session{}, subs: map[chan Status]struct{}{},
		events: make(chan gomavlib.Event), closing: make(chan struct{}), done: make(chan struct{})}
	m.resolveProfilesAtStartup()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-m.closing:
				return
			case <-tick.C:
				m.sweep()
			}
		}
	}()
	return m
}

func clone(s Status) Status {
	s.VehicleKeys = slices.Clone(s.VehicleKeys)
	return s
}

func (m *Manager) publish(e *entry) {
	s := clone(e.status)
	if m.onStatus != nil {
		m.onStatus(s)
	}
	for ch := range m.subs {
		select {
		case ch <- clone(s):
		default:
			delete(m.subs, ch)
			close(ch)
		}
	}
}

// Subscribe atomically registers and primes a bounded status subscription.
// A subscriber that falls behind is closed rather than blocking acquisition.
func (m *Manager) Subscribe() (<-chan Status, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan Status, len(m.entries)+64)
	for _, s := range m.snapshot() {
		ch <- s
	}
	if m.closed {
		close(ch)
	} else {
		m.subs[ch] = struct{}{}
	}
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.subs[ch]; ok {
			delete(m.subs, ch)
			close(ch)
		}
	}
}

func (m *Manager) snapshot() []Status {
	out := make([]Status, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, clone(e.status))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Connections returns independent snapshots sorted by connection ID.
func (m *Manager) Connections() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshot()
}

// Devices refreshes OS candidates and includes the fixed UDP target, if configured.
func (m *Manager) Devices(ctx context.Context) ([]Device, error) {
	devices, err := m.inventory.List(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.status.Device.Kind == "udp" {
			devices = append(devices, e.status.Device)
		}
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })
	return devices, nil
}

// Events is the shared receive stream; child disconnection does not close it.
func (m *Manager) Events() <-chan gomavlib.Event { return m.events }

// Connect opens an inventoried target, or returns the existing identical session.
// Changed settings require explicit disconnect; no implicit device handoff occurs.
func (m *Manager) Connect(ctx context.Context, id string, settings Settings) (Status, error) {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Status{}, ErrClosed
	}
	var device Device
	if e := m.entries[id]; e != nil && e.status.Device.Kind == "udp" {
		device = e.status.Device
		settings = Settings{}
	} else if settings.BaudRate <= 0 || settings.BaudRate > 4000000 {
		m.mu.Unlock()
		return Status{}, ErrSettings
	}
	if e := m.entries[id]; e != nil && e.session != nil && (e.status.State == Awaiting || e.status.State == Reporting || e.status.State == Interrupted) {
		s := clone(e.status)
		m.mu.Unlock()
		if s.Settings != settings {
			return s, ErrConflict
		}
		return s, nil
	}
	m.mu.Unlock()
	if device.ID == "" {
		devices, err := m.inventory.List(ctx)
		if err != nil {
			return Status{}, err
		}
		for _, d := range devices {
			if d.ID == id && d.Kind == "serial" {
				device = d
				break
			}
		}
	}

	if device.ID == "" {
		return Status{}, ErrMissing
	}
	return m.acquire(ctx, id, device, settings, "")
}

// acquire opens an already-identified device under a stable connection id,
// which is a Device.ID for a direct connect or a Profile.ID for a
// profile-driven one. Callers must hold m.ops for the duration of the call;
// it does not itself serialize against Close or a concurrent acquire/disconnect.
func (m *Manager) acquire(ctx context.Context, id string, device Device, settings Settings, profileID string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	e := m.entries[id]
	m.mu.Unlock()
	if e != nil {
		m.stopSession(e)
	}
	m.mu.Lock()
	if e == nil {
		e = &entry{profileID: profileID}
		m.entries[id] = e
	} else if profileID != "" {
		e.profileID = profileID
	}
	e.status = Status{ID: id, Device: device, Settings: settings, State: Opening, VehicleKeys: []VehicleKey{}}
	m.generation++
	label := fmt.Sprintf("serial:%d:%s", m.generation, device.Path)
	m.publish(e)
	m.mu.Unlock()
	var source codec.FrameSource
	var err error
	if device.Kind == "udp" {
		source, err = codec.NewNode([]gomavlib.EndpointConf{gomavlib.EndpointUDPServer{Address: device.Path}})
	} else {
		source, err = m.open(device, settings, label)
	}
	if err == nil && ctx.Err() != nil {
		_ = source.Close()
		err = ctx.Err()
	}
	m.mu.Lock()
	if err != nil {
		e.status.State = AccessFailed
		e.status.DetailedError = err.Error()
		e.status.ErrorCode = errorCode(err)
		if e.status.ErrorCode == "missing" {
			e.status.State = DeviceMissing
		}
		m.publish(e)
		s := clone(e.status)
		m.mu.Unlock()
		return s, err
	}
	e.status.State = Awaiting
	e.status.OpenedAtMs = m.now().UnixMilli()
	m.attach(e, source)
	m.publish(e)
	s := clone(e.status)
	m.mu.Unlock()
	return s, nil
}

// connectResolved runs acquire under its own ops/closed check, for callers
// (startup resolution) that are not already inside a Connect/ConnectProfile
// critical section.
func (m *Manager) connectResolved(ctx context.Context, id string, device Device, settings Settings, profileID string) (Status, error) {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Status{}, ErrClosed
	}
	m.mu.Unlock()
	return m.acquire(ctx, id, device, settings, profileID)
}

// ConnectProfile resolves a saved profile against the current inventory and
// opens it, persisting CONNECTED intent before the device is actually opened
// so a persistence failure never leaves acquisition running unrecorded.
// explicitDeviceID, when set, is used verbatim instead of identity matching
// (the operator's explicit selection when identity is absent or ambiguous);
// it is never written back into the saved profile's identity.
func (m *Manager) ConnectProfile(ctx context.Context, profileID, explicitDeviceID string, settings Settings) (Status, error) {
	if m.profiles == nil {
		return Status{}, ErrProfilesDisabled
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Status{}, ErrClosed
	}
	if e := m.entries[profileID]; e != nil && e.session != nil && (e.status.State == Awaiting || e.status.State == Reporting || e.status.State == Interrupted) {
		s := clone(e.status)
		m.mu.Unlock()
		if settings.BaudRate > 0 && s.Settings != settings {
			return s, ErrConflict
		}
		return s, nil
	}
	m.mu.Unlock()

	profiles, err := m.profiles.Load()
	if err != nil {
		return Status{}, err
	}
	idx := slices.IndexFunc(profiles, func(p Profile) bool { return p.ID == profileID })
	if idx == -1 {
		return Status{}, ErrProfileNotFound
	}
	profile := profiles[idx]
	effective := settings
	if effective.BaudRate <= 0 {
		effective = profile.Settings
	}
	if effective.BaudRate <= 0 || effective.BaudRate > 4000000 {
		return Status{}, ErrSettings
	}
	devices, err := m.inventory.List(ctx)
	if err != nil {
		return Status{}, err
	}
	var device Device
	if explicitDeviceID != "" {
		for _, d := range devices {
			if d.ID == explicitDeviceID && d.Kind == "serial" {
				device = d
				break
			}
		}
		if device.ID == "" {
			return Status{}, ErrMissing
		}
	} else {
		switch matches := matchIdentity(profile.Device, devices); len(matches) {
		case 0:
			return Status{}, ErrMissing
		case 1:
			device = matches[0]
		default:
			return Status{}, ErrAmbiguous
		}
	}
	if err := m.profiles.UpdateIntent(profileID, IntentConnected); err != nil {
		return Status{}, fmt.Errorf("persist connection intent: %w", err)
	}
	return m.acquire(ctx, profileID, device, effective, profileID)
}

// Profiles returns every saved connection profile.
func (m *Manager) Profiles() ([]Profile, error) {
	if m.profiles == nil {
		return nil, nil
	}
	return m.profiles.Load()
}

// SaveProfile inserts or updates a saved profile. Device identity/metadata is
// always captured fresh from the current inventory, never from the caller,
// per ADR 0006's "do not silently substitute another device."
func (m *Manager) SaveProfile(ctx context.Context, in Profile) (Profile, error) {
	if m.profiles == nil {
		return Profile{}, ErrProfilesDisabled
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Profile{}, ErrProfileInvalid
	}
	if in.Settings.BaudRate <= 0 || in.Settings.BaudRate > 4000000 {
		return Profile{}, ErrSettings
	}
	// The fixed UDP development endpoint is configured by environment, not
	// saved: identity matching and startup resolution only apply to serial.
	devices, err := m.Devices(ctx)
	if err != nil {
		return Profile{}, err
	}
	idx := slices.IndexFunc(devices, func(d Device) bool { return d.ID == in.Device.ID })
	if idx == -1 || devices[idx].Kind != "serial" {
		return Profile{}, ErrMissing
	}
	return m.profiles.Save(Profile{ID: in.ID, Name: name, Device: devices[idx], Settings: in.Settings})
}

// DeleteProfile removes a saved profile. An unconnected placeholder entry for
// it is removed from Connections(); an actively open connection keeps running
// but is detached from the deleted profile, so a later Disconnect no longer
// tries to persist intent against it.
func (m *Manager) DeleteProfile(id string) error {
	if m.profiles == nil {
		return ErrProfilesDisabled
	}
	if err := m.profiles.Delete(id); err != nil {
		return err
	}
	m.mu.Lock()
	if e, ok := m.entries[id]; ok {
		if e.session == nil {
			delete(m.entries, id)
		} else {
			e.profileID = ""
		}
	}
	m.mu.Unlock()
	return nil
}

// resolveProfilesAtStartup seeds Connections() with every saved profile and
// auto-connects the ones that were CONNECTED (not RELEASED) when the backend
// last stopped and whose identity resolves to exactly one current device,
// per the contract's startup policy (docs/tasks/connection-contract.md §9).
// An unreadable profile store or inventory failure disables only automatic
// acquisition; manual, non-profile Connect calls are unaffected.
func (m *Manager) resolveProfilesAtStartup() {
	if m.profiles == nil {
		return
	}
	profiles, err := m.profiles.Load()
	if err != nil || len(profiles) == 0 {
		return
	}
	devices, invErr := m.inventory.List(context.Background())
	type pending struct {
		id       string
		device   Device
		settings Settings
	}
	var toConnect []pending
	m.mu.Lock()
	for _, p := range profiles {
		if _, exists := m.entries[p.ID]; exists {
			continue
		}
		status := Status{ID: p.ID, Device: p.Device, Settings: p.Settings, VehicleKeys: []VehicleKey{}}
		switch {
		case p.Intent != IntentConnected:
			if p.Intent == IntentReleased {
				status.State = Released
			} else {
				status.State = Idle
			}
		case invErr != nil:
			status.State = DeviceMissing
		default:
			switch matches := matchIdentity(p.Device, devices); len(matches) {
			case 0:
				status.State = DeviceMissing
			case 1:
				status.State = Opening
				toConnect = append(toConnect, pending{p.ID, matches[0], p.Settings})
			default:
				status.State = Ambiguous
			}
		}
		m.entries[p.ID] = &entry{status: status, profileID: p.ID}
	}
	m.mu.Unlock()
	for _, c := range toConnect {
		m.wg.Add(1)
		go func(c pending) {
			defer m.wg.Done()
			_, _ = m.connectResolved(context.Background(), c.id, c.device, c.settings, c.id)
		}(c)
	}
}

// AddUDP admits the fixed development endpoint to the same lifecycle and fold.
func (m *Manager) AddUDP(bind string, source codec.FrameSource) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	if _, ok := m.entries["udp:"+bind]; ok {
		return ErrConflict
	}
	e := &entry{status: Status{ID: "udp:" + bind, Device: Device{ID: "udp:" + bind, Kind: "udp", Path: bind}, State: Awaiting, OpenedAtMs: m.now().UnixMilli(), VehicleKeys: []VehicleKey{}}}
	m.entries[e.status.ID] = e
	m.attach(e, source)
	m.publish(e)
	return nil
}

func (m *Manager) attach(e *entry, source codec.FrameSource) {
	s := &session{source: source, stop: make(chan struct{}), done: make(chan struct{}), channels: map[codec.LinkID]*gomavlib.Channel{}}
	e.session = s
	m.wg.Add(1)
	go m.pump(e, s)
}

// Disconnect releases one port and retires its channels in receive order.
func (m *Manager) Disconnect(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	e := m.entries[id]
	m.mu.Unlock()
	if e == nil {
		return nil
	}
	m.stopSession(e)
	m.mu.Lock()
	e.status.State = Released
	e.status.VehicleKeys = []VehicleKey{}
	e.status.DetailedError = ""
	e.status.ErrorCode = ""
	profileID := e.profileID
	m.publish(e)
	m.mu.Unlock()
	if profileID != "" && m.profiles != nil {
		// Explicit disconnect must still release the port even if this fails;
		// the caller learns the intent could not be durably remembered.
		if err := m.profiles.UpdateIntent(profileID, IntentReleased); err != nil && !errors.Is(err, ErrProfileNotFound) {
			return fmt.Errorf("persist disconnect intent: %w", err)
		}
	}
	return nil
}

func (m *Manager) stopSession(e *entry) {
	m.mu.Lock()
	s := e.session
	if s == nil {
		m.mu.Unlock()
		return
	}
	e.session = nil
	// Disable addressed writes before stopping producers. Generation-specific
	// labels prevent an old route from becoming valid again after reopen.
	for link, owner := range m.links {
		if owner == s {
			delete(m.links, link)
			m.forgetSightings(link)
		}
	}
	close(s.stop)
	m.mu.Unlock()
	_ = s.source.Close()
	<-s.done
}

// WriteTo resolves only an active inbound link, never a broadcast fallback.
func (m *Manager) WriteTo(link codec.LinkID, msg message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.links[link]
	if s == nil {
		return codec.ErrUnknownLink
	}
	return s.source.WriteTo(link, msg)
}

func (m *Manager) pump(e *entry, s *session) {
	defer m.wg.Done()
	defer close(s.done)
	defer func() {
		// Child Close can suppress gomavlib close events. Explicitly retire every
		// channel after its last forwarded frame so the bridge forgets its routes.
		m.mu.Lock()
		channels := make([]*gomavlib.Channel, 0, len(s.channels))
		for link, ch := range s.channels {
			m.forgetSightings(link)
			channels = append(channels, ch)
			if m.links[link] == s {
				delete(m.links, link)
			}
		}
		m.mu.Unlock()
		for _, ch := range channels {
			select {
			case m.events <- &gomavlib.EventChannelClose{Channel: ch}:
			case <-m.closing:
				return
			}
		}
	}()
	for {
		select {
		case <-m.closing:
			return
		case <-s.stop:
			return
		case ev, ok := <-s.source.Events():
			if !ok {
				m.failed(e, s, "transport event stream closed")
				return
			}
			if !m.observe(e, s, ev) {
				return
			}
			select {
			case m.events <- ev:
				if closed, ok := ev.(*gomavlib.EventChannelClose); ok {
					m.mu.Lock()
					delete(s.channels, codec.LinkID(closed.Channel.String()))
					m.mu.Unlock()
				}
			case <-s.stop:
				return
			case <-m.closing:
				return
			}
		}
	}
}

func (m *Manager) observe(e *entry, s *session, ev gomavlib.Event) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.session != s {
		return false
	}
	switch v := ev.(type) {
	case *gomavlib.EventChannelOpen:
		link := codec.LinkID(v.Channel.String())
		s.channels[link] = v.Channel
		m.links[link] = s
	case *gomavlib.EventChannelClose:
		link := codec.LinkID(v.Channel.String())
		m.forgetSightings(link)
		if m.links[link] == s {
			delete(m.links, link)
		}
		if e.status.Device.Kind == "serial" {
			e.status.State = TransportFailed
			e.status.VehicleKeys = []VehicleKey{}
			if v.Error != nil {
				e.status.DetailedError = v.Error.Error()
				e.status.ErrorCode = errorCode(v.Error)
			}
			m.publish(e)
		}
	case *gomavlib.EventFrame:
		if v.SystemID() == codec.GCSSystemID && v.ComponentID() == codec.GCSComponentID {
			return true
		}

		e.status.State = Reporting
		e.status.LastFrameAtMs = m.now().UnixMilli()
		key := VehicleKey{v.SystemID(), v.ComponentID()}
		if prev, ok := m.sightings[key]; ok && prev.owner != e {
			prev.owner.status.VehicleKeys = slices.DeleteFunc(prev.owner.status.VehicleKeys, func(k VehicleKey) bool { return k == key })
			m.publish(prev.owner)
		}
		m.sightings[key] = sighting{owner: e, link: codec.LinkID(v.Channel.String()), seen: e.status.LastFrameAtMs}
		if !slices.Contains(e.status.VehicleKeys, key) {
			e.status.VehicleKeys = append(e.status.VehicleKeys, key)
			sort.Slice(e.status.VehicleKeys, func(i, j int) bool {
				a, b := e.status.VehicleKeys[i], e.status.VehicleKeys[j]
				if a.SystemID != b.SystemID {
					return a.SystemID < b.SystemID
				}
				return a.ComponentID < b.ComponentID
			})
		}
		m.publish(e)
	}
	return true
}

func (m *Manager) failed(e *entry, s *session, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.session != s {
		return
	}
	e.status.State = TransportFailed
	e.status.DetailedError = detail
	e.status.VehicleKeys = []VehicleKey{}
	m.publish(e)
}

func (m *Manager) sweep() {
	m.ops.Lock()
	defer m.ops.Unlock()
	select {
	case <-m.closing:
		return
	default:
	}
	// Enumeration failure is not evidence that any device disappeared.
	devices, inventoryErr := m.inventory.List(context.Background())
	present := map[string]bool{}
	for _, d := range devices {
		present[d.ID] = true
	}
	m.mu.Lock()
	var missing []*entry
	for key, sight := range m.sightings {
		if m.now().UnixMilli()-sight.seen > codec.HeartbeatTTL.Milliseconds() {
			delete(m.sightings, key)
			sight.owner.status.VehicleKeys = slices.DeleteFunc(sight.owner.status.VehicleKeys, func(k VehicleKey) bool { return k == key })
			m.publish(sight.owner)
		}
	}
	for _, e := range m.entries {
		if e.session != nil && e.status.Device.Kind == "serial" && inventoryErr == nil && !present[e.status.Device.ID] {
			missing = append(missing, e)
		} else if e.status.State == Reporting && m.now().UnixMilli()-e.status.LastFrameAtMs > m.silence.Milliseconds() {
			e.status.State = Interrupted
			m.publish(e)
		}
	}
	m.mu.Unlock()
	for _, e := range missing {
		m.stopSession(e)
		m.mu.Lock()
		e.status.State = DeviceLost
		e.status.DetailedError = "device no longer present in OS inventory"
		e.status.ErrorCode = "missing"
		e.status.VehicleKeys = []VehicleKey{}
		m.publish(e)
		m.mu.Unlock()
	}
}

// Close cancels all producers and blocked consumers before joining workers.
func (m *Manager) Close() error {
	m.once.Do(func() {
		// Cancel sends before waiting for ops: Disconnect may be delivering its
		// final close event to a stalled bridge consumer.
		close(m.closing)
		m.ops.Lock()
		m.mu.Lock()
		m.closed = true
		entries := make([]*entry, 0, len(m.entries))
		for _, e := range m.entries {
			entries = append(entries, e)
		}
		m.mu.Unlock()
		for _, e := range entries {
			m.stopSession(e)
		}
		m.ops.Unlock()
		m.wg.Wait()
		m.mu.Lock()
		for ch := range m.subs {
			delete(m.subs, ch)
			close(ch)
		}
		m.mu.Unlock()
		close(m.events)
		close(m.done)
	})
	<-m.done
	return nil
}

// forgetSightings removes only attribution still owned by the closed link.
// m.mu is held; a vehicle already heard on another connection is preserved.
func (m *Manager) forgetSightings(link codec.LinkID) {
	for key, sight := range m.sightings {
		if sight.link == link {
			delete(m.sightings, key)
			sight.owner.status.VehicleKeys = slices.DeleteFunc(sight.owner.status.VehicleKeys, func(k VehicleKey) bool { return k == key })
			m.publish(sight.owner)
		}
	}
}
