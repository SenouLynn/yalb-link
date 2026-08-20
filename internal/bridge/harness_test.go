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

// fakeClock is shared by the bridge and test goroutines.
type fakeClock struct{ ms atomic.Int64 }

func newClock(startMs int64) *fakeClock {
	c := &fakeClock{}
	c.ms.Store(startMs)

	return c
}

func (c *fakeClock) Now() int64              { return c.ms.Load() }
func (c *fakeClock) Advance(d time.Duration) { c.ms.Add(int64(d / time.Millisecond)) }

// captureSink records events and can fail on demand.
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

// await polls until one captured event satisfies pred.
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

// harness runs real MAVLink bytes through codec.Node and Bridge over net.Pipe.
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

// wait returns and caches Run's result.
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
		// The injected clock controls conclusions; this only reduces test latency.
		Sweep: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("creating bridge: %v", err)
	}

	h := &harness{
		t: t, node: node, conn: testSide, sink: sink, clock: clock,
		bridge: b, runErr: make(chan error, 1),
	}

	// Drain heartbeat writes so the unbuffered pipe cannot block shutdown.
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
		// Stop consumer, producer, then pipe.
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
