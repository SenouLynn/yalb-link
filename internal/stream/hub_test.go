package stream

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"go.uber.org/goleak"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

// TestMain asserts the hub starts no goroutines of its own. Fan-out happens on
// the publisher's goroutine by design; a background pump here would be a place
// for events to be reordered or lost during shutdown.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHub(t *testing.T, queue int) *Hub {
	t.Helper()

	h := NewHub(discard(), queue)
	t.Cleanup(h.Close)

	return h
}

func fleetEvent(kind gcsv1.FleetEventType, sysID, compID uint32) vehicle.Event {
	return vehicle.Event{Fleet: &gcsv1.FleetEvent{
		Type:      kind,
		VehicleId: &gcsv1.VehicleId{SystemId: sysID, ComponentId: compID},
		Heartbeat: &gcsv1.HeartbeatState{Type: gcsv1.MavType_MAV_TYPE_QUADROTOR},
	}}
}

func attitudeEvent(sysID, compID uint32, roll float32) vehicle.Event {
	return vehicle.Event{Telemetry: &gcsv1.TelemetryEvent{
		VehicleId: &gcsv1.VehicleId{SystemId: sysID, ComponentId: compID},
		Payload:   &gcsv1.TelemetryEvent_Attitude{Attitude: &gcsv1.Attitude{RollRad: roll}},
	}}
}

func vfrEvent(sysID, compID uint32, groundspeed float32) vehicle.Event {
	return vehicle.Event{Telemetry: &gcsv1.TelemetryEvent{
		VehicleId: &gcsv1.VehicleId{SystemId: sysID, ComponentId: compID},
		Payload:   &gcsv1.TelemetryEvent_VfrHud{VfrHud: &gcsv1.VfrHud{GroundspeedMS: groundspeed}},
	}}
}

// drain reads everything currently buffered without blocking.
func drain(events <-chan Event) []Event {
	var out []Event

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return out
			}

			out = append(out, ev)
		default:
			return out
		}
	}
}

// names projects events to their SSE event names.
func names(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Name)
	}

	return out
}

func publishAll(t *testing.T, h *Hub, events ...vehicle.Event) {
	t.Helper()

	for _, ev := range events {
		if err := h.Publish(t.Context(), ev); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

// TestSubscribeBootstrapsRetainedState is the reason the hub retains anything:
// a browser opened after the vehicle appeared must still see it.
func TestSubscribeBootstrapsRetainedState(t *testing.T) {
	h := newTestHub(t, 8)

	publishAll(t, h,
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1),
		attitudeEvent(1, 1, 0.25),
	)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	got := drain(events)

	want := []string{EventFleet, EventTelemetry}
	if diff := names(got); len(diff) != len(want) || diff[0] != want[0] || diff[1] != want[1] {
		t.Fatalf("bootstrap = %v, want %v", diff, want)
	}

	//nolint:forcetypeassert // the event name asserts the message type.
	if roll := got[1].Message.(*gcsv1.TelemetryEvent).GetAttitude().GetRollRad(); roll != 0.25 {
		t.Errorf("bootstrap attitude roll = %v, want 0.25", roll)
	}
}

// TestBootstrapEmitsFleetBeforeTelemetry pins the documented order. A
// subscriber that learned about a vehicle from a telemetry event would have to
// invent its identity before the fleet event arrived to correct it.
func TestBootstrapEmitsFleetBeforeTelemetry(t *testing.T) {
	h := newTestHub(t, 32)

	publishAll(t, h,
		attitudeEvent(2, 1, 0.1),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 2, 1),
		vfrEvent(2, 1, 12),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1),
		attitudeEvent(1, 1, 0.2),
	)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	got := names(drain(events))

	// Two fleet events, then three telemetry events.
	want := []string{EventFleet, EventFleet, EventTelemetry, EventTelemetry, EventTelemetry}

	if len(got) != len(want) {
		t.Fatalf("bootstrap = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bootstrap = %v, want %v", got, want)
		}
	}
}

