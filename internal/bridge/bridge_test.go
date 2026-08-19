package bridge

import (
	"errors"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

// fleetOfType matches a FleetEvent of a given type from a given system.
func fleetOfType(t gcsv1.FleetEventType, sysID uint32) func(vehicle.Event) bool {
	return func(ev vehicle.Event) bool {
		return ev.Fleet.GetType() == t && ev.Fleet.GetVehicleId().GetSystemId() == sysID
	}
}

// BRIDGE-DISCOVER: raw HEARTBEAT bytes on the wire become a VEHICLE_DISCOVERED
// event out of the sink, and a route the send path can resolve.
//
// This is the whole Tier 5 receive path asserted end to end without a socket:
// bytes → gomavlib framing → codec decode → fold → sink. It is the offline
// version of the tier's headline gate ("VEHICLE_DISCOVERED in the logs"), so a
// failure against live SITL after this passes is a transport problem and not a
// pipeline one.
func TestBridgeDiscoversVehicleFromHeartbeat(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "heartbeat_v2")
	h := newHarness(t)

	h.feed(raw)

	ev := h.sink.await(t, "VEHICLE_DISCOVERED",
		fleetOfType(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, uint32(meta.SysID)))

	if got := ev.Fleet.GetVehicleId().GetComponentId(); got != uint32(meta.CompID) {
		t.Errorf("compid = %d, want %d", got, meta.CompID)
	}

	// Identity has to arrive with the discovery, not after it. A discovery
	// event that cannot name the airframe forces every consumer to wait for a
	// second message before it can render anything.
	if ev.Fleet.GetHeartbeat() == nil {
		t.Fatal("VEHICLE_DISCOVERED carried no HeartbeatState; consumers have nothing to render")
	}

	key := routes.Key{SysID: meta.SysID, CompID: meta.CompID}
	if _, ok := h.bridge.Routes().Lookup(key, h.clock.Now()); !ok {
		t.Errorf("no route for %s after discovery; the vehicle is visible but unaddressable", key)
	}
}

// BRIDGE-TELEMETRY: a telemetry family folds and reaches the sink.
//
// ATTITUDE rather than HEARTBEAT because the two take different paths through
// the fold — one is a TelemetryEvent, the other is fleet identity — and this
// asserts the branch the fleet test does not reach.
func TestBridgeForwardsTelemetry(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "attitude_v2")
	h := newHarness(t)

	h.feed(raw)

	ev := h.sink.await(t, "ATTITUDE telemetry", func(ev vehicle.Event) bool {
		return ev.Telemetry.GetAttitude() != nil
	})

	if got := ev.Telemetry.GetVehicleId().GetSystemId(); got != uint32(meta.SysID) {
		t.Errorf("telemetry sysid = %d, want %d", got, meta.SysID)
	}
}

// BRIDGE-SELF-FILTER: a frame carrying our own identity creates no vehicle.
//
// heartbeat_gcs_out is the GCS's own outbound heartbeat — (255, 190), the
// identity codec.NewNode transmits under. gomavlib does not loop our writes
// back, so this only happens when something else in the path does: a
// misconfigured UDP route, a mavproxy relay, or a second GCS on the network.
// Whichever it is, folding it produces a phantom "vehicle 255" that would then
// be published to fleet:active and shown to an operator as an aircraft.
func TestBridgeIgnoresItsOwnIdentity(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "heartbeat_gcs_out")

	if meta.SysID != codec.GCSSystemID || meta.CompID != codec.GCSComponentID {
		t.Fatalf("fixture identity (%d,%d) is not the GCS identity (%d,%d); "+
			"this test asserts nothing unless they match",
			meta.SysID, meta.CompID, codec.GCSSystemID, codec.GCSComponentID)
	}

	h := newHarness(t)

	h.feed(raw)

	h.sink.refute(t, "event from our own identity", func(vehicle.Event) bool { return true })

	key := routes.Key{SysID: meta.SysID, CompID: meta.CompID}
	if _, ok := h.bridge.Routes().Lookup(key, h.clock.Now()); ok {
		t.Error("a route was created for the GCS's own identity")
	}
}

// BRIDGE-EXPIRE: a vehicle that goes silent is declared lost by the sweep.
//
// The sweep exists because Fold only runs when a frame arrives, so the fold's
// own TTL check can never fire for the case it was written for: a vehicle that
// stops transmitting produces no more folds. Nothing but this tick closes that
// gap, which is why the clock is injected rather than slept through.
func TestBridgeExpiresSilentVehicle(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "heartbeat_v2")
	h := newHarness(t)

	h.feed(raw)
	h.sink.await(t, "VEHICLE_DISCOVERED",
		fleetOfType(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, uint32(meta.SysID)))

	h.clock.Advance(codec.HeartbeatTTL + time.Second)

	h.sink.await(t, "VEHICLE_LOST",
		fleetOfType(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST, uint32(meta.SysID)))
}

