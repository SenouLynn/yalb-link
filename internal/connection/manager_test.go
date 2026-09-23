package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialect"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/frame"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
	"github.com/bluenviron/gomavlib/v3/pkg/streamwriter"
	"yalb.gcs/internal/codec"
)

type inventoryFunc func(context.Context) ([]Device, error)

func (f inventoryFunc) List(ctx context.Context) ([]Device, error) { return f(ctx) }

var testDevice = Device{ID: "serial:test", Kind: "serial", Path: "test"}

func testInventory(context.Context) ([]Device, error) { return []Device{testDevice}, nil }

type fakeSource struct {
	events chan gomavlib.Event
	once   sync.Once
}

func (f *fakeSource) Events() <-chan gomavlib.Event               { return f.events }
func (f *fakeSource) WriteTo(codec.LinkID, message.Message) error { return nil }
func (f *fakeSource) Close() error                                { f.once.Do(func() { close(f.events) }); return nil }
func TestAccessErrorsAndIdempotence(t *testing.T) {
	for _, tc := range []struct {
		err         error
		code, state string
	}{{syscall.EACCES, "permission_denied", AccessFailed}, {syscall.EBUSY, "busy", AccessFailed}, {syscall.ENOENT, "missing", DeviceMissing}, {errors.New("unknown"), "io_error", AccessFailed}} {
		t.Run(tc.code, func(t *testing.T) {
			m := New(Config{Inventory: inventoryFunc(testInventory), Open: func(Device, Settings, string) (codec.FrameSource, error) { return nil, tc.err }})
			defer m.Close()
			s, err := m.Connect(context.Background(), testDevice.ID, Settings{57600})
			if !errors.Is(err, tc.err) || s.State != tc.state || s.ErrorCode != tc.code || s.DetailedError != tc.err.Error() {
				t.Fatalf("%+v %v", s, err)
			}
		})
	}
	opens := 0
	m := New(Config{Inventory: inventoryFunc(testInventory), Open: func(Device, Settings, string) (codec.FrameSource, error) {
		opens++
		return &fakeSource{events: make(chan gomavlib.Event)}, nil
	}})
	defer m.Close()
	for range 2 {
		if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
			t.Fatal(err)
		}
	}
	if opens != 1 {
		t.Fatalf("opened %d times", opens)
	}
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{115200}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := m.Connect(context.Background(), "missing", Settings{57600}); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{}); !errors.Is(err, ErrSettings) {
		t.Fatal(err)
	}
	if err := m.Disconnect(context.Background(), testDevice.ID); err != nil {
		t.Fatal(err)
	}
	if got := m.Connections()[0].State; got != Released {
		t.Fatal(got)
	}
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{115200}); err != nil {
		t.Fatal(err)
	}
	if opens != 2 {
		t.Fatal(opens)
	}
}

func waitState(t *testing.T, m *Manager, state string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s := m.Connections(); len(s) > 0 && s[0].State == state {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("wanted %s; got %+v", state, m.Connections())
}

// Real gomavlib stream framing through net.Pipe isolates the manager's
// lifecycle from OS drivers. The PTY harness covers the native serial adapter.
func TestFramingNoHeartbeatSilenceReopenAndStalledShutdown(t *testing.T) {
	var clock atomic.Int64
	clock.Store(1000)
	var peers []net.Conn
	var labels []string
	m := New(Config{Inventory: inventoryFunc(testInventory), Now: func() time.Time { return time.UnixMilli(clock.Load()) }, Silence: time.Second,
		Open: func(_ Device, _ Settings, label string) (codec.FrameSource, error) {
			local, peer := net.Pipe()
			peers = append(peers, peer)
			labels = append(labels, label)
			first := true
			n, err := codec.NewNode([]gomavlib.EndpointConf{gomavlib.EndpointCustomClient{Label: label, Connect: func(ctx context.Context) (net.Conn, error) {
				if first {
					first = false
					return local, nil
				}
				<-ctx.Done()
				return nil, ctx.Err()
			}}})
			go func() {
				buf := make([]byte, 4096)
				for {
					if _, err := peer.Read(buf); err != nil {
						return
					}
				}
			}()
			return &serialSource{Node: n, conn: &serialConn{ReadWriteCloser: local}}, err
		}})
	defer m.Close()
	defer func() {
		for _, p := range peers {
			_ = p.Close()
		}
	}()
	drainStop := make(chan struct{})
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for {
			select {
			case <-drainStop:
				return
			case _, ok := <-m.Events():
				if !ok {
					return
				}
			}
		}
	}()
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
		t.Fatal(err)
	}
	// Valid ATTITUDE without HEARTBEAT is connection evidence, not fleet liveness.
	drw := &dialect.ReadWriter{Dialect: ardupilotmega.Dialect}
	if err := drw.Initialize(); err != nil {
		t.Fatal(err)
	}
	writer := frame.ReadWriter{ByteReadWriter: peers[0], DialectRW: drw}
	if err := writer.Initialize(); err != nil {
		t.Fatal(err)
	}
	sw := streamwriter.Writer{FrameWriter: writer.Writer, SystemID: 7, ComponentID: 1, Version: streamwriter.V2}
	if err := sw.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := sw.Write(&ardupilotmega.MessageAttitude{Roll: 1}); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, Reporting)
	states, unsub := m.Subscribe()
	defer unsub()
	s := <-states
	if len(s.VehicleKeys) != 1 || s.VehicleKeys[0].SystemID != 7 {
		t.Fatalf("%+v", s)
	}
	s.VehicleKeys[0].SystemID = 99
	if m.Connections()[0].VehicleKeys[0].SystemID != 7 {
		t.Fatal("snapshot aliased")
	}
	clock.Store(3001)
	m.sweep()
	waitState(t, m, Interrupted)
	sw.SystemID = 8
	if err := sw.Write(&ardupilotmega.MessageAttitude{}); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, Reporting)
	if err := m.Disconnect(context.Background(), testDevice.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.WriteTo(codec.LinkID(labels[0]), codec.EncodeHeartbeat()); !errors.Is(err, codec.ErrUnknownLink) {
		t.Fatal("closed link remains writable", err)
	}
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
		t.Fatal(err)
	}
	if labels[0] == labels[1] {
		t.Fatal("reused a stale route label")
	}
	close(drainStop)
	<-drainDone
	done := make(chan struct{})
	go func() { _ = m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked on stalled consumer")
	}
}