// TestBootstrapOrderIsStable makes two subscribers of the same hub receive the
// identical sequence, so the ordering claim does not depend on map iteration.
func TestBootstrapOrderIsStable(t *testing.T) {
	h := newTestHub(t, 64)

	// Published out of identity order on purpose.
	publishAll(t, h,
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 3, 1),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 2, 1),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 190),
		vfrEvent(3, 1, 1),
		attitudeEvent(3, 1, 1),
		attitudeEvent(1, 1, 1),
	)

	identities := func() []vehicleKey {
		events, unsubscribe := h.Subscribe()
		defer unsubscribe()

		var out []vehicleKey

		for _, ev := range drain(events) {
			switch msg := ev.Message.(type) {
			case *gcsv1.FleetEvent:
				out = append(out, keyOfFleet(msg))
			case *gcsv1.TelemetryEvent:
				out = append(out, keyOfTelemetry(msg))
			}
		}

		return out
	}

	first, second := identities(), identities()

	if len(first) != len(second) {
		t.Fatalf("bootstraps differ in length: %d vs %d", len(first), len(second))
	}

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("bootstrap %d = %+v, second subscriber got %+v", i, first[i], second[i])
		}
	}

	// Fleet events ascend by (sysid, compid).
	wantFleet := []vehicleKey{{1, 1}, {1, 190}, {2, 1}, {3, 1}}
	for i, want := range wantFleet {
		if first[i] != want {
			t.Errorf("fleet bootstrap %d = %+v, want %+v", i, first[i], want)
		}
	}
}

// TestTelemetryRetainedPerFamily keeps one family from evicting another: an
// ATTITUDE at 10 Hz must not push VFR_HUD out of the bootstrap.
func TestTelemetryRetainedPerFamily(t *testing.T) {
	h := newTestHub(t, 32)

	publishAll(t, h,
		attitudeEvent(1, 1, 0.1),
		vfrEvent(1, 1, 5),
		attitudeEvent(1, 1, 0.2),
		attitudeEvent(1, 1, 0.3),
	)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	got := drain(events)

	if len(got) != 2 {
		t.Fatalf("bootstrap has %d telemetry events, want one per family (2)", len(got))
	}

	var roll, speed float32

	for _, ev := range got {
		//nolint:forcetypeassert // this hub only retains telemetry here.
		evt := ev.Message.(*gcsv1.TelemetryEvent)
		if a := evt.GetAttitude(); a != nil {
			roll = a.GetRollRad()
		}

		if v := evt.GetVfrHud(); v != nil {
			speed = v.GetGroundspeedMS()
		}
	}

	if roll != 0.3 {
		t.Errorf("retained roll = %v, want the latest 0.3", roll)
	}

	if speed != 5 {
		t.Errorf("retained groundspeed = %v, want 5; a fast family evicted a slow one", speed)
	}
}

// TestVehiclesAreIsolated keeps one vehicle's telemetry out of another's slot.
func TestVehiclesAreIsolated(t *testing.T) {
	h := newTestHub(t, 32)

	publishAll(t, h,
		attitudeEvent(1, 1, 0.1),
		attitudeEvent(2, 1, 0.9),
		attitudeEvent(1, 190, 0.5),
	)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	got := drain(events)
	if len(got) != 3 {
		t.Fatalf("retained %d events, want 3 distinct identities", len(got))
	}
}

// TestLatestFleetEventWins means a reconnecting browser learns the vehicle is
// gone rather than replaying the discovery that is no longer true.
func TestLatestFleetEventWins(t *testing.T) {
	h := newTestHub(t, 32)

	publishAll(t, h,
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1),
		fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST, 1, 1),
	)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	got := drain(events)
	if len(got) != 1 {
		t.Fatalf("retained %d fleet events for one vehicle, want 1", len(got))
	}

	//nolint:forcetypeassert // only fleet events were published.
	if kind := got[0].Message.(*gcsv1.FleetEvent).GetType(); kind != gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST {
		t.Errorf("bootstrap reports %v, want VEHICLE_LOST", kind)
	}
}