// BRIDGE-EXPIRE-ONCE: the sweep declares a vehicle lost once per outage.
//
// The sweep runs every few milliseconds in this harness against a clock that
// stays far past the TTL, so a fold that re-emitted on every tick would
// produce hundreds of VEHICLE_LOST events. The `Lost` flag in vehicle.State is
// what prevents that; this asserts the bridge actually carries the returned
// state forward rather than folding from the pre-sweep copy.
func TestBridgeExpiresOncePerOutage(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "heartbeat_v2")
	h := newHarness(t)

	h.feed(raw)
	h.sink.await(t, "VEHICLE_DISCOVERED",
		fleetOfType(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, uint32(meta.SysID)))

	h.clock.Advance(codec.HeartbeatTTL + time.Second)

	h.sink.await(t, "VEHICLE_LOST",
		fleetOfType(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST, uint32(meta.SysID)))

	// Let many more sweep ticks elapse at the same expired clock reading.
	time.Sleep(200 * time.Millisecond)

	lost := 0

	for _, ev := range h.sink.snapshot() {
		if ev.Fleet.GetType() == gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST {
			lost++
		}
	}

	if lost != 1 {
		t.Errorf("VEHICLE_LOST emitted %d times for one outage, want 1", lost)
	}
}

// BRIDGE-SINK-ERROR: a sink that refuses an event stops the pipeline.
//
// Deliberately fatal rather than logged-and-continued. The sink is what makes
// an observation durable; a bridge that keeps folding while nothing records
// the result presents as healthy and silently loses the fleet history the
// operator will later be asked to trust.
func TestBridgeStopsWhenSinkFails(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sink is down")
	raw, _ := loadFixture(t, "heartbeat_v2")

	h := newHarnessWithSink(t, &captureSink{err: sentinel})

	h.feed(raw)

	select {
	case err := <-h.runErr:
		h.runOnce.Do(func() { h.runErrIn = err })

		if !errors.Is(err, sentinel) {
			t.Errorf("Run returned %v, want it to wrap %v", err, sentinel)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after the sink failed")
	}
}

// BRIDGE-SHUTDOWN: cancelling the context stops Run cleanly.
//
// nil, not context.Canceled: cancellation here is the operator stopping the
// process, and an errgroup that reports its own shutdown as a failure makes
// every clean exit look like a crash in the logs.
func TestBridgeStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	h.cancel()

	if err := h.wait(); err != nil {
		t.Errorf("Run returned %v on cancellation, want nil", err)
	}
}

// New refuses a Config with no source rather than defaulting one.
//
// There is no sensible default: a bridge with no frame source is a process
// that binds nothing and reports healthy forever.
func TestNewRequiresSource(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); !errors.Is(err, ErrNoSource) {
		t.Errorf("New(Config{}) error = %v, want ErrNoSource", err)
	}
}

// New fills in every optional field, so a Config carrying only a Source runs.
func TestNewDefaultsOptionalFields(t *testing.T) {
	t.Parallel()

	b, err := New(Config{Source: stubSource{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if b.sink == nil || b.routes == nil || b.log == nil || b.now == nil {
		t.Error("New left a required collaborator nil")
	}

	if b.sweep != SweepInterval {
		t.Errorf("sweep = %v, want %v", b.sweep, SweepInterval)
	}
}

// splitLabel is diagnostics only, and every case it cannot parse must degrade
// to zero values rather than to a wrong address — routes.Upsert treats an
// empty SrcIP as "carry forward what you had", and a half-parsed one as truth.
func TestSplitLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label string
		host  string
		port  int
	}{
		{"udp:192.168.1.5:14550", "192.168.1.5", 14550},
		{"udp:[::1]:14550", "::1", 14550},
		{"tcp:10.0.0.2:5760", "10.0.0.2", 5760},
		// Client and custom endpoints carry a bare label with no peer.
		{"test", "", 0},
		{"serial:/dev/ttyUSB0", "", 0},
		{"udp:notaport", "", 0},
		{"", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()

			host, port := splitLabel(tc.label)
			if host != tc.host || port != tc.port {
				t.Errorf("splitLabel(%q) = (%q, %d), want (%q, %d)",
					tc.label, host, port, tc.host, tc.port)
			}
		})
	}
}

// stubSource is a FrameSource that never produces anything. It exists so
// TestNewDefaultsOptionalFields can construct a Bridge without a node.
type stubSource struct{}

var _ codec.FrameSource = stubSource{}

func (stubSource) Events() <-chan gomavlib.Event { return nil }

func (stubSource) WriteTo(codec.LinkID, message.Message) error { return nil }

func (stubSource) Close() error { return nil }
