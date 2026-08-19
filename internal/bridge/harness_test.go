package bridge

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"go.uber.org/goleak"

	"yalb.gcs/internal/codec"
	"yalb.gcs/internal/vehicle"
)

func TestMain(m *testing.M) {
	// Same claim as the codec's: the node and the bridge each own goroutines,
	// so "none leaked" is the only assertion worth making about them, and it
	// is only true if every harness tears down in the right order.
	goleak.VerifyTestMain(m)
}

const fixtureDir = "../../contracts/mavlink"

type fixtureMeta struct {
	SysID  uint8 `json:"sys_id"`
	CompID uint8 `json:"comp_id"`
}

func loadFixture(t *testing.T, name string) ([]byte, fixtureMeta) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(fixtureDir, name+".bin"))
	if err != nil {
		t.Fatalf("reading fixture %s.bin: %v", name, err)
	}

	metaRaw, err := os.ReadFile(filepath.Join(fixtureDir, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture %s.json: %v", name, err)
	}

	var meta fixtureMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("parsing fixture %s.json: %v", name, err)
	}

	return raw, meta
}

// fakeClock is the injected millisecond clock.
//
// Atomic rather than mutex-guarded because it is read on the bridge goroutine
// and advanced from the test goroutine, and the race detector is on.
type fakeClock struct{ ms atomic.Int64 }

func newClock(startMs int64) *fakeClock {
	c := &fakeClock{}
	c.ms.Store(startMs)

	return c
}

func (c *fakeClock) Now() int64              { return c.ms.Load() }
func (c *fakeClock) Advance(d time.Duration) { c.ms.Add(int64(d / time.Millisecond)) }

// captureSink records everything the fold emitted, and can fail on demand.
type captureSink struct {
	err    error
	events []vehicle.Event
	mu     sync.Mutex
}

func (s *captureSink) Publish(_ context.Context, ev vehicle.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	s.events = append(s.events, ev)

	return nil
}

func (s *captureSink) snapshot() []vehicle.Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]vehicle.Event(nil), s.events...)
}

// await polls until pred is satisfied by some captured event, or fails.
//
// Polling rather than a channel handshake: the bridge publishes from its own
// goroutine on its own schedule, and a test that blocks on an exact event
// count has to know how many events a fixture produces — which couples the
// test to the fold's internals rather than to the observation it cares about.
func (s *captureSink) await(t *testing.T, what string, pred func(vehicle.Event) bool) vehicle.Event {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		for _, ev := range s.snapshot() {
			if pred(ev) {
				return ev
			}
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s; captured %d events", what, len(s.snapshot()))

	return vehicle.Event{}
}

// refute asserts pred is satisfied by nothing within a settling window.
func (s *captureSink) refute(t *testing.T, what string, pred func(vehicle.Event) bool) {
	t.Helper()

	time.Sleep(250 * time.Millisecond)

	for _, ev := range s.snapshot() {
		if pred(ev) {
			t.Fatalf("expected no %s, but one was published", what)
		}
	}
}

// harness runs a real codec.Node over an in-memory pipe with a Bridge
// consuming it.
//
// The bridge is fed real MAVLink bytes through the real decoder rather than
// hand-built gomavlib events. Hand-built events would skip framing, CRC_EXTRA
// validation and dialect population — and, more to the point here, would let
// the test invent a *gomavlib.Channel, whose label is the source attribution
// the bridge actually depends on.
// Field order is pointer-bearing first and sync.Once last, which is what
// govet's fieldalignment asks for.
type harness struct {
	conn     net.Conn
	runErrIn error
	t        *testing.T
	node     *codec.Node
	sink     *captureSink
	clock    *fakeClock
	bridge   *Bridge
	cancel   context.CancelFunc
	runErr   chan error
	runOnce  sync.Once
}

// wait returns Run's result, blocking for it once and caching it.
//
// Cached because both a test and the cleanup hook may want it, and a second
// receive on the channel would block forever.
func (h *harness) wait() error {
	h.runOnce.Do(func() { h.runErrIn = <-h.runErr })

	return h.runErrIn
}

const clockEpochMs int64 = 1_700_000_000_000

func newHarness(t *testing.T) *harness {
	t.Helper()

	return newHarnessWithSink(t, &captureSink{})
}

func newHarnessWithSink(t *testing.T, sink *captureSink) *harness {
	t.Helper()

	testSide, nodeSide := net.Pipe()

	node, err := codec.NewNode([]gomavlib.EndpointConf{
		gomavlib.EndpointCustomClient{
			Connect: func(context.Context) (net.Conn, error) { return nodeSide, nil },
			Label:   "test",
		},
	})
	if err != nil {
		t.Fatalf("creating node: %v", err)
	}

	clock := newClock(clockEpochMs)

	b, err := New(Config{
		Source: node,
		Sink:   sink,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    clock.Now,
		// Far shorter than SweepInterval: the sweep is driven by the injected
		// clock, so the ticker only decides how soon the test notices, not
		// what the fold concludes.
		Sweep: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("creating bridge: %v", err)
	}

	h := &harness{
		t: t, node: node, conn: testSide, sink: sink, clock: clock,
		bridge: b, runErr: make(chan error, 1),
	}

	// Drain the node's outbound bytes. Load-bearing, not hygiene: net.Pipe is
	// unbuffered and the node emits a GCS heartbeat every second, so with
	// nothing reading this side the node's writer wedges and Close deadlocks.
	drained := make(chan struct{})

	go func() {
		defer close(drained)

		buf := make([]byte, 4096)

		for {
			if _, err := testSide.Read(buf); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	go func() { h.runErr <- b.Run(ctx) }()

	t.Cleanup(func() {
		// Order matters and mirrors the shutdown sequence in cmd/gcs: cancel
		// first so Run stops consuming, then close the node so its pump can
		// return, then close the pipe so the drain goroutine returns.
		cancel()
		_ = h.wait()
		_ = node.Close()
		_ = testSide.Close()
		<-drained
	})

	return h
}

// feed writes raw frame bytes into the node.
func (h *harness) feed(raw []byte) {
	h.t.Helper()

	go func() {
		_ = h.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = h.conn.Write(raw)
	}()
}
