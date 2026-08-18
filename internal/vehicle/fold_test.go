package vehicle

import (
	"testing"

	"go.uber.org/goleak"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// TestMain asserts no goroutine leaks. This package starts none — that is the
// claim, and it is worth pinning, because the fold is the thing most likely to
// grow a background ticker the first time someone wants "just a timeout".
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// The clock is a plain counter. Nothing here is derived from the wall clock,
// so the numbers are chosen to make the arithmetic readable in a failure.
const (
	t0    int64 = 1_000_000
	ttlMs       = heartbeatTTLMs
)

func heartbeat(armed bool, customMode uint32) *gcsv1.HeartbeatState {
	return &gcsv1.HeartbeatState{
		Type:         gcsv1.MavType_MAV_TYPE_QUADROTOR,
		Autopilot:    gcsv1.MavAutopilot_MAV_AUTOPILOT_ARDUPILOTMEGA,
		SystemStatus: gcsv1.MavState_MAV_STATE_STANDBY,
		Armed:        armed,
		CustomMode:   customMode,
	}
}

func beat(armed bool, customMode uint32) Inbound {
	return Inbound{
		Heartbeat: heartbeat(armed, customMode),
		SrcAddr:   "10.0.0.5:14550",
		Decoded:   codec.Decoded{SysID: 1, CompID: 1},
		MsgID:     0,
	}
}

func telem(evt *gcsv1.TelemetryEvent, msgID uint32) Inbound {
	return Inbound{
		SrcAddr: "10.0.0.5:14550",
		Decoded: codec.Decoded{Telemetry: evt, SysID: 1, CompID: 1},
		MsgID:   msgID,
	}
}

func fleetTypes(t *testing.T, events []Event) []gcsv1.FleetEventType {
	t.Helper()

	var out []gcsv1.FleetEventType

	for _, e := range events {
		if e.Fleet != nil {
			out = append(out, e.Fleet.GetType())
		}
	}

	return out
}

func onlyFleet(t *testing.T, events []Event) gcsv1.FleetEventType {
	t.Helper()

	types := fleetTypes(t, events)
	if len(types) != 1 {
		t.Fatalf("expected exactly one fleet event, got %v", types)
	}

	return types[0]
}

func TestFoldDiscoversOnFirstHeartbeat(t *testing.T) {
	state, events := Fold(State{}, beat(false, 3), t0)

	if got := onlyFleet(t, events); got != gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED {
		t.Errorf("first heartbeat emitted %v, want VEHICLE_DISCOVERED", got)
	}

	if !state.Known {
		t.Error("state not marked known after first heartbeat")
	}

	if state.VehicleType != gcsv1.MavType_MAV_TYPE_QUADROTOR {
		t.Errorf("vehicle type = %v, want QUADROTOR", state.VehicleType)
	}

	if state.FirstSeenMs != t0 || state.LastSeenMs != t0 || state.LastHeartbeatMs != t0 {
		t.Errorf("timestamps = first %d last %d hb %d, want %d for all",
			state.FirstSeenMs, state.LastSeenMs, state.LastHeartbeatMs, t0)
	}

	// occurred_at is the injected clock, not the wall clock.
	if got := events[0].Fleet.GetOccurredAt().AsTime().UnixMilli(); got != t0 {
		t.Errorf("occurred_at = %d, want %d", got, t0)
	}
}

// Telemetry alone does not announce a vehicle: identity, type and armed state
// all come from HEARTBEAT.
func TestFoldTelemetryBeforeHeartbeatDoesNotDiscover(t *testing.T) {
	state, events := Fold(State{}, telem(attitudeEvent(0.1, 0.2, 0.3), 30), t0)

	if types := fleetTypes(t, events); len(types) != 0 {
		t.Errorf("telemetry from an unknown vehicle emitted %v, want no fleet event", types)
	}

	if state.Known {
		t.Error("telemetry marked the vehicle known")
	}

	if len(events) != 1 || events[0].Telemetry == nil {
		t.Fatalf("expected one telemetry pass-through event, got %d", len(events))
	}
}

func TestFoldHeartbeatUpdatedOnlyOnChange(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	// Identical heartbeat one second later: nothing changed, nothing emitted.
	state, events := Fold(state, beat(false, 3), t0+1_000)
	if types := fleetTypes(t, events); len(types) != 0 {
		t.Errorf("unchanged heartbeat emitted %v, want nothing", types)
	}

	state, events = Fold(state, beat(true, 3), t0+2_000)
	if got := onlyFleet(t, events); got != gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED {
		t.Errorf("arming emitted %v, want HEARTBEAT_UPDATED", got)
	}

	if !state.Armed {
		t.Error("armed flag not carried into state")
	}

	_, events = Fold(state, beat(true, 4), t0+3_000)
	if got := onlyFleet(t, events); got != gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED {
		t.Errorf("mode change emitted %v, want HEARTBEAT_UPDATED", got)
	}
}

// A message arriving after the TTL reports both the outage and its end.
// Swallowing the loss because the vehicle came back would leave the fleet
// stream claiming contact that never happened.
func TestFoldLostThenRecovered(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	state, events := Fold(state, beat(false, 3), t0+ttlMs+1)

	want := []gcsv1.FleetEventType{
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST,
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_RECOVERED,
	}

	got := fleetTypes(t, events)
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("late heartbeat emitted %v, want %v", got, want)
	}

	if state.Lost {
		t.Error("state still marked lost after recovery")
	}
}

// Exactly on the TTL boundary is not yet an outage.
func TestFoldTTLBoundaryIsNotLost(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	_, events := Fold(state, beat(false, 3), t0+ttlMs)
	if types := fleetTypes(t, events); len(types) != 0 {
		t.Errorf("heartbeat at exactly the TTL emitted %v, want nothing", types)
	}
}

func TestExpireEmitsLostOnce(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	state, events := Expire(state, t0+ttlMs)
	if types := fleetTypes(t, events); len(types) != 0 {
		t.Fatalf("sweep inside the TTL emitted %v, want nothing", types)
	}

	state, events = Expire(state, t0+ttlMs+1)
	if got := onlyFleet(t, events); got != gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST {
		t.Fatalf("sweep past the TTL emitted %v, want VEHICLE_LOST", got)
	}

	if !state.Lost {
		t.Error("state not marked lost")
	}

	// A silent vehicle is swept repeatedly; it is lost once.
	state, events = Expire(state, t0+ttlMs+2_000)
	if len(events) != 0 {
		t.Errorf("second sweep emitted %d events, want 0", len(events))
	}

	// Recovery still reports as recovery after a sweep-driven loss.
	_, events = Fold(state, beat(false, 3), t0+ttlMs+3_000)
	if got := onlyFleet(t, events); got != gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_RECOVERED {
		t.Errorf("heartbeat after sweep emitted %v, want VEHICLE_RECOVERED", got)
	}
}

// An unknown vehicle is never expired: there is nothing to have lost.
func TestExpireIgnoresUnknownVehicle(t *testing.T) {
	if _, events := Expire(State{}, t0); len(events) != 0 {
		t.Errorf("expiring an unknown vehicle emitted %d events, want 0", len(events))
	}
}