func TestInventoryRemovalAndEnumerationFailure(t *testing.T) {
	var missing atomic.Bool
	var failed atomic.Bool
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) {
		if failed.Load() {
			return nil, errors.New("enumeration failed")
		}
		if missing.Load() {
			return []Device{}, nil
		}
		return []Device{testDevice}, nil
	}),
		Open: func(Device, Settings, string) (codec.FrameSource, error) {
			return &fakeSource{events: make(chan gomavlib.Event)}, nil
		}})
	defer m.Close()
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
		t.Fatal(err)
	}
	failed.Store(true)
	m.sweep()
	waitState(t, m, Awaiting)
	failed.Store(false)
	missing.Store(true)
	m.sweep()
	waitState(t, m, DeviceLost)
	missing.Store(false)
	m.sweep()
	waitState(t, m, DeviceLost) // No automatic retry in T-046.
}

func TestCloseInterruptsDisconnectWithStalledConsumer(t *testing.T) {
	source := &fakeSource{events: make(chan gomavlib.Event, 1)}
	source.events <- &gomavlib.EventChannelOpen{Channel: &gomavlib.Channel{}}
	m := New(Config{Inventory: inventoryFunc(testInventory), Open: func(Device, Settings, string) (codec.FrameSource, error) { return source, nil }})
	defer m.Close()
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		observed := len(m.links) > 0
		m.mu.Unlock()
		if observed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("channel was not observed")
		}
		time.Sleep(time.Millisecond)
	}
	disconnected := make(chan struct{})
	go func() { _ = m.Disconnect(context.Background(), testDevice.ID); close(disconnected) }()
	closed := make(chan struct{})
	go func() { _ = m.Close(); close(closed) }()
	for _, done := range []chan struct{}{closed, disconnected} {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("shutdown blocked")
		}
	}
}

func TestFixedUDPInventoryReleaseAndReconnect(t *testing.T) {
	m := New(Config{Inventory: inventoryFunc(testInventory)})
	defer m.Close()
	source := &fakeSource{events: make(chan gomavlib.Event)}
	if err := m.AddUDP("127.0.0.1:0", source); err != nil {
		t.Fatal(err)
	}
	devices, err := m.Devices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[1].Kind != "udp" {
		t.Fatal(devices)
	}
	if err := m.Disconnect(context.Background(), "udp:127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	status, err := m.Connect(context.Background(), "udp:127.0.0.1:0", Settings{})
	if err != nil || status.State != Awaiting {
		t.Fatal(status, err)
	}
}

func TestDisconnectDoesNotDropPendingChannelRetirement(t *testing.T) {
	source := &fakeSource{events: make(chan gomavlib.Event, 2)}
	channel := &gomavlib.Channel{}
	source.events <- &gomavlib.EventChannelOpen{Channel: channel}
	m := New(Config{Inventory: inventoryFunc(testInventory), Open: func(Device, Settings, string) (codec.FrameSource, error) { return source, nil }})
	defer m.Close()
	if _, err := m.Connect(context.Background(), testDevice.ID, Settings{57600}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-m.Events():
	case <-time.After(3 * time.Second):
		t.Fatal("missing open")
	}
	source.events <- &gomavlib.EventChannelClose{Channel: channel, Error: errors.New("injected EOF")}
	waitState(t, m, TransportFailed) // Close is observed but its consumer send is stalled.
	disconnected := make(chan struct{})
	go func() { _ = m.Disconnect(context.Background(), testDevice.ID); close(disconnected) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		stopped := m.entries[testDevice.ID].session == nil
		m.mu.Unlock()
		if stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnect did not start")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case ev := <-m.Events():
		if _, ok := ev.(*gomavlib.EventChannelClose); !ok {
			t.Fatalf("%T", ev)
		}
	case <-disconnected:
		t.Fatal("disconnect dropped the bridge's close event")
	case <-time.After(3 * time.Second):
		t.Fatal("missing channel retirement")
	}
	select {
	case <-disconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect did not finish")
	}
}
