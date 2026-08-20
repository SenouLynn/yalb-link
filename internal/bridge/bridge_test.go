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

func fleetOfType(t gcsv1.FleetEventType, sysID uint32) func(vehicle.Event) bool {
	return func(ev vehicle.Event) bool {
		return ev.Fleet.GetType() == t && ev.Fleet.GetVehicleId().GetSystemId() == sysID
	}
}

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

	if ev.Fleet.GetHeartbeat() == nil {
		t.Fatal("VEHICLE_DISCOVERED carried no HeartbeatState; consumers have nothing to render")
	}

	key := routes.Key{SysID: meta.SysID, CompID: meta.CompID}
	if _, ok := h.bridge.Routes().Lookup(key, h.clock.Now()); !ok {
		t.Errorf("no route for %s after discovery; the vehicle is visible but unaddressable", key)
	}
}

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

func TestBridgeStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	h.cancel()

	if err := h.wait(); err != nil {
		t.Errorf("Run returned %v on cancellation, want nil", err)
	}
}

func TestNewRequiresSource(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); !errors.Is(err, ErrNoSource) {
		t.Errorf("New(Config{}) error = %v, want ErrNoSource", err)
	}
}

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

// TestBridgeForwardsCommandAck runs a real COMMAND_ACK frame the whole way
// through, because that ack is the only evidence a rate request was honoured.
func TestBridgeForwardsCommandAck(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "command_ack_v2")
	h := newHarness(t)

	h.feed(raw)

	ev := h.sink.await(t, "COMMAND_ACK protocol event", func(ev vehicle.Event) bool {
		return ev.Protocol.GetCommandAck() != nil
	})

	if got := ev.Protocol.GetVehicleId().GetSystemId(); got != uint32(meta.SysID) {
		t.Errorf("sysid = %d, want %d", got, meta.SysID)
	}
}

// TestBridgeStampsTelemetryObservedAt proves the fold's injected clock reaches
// the wire, not just the unit test: a subscriber's freshness check is only as
// trustworthy as this stamp.
func TestBridgeStampsTelemetryObservedAt(t *testing.T) {
	t.Parallel()

	raw, _ := loadFixture(t, "attitude_v2")
	h := newHarness(t)

	h.feed(raw)

	ev := h.sink.await(t, "ATTITUDE telemetry", func(ev vehicle.Event) bool {
		return ev.Telemetry.GetAttitude() != nil
	})

	stamp := ev.Telemetry.GetAttitude().GetObservedAt()
	if stamp == nil {
		t.Fatal("ATTITUDE carried no observed_at; downstream freshness has nothing to measure")
	}

	if got := stamp.AsTime().UnixMilli(); got != clockEpochMs {
		t.Errorf("observed_at = %d ms, want the injected clock's %d ms", got, clockEpochMs)
	}
}