// TestLiveEventsFollowBootstrap covers the ordinary case: subscribe, then
// receive what happens next.
func TestLiveEventsFollowBootstrap(t *testing.T) {
	h := newTestHub(t, 32)

	publishAll(t, h, fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	publishAll(t, h, attitudeEvent(1, 1, 0.75))

	got := drain(events)

	if len(got) != 2 {
		t.Fatalf("received %d events, want bootstrap + 1 live", len(got))
	}

	//nolint:forcetypeassert // the second event is the telemetry just published.
	if roll := got[1].Message.(*gcsv1.TelemetryEvent).GetAttitude().GetRollRad(); roll != 0.75 {
		t.Errorf("live roll = %v, want 0.75", roll)
	}
}

// TestSlowSubscriberIsEvicted is the back-pressure boundary. The hub must drop
// the subscriber rather than block, because blocking here blocks the MAVLink
// receive loop.
func TestSlowSubscriberIsEvicted(t *testing.T) {
	h := newTestHub(t, 4)

	events, unsubscribe := h.Subscribe()
	defer unsubscribe()

	// Never read; overrun the queue.
	for i := range 32 {
		if err := h.Publish(t.Context(), attitudeEvent(1, 1, float32(i))); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	if got := h.Subscribers(); got != 0 {
		t.Fatalf("hub still holds %d subscribers; the slow one was not evicted", got)
	}

	// The channel is closed, so the handler's read loop terminates.
	for range events { //nolint:revive // draining to the close is the assertion.
	}
}

// TestEvictionDoesNotStopPublishing proves the eviction is contained: the hub
// keeps recording state for the next subscriber.
func TestEvictionDoesNotStopPublishing(t *testing.T) {
	h := newTestHub(t, 2)

	slow, unsubscribeSlow := h.Subscribe()
	defer unsubscribeSlow()

	for i := range 16 {
		if err := h.Publish(t.Context(), attitudeEvent(1, 1, float32(i))); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	for range slow { //nolint:revive // draining to the close.
	}

	if err := h.Publish(t.Context(), vfrEvent(1, 1, 42)); err != nil {
		t.Fatalf("Publish after eviction: %v", err)
	}

	fresh, unsubscribeFresh := h.Subscribe()
	defer unsubscribeFresh()

	var speed float32

	for _, ev := range drain(fresh) {
		//nolint:forcetypeassert // only telemetry was published.
		if v := ev.Message.(*gcsv1.TelemetryEvent).GetVfrHud(); v != nil {
			speed = v.GetGroundspeedMS()
		}
	}

	if speed != 42 {
		t.Errorf("post-eviction groundspeed = %v, want 42", speed)
	}
}

// TestReconnectGetsFreshBootstrap is the recovery path an evicted or
// disconnected browser takes.
func TestReconnectGetsFreshBootstrap(t *testing.T) {
	h := newTestHub(t, 8)

	first, unsubscribeFirst := h.Subscribe()
	publishAll(t, h, fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))
	unsubscribeFirst()

	// The dropped subscriber's channel is closed rather than left dangling.
	for range first { //nolint:revive // draining to the close.
	}

	publishAll(t, h, attitudeEvent(1, 1, 0.4))

	second, unsubscribeSecond := h.Subscribe()
	defer unsubscribeSecond()

	if got := len(drain(second)); got != 2 {
		t.Errorf("reconnect bootstrap has %d events, want the full fleet + telemetry state (2)", got)
	}
}

func TestCommandIsLiveButNotBootstrapped(t *testing.T) {
	h := newTestHub(t, 4)
	events, unsubscribe := h.Subscribe()
	defer unsubscribe()
	if err := h.Publish(context.Background(), vehicle.Event{Command: &gcsv1.CommandTransaction{
		Id: 7, VehicleId: &gcsv1.VehicleId{SystemId: 1, ComponentId: 1},
		State: gcsv1.CommandState_COMMAND_STATE_ACCEPTED,
	}}); err != nil {
		t.Fatal(err)
	}
	got := <-events
	if got.Name != EventCommand {
		t.Fatalf("live event = %q, want command", got.Name)
	}

	bootstrap, unsubscribe2 := h.Subscribe()
	defer unsubscribe2()
	select {
	case event := <-bootstrap:
		t.Fatalf("command retained in bootstrap: %v", event)
	default:
	}
}

// TestUnsubscribeIsIdempotent covers the handler's defer running after the hub
// already evicted or closed the same subscriber.
func TestUnsubscribeIsIdempotent(t *testing.T) {
	h := newTestHub(t, 4)

	_, unsubscribe := h.Subscribe()

	unsubscribe()
	unsubscribe()
	h.Close()
	unsubscribe()
}

// TestConcurrentPublishAndSubscribe is the -race assertion: the bridge
// publishes on one goroutine while browsers connect and disconnect on others.
func TestConcurrentPublishAndSubscribe(t *testing.T) {
	h := newTestHub(t, 64)

	stop := make(chan struct{})

	var publisher sync.WaitGroup

	// One publisher, mirroring the bridge's single-owner receive loop.
	publisher.Add(1)

	go func() {
		defer publisher.Done()

		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}

			sysID := uint32(i%4) + 1

			//nolint:errcheck // Publish is documented never to fail.
			_ = h.Publish(context.Background(), attitudeEvent(sysID, 1, float32(i)))
			_ = h.Publish(context.Background(), fleetEvent(
				gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED, sysID, 1))
		}
	}()

	// Several browsers connecting, reading a little, and leaving. Some will be
	// evicted mid-read; that is the point.
	var readers sync.WaitGroup

	for range 8 {
		readers.Add(1)

		go func() {
			defer readers.Done()

			for range 20 {
				events, unsubscribe := h.Subscribe()

				for range 5 {
					if _, ok := <-events; !ok {
						break
					}
				}

				unsubscribe()
			}
		}()
	}

	readers.Wait()
	close(stop)
	publisher.Wait()

	if got := h.Subscribers(); got != 0 {
		t.Errorf("%d subscribers left registered after every reader unsubscribed", got)
	}
}
