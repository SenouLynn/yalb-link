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

// recordFlight persists count telemetry events and returns the stopped recording.
func recordFlight(t *testing.T, store *Store, count int) Recording {
	t.Helper()
	recording, err := store.StartRecording(context.Background(), "paging flight")
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	recorder := &Recorder{Store: store}
	at := time.Unix(1_800_000_000, 0).UTC()
	for i := range count {
		event := vehicle.Event{Telemetry: &gcsv1.TelemetryEvent{
			VehicleId: &gcsv1.VehicleId{SystemId: 7, ComponentId: 1},
			Payload: &gcsv1.TelemetryEvent_Attitude{Attitude: &gcsv1.Attitude{
				RollRad:    float32(i),
				ObservedAt: timestamppb.New(at.Add(time.Duration(i) * time.Millisecond)),
			}},
		}}
		if err := recorder.Publish(context.Background(), event); err != nil {
			t.Fatalf("Publish(%d) error = %v", i, err)
		}
	}
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}
	return recording
}

func TestReplayFromPagesInSequenceOrder(t *testing.T) {
	store := newTestStore(t, nil)
	recording := recordFlight(t, store, 25)

	all, err := store.Replay(context.Background(), recording.ID)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(all) != 25 {
		t.Fatalf("Replay() length = %d, want 25", len(all))
	}

	// Concatenated pages must equal one unbounded read, or the display would
	// silently skip or duplicate telemetry across a page boundary.
	var paged []ReplayEvent
	for fromSeq := int64(0); ; {
		page, err := store.ReplayFrom(context.Background(), recording.ID, fromSeq, 10)
		if err != nil {
			t.Fatalf("ReplayFrom(%d) error = %v", fromSeq, err)
		}
		if len(page) == 0 {
			break
		}
		paged = append(paged, page...)
		fromSeq = page[len(page)-1].Seq + 1
	}

	if len(paged) != len(all) {
		t.Fatalf("paged length = %d, want %d", len(paged), len(all))
	}
	for i := range all {
		if paged[i].Seq != all[i].Seq {
			t.Errorf("page event %d seq = %d, want %d", i, paged[i].Seq, all[i].Seq)
		}
		if !proto.Equal(paged[i].Telemetry, all[i].Telemetry) {
			t.Errorf("page event %d payload mismatch", i)
		}
	}
}

func TestReplayFromBoundaries(t *testing.T) {
	store := newTestStore(t, nil)
	recording := recordFlight(t, store, 5)

	tests := []struct {
		name     string
		fromSeq  int64
		limit    int
		wantLen  int
		wantHead int64
	}{
		{name: "unbounded limit reads all", fromSeq: 0, limit: 0, wantLen: 5, wantHead: 1},
		{name: "negative limit reads all", fromSeq: 0, limit: -1, wantLen: 5, wantHead: 1},
		{name: "limit truncates", fromSeq: 0, limit: 2, wantLen: 2, wantHead: 1},
		{name: "from_seq is inclusive", fromSeq: 3, limit: 0, wantLen: 3, wantHead: 3},
		{name: "from_seq past the end is empty", fromSeq: 99, limit: 0, wantLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := store.ReplayFrom(context.Background(), recording.ID, tc.fromSeq, tc.limit)
			if err != nil {
				t.Fatalf("ReplayFrom() error = %v", err)
			}
			if len(page) != tc.wantLen {
				t.Fatalf("length = %d, want %d", len(page), tc.wantLen)
			}
			if tc.wantLen > 0 && page[0].Seq != tc.wantHead {
				t.Errorf("head seq = %d, want %d", page[0].Seq, tc.wantHead)
			}
		})
	}
}

func TestReplayFromUnknownRecordingIsEmpty(t *testing.T) {
	store := newTestStore(t, nil)

	// Absence of events is not an error here; the handler distinguishes an
	// unknown recording from an empty one by looking up its row.
	page, err := store.ReplayFrom(context.Background(), 404, 0, 10)
	if err != nil {
		t.Fatalf("ReplayFrom() error = %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("length = %d, want 0", len(page))
	}
}

func TestCommandRoundTripsThroughRecording(t *testing.T) {
	store := newTestStore(t, nil)
	recording, err := store.StartRecording(context.Background(), "command")
	if err != nil {
		t.Fatal(err)
	}
	at := timestamppb.New(time.Unix(1_800_000_000, 0).UTC())
	tx := &gcsv1.CommandTransaction{Id: 9, VehicleId: &gcsv1.VehicleId{SystemId: 1, ComponentId: 1},
		Command: gcsv1.MavCmd_MAV_CMD_COMPONENT_ARM_DISARM, State: gcsv1.CommandState_COMMAND_STATE_ACCEPTED,
		IssuedAt: at, SettledAt: at}
	if err := (&Recorder{Store: store}).Publish(context.Background(), vehicle.Event{Command: tx}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, err := store.Replay(context.Background(), recording.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !proto.Equal(events[0].Command, tx) {
		t.Fatalf("replayed command = %+v", events)
	}
}
