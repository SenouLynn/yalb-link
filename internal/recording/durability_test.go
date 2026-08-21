package recording

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

func TestRecordRestartReplayRoundTrip(t *testing.T) {
	path := t.TempDir() + "/flight.db"
	clock := time.Unix(1_800_000_000, 0).UTC()
	store, err := Open(Config{Path: path, Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	recording, err := store.StartRecording(context.Background(), "deterministic flight")
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	events := deterministicEvents(clock)
	recorder := &Recorder{Store: store}
	for _, event := range events {
		if err := recorder.Publish(context.Background(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(Config{Path: path, Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	replayed, err := reopened.Replay(context.Background(), recording.ID)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(replayed) != len(events) {
		t.Fatalf("Replay() length = %d, want %d", len(replayed), len(events))
	}

	for i, got := range replayed {
		want := events[i]
		if got.Seq != int64(i+1) {
			t.Errorf("event %d seq = %d, want %d", i, got.Seq, i+1)
		}
		switch {
		case want.Fleet != nil && !proto.Equal(got.Fleet, want.Fleet):
			t.Errorf("event %d fleet mismatch: got %v want %v", i, got.Fleet, want.Fleet)
		case want.Telemetry != nil && !proto.Equal(got.Telemetry, want.Telemetry):
			t.Errorf("event %d telemetry mismatch: got %v want %v", i, got.Telemetry, want.Telemetry)
		case want.Fleet != nil && got.Telemetry != nil:
			t.Errorf("event %d kind = telemetry, want fleet", i)
		case want.Telemetry != nil && got.Fleet != nil:
			t.Errorf("event %d kind = fleet, want telemetry", i)
		}
	}
}

func deterministicEvents(at time.Time) []vehicle.Event {
	id := &gcsv1.VehicleId{SystemId: 7, ComponentId: 1}
	return []vehicle.Event{
		{Fleet: &gcsv1.FleetEvent{Type: gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, VehicleId: id, OccurredAt: timestamppb.New(at)}},
		{Telemetry: &gcsv1.TelemetryEvent{VehicleId: id, Payload: &gcsv1.TelemetryEvent_Attitude{Attitude: &gcsv1.Attitude{RollRad: 0.25, ObservedAt: timestamppb.New(at.Add(time.Millisecond))}}}},
		{Telemetry: &gcsv1.TelemetryEvent{VehicleId: id, Payload: &gcsv1.TelemetryEvent_GlobalPosition{GlobalPosition: &gcsv1.GlobalPosition{LatDeg: 40.7, LonDeg: -74, ObservedAt: timestamppb.New(at.Add(2 * time.Millisecond))}}}},
		{Fleet: &gcsv1.FleetEvent{Type: gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED, VehicleId: id, OccurredAt: timestamppb.New(at.Add(3 * time.Millisecond))}},
	}
}
