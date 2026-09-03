// Package mission holds the read-only mission download contract and, from
// T-005, the coordinator that fills it.
package mission

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// observedAt is a fixed instant: a snapshot's timestamp must survive the
// round trip, and reading the clock here would hide an encoding bug.
var observedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func snapshot(items ...*gcsv1.MissionItem) *gcsv1.MissionSnapshot {
	return &gcsv1.MissionSnapshot{
		VehicleId:   &gcsv1.VehicleId{SystemId: 1, ComponentId: 1},
		MissionType: gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
		Items:       items,
		ObservedAt:  timestamppb.New(observedAt),
	}
}

// TestSnapshotIdentifiesVehicleMissionTypeAndObservationTime covers the four
// things a consumer needs before it can trust a snapshot: which vehicle it came
// from, which mission type it describes, and when it was observed.
func TestSnapshotIdentifiesVehicleMissionTypeAndObservationTime(t *testing.T) {
	t.Parallel()

	snap := snapshot()

	if snap.GetVehicleId().GetSystemId() != 1 || snap.GetVehicleId().GetComponentId() != 1 {
		t.Errorf("vehicle_id = %v, want system 1 component 1", snap.GetVehicleId())
	}

	if snap.GetMissionType() != gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION {
		t.Errorf("mission_type = %v, want MAV_MISSION_TYPE_MISSION", snap.GetMissionType())
	}

	if got := snap.GetObservedAt().AsTime(); !got.Equal(observedAt) {
		t.Errorf("observed_at = %v, want %v", got, observedAt)
	}
}

// TestEmptyMissionIsACompletedSnapshotNotAnError is the case a mission download
// is easiest to get wrong. A vehicle that reports zero items has answered the
// question; the contract must be able to say "no waypoints" without inventing
// one and without the absence being mistaken for a failed or pending download.
func TestEmptyMissionIsACompletedSnapshotNotAnError(t *testing.T) {
	t.Parallel()

	raw, err := protojson.Marshal(snapshot())
	if err != nil {
		t.Fatalf("marshalling empty snapshot: %v", err)
	}

	var back gcsv1.MissionSnapshot
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshalling empty snapshot: %v", err)
	}

	if len(back.GetItems()) != 0 {
		t.Errorf("items = %d, want 0 — an empty mission must not gain a waypoint", len(back.GetItems()))
	}

	// observed_at is what separates a completed empty download from a zero
	// value that nobody filled in. Without it, "no items" is unreadable.
	if back.GetObservedAt() == nil {
		t.Error("observed_at is nil after round trip; an empty snapshot cannot show it completed")
	}

	if !back.GetObservedAt().AsTime().Equal(observedAt) {
		t.Errorf("observed_at = %v, want %v", back.GetObservedAt().AsTime(), observedAt)
	}

	// The wire form is what the HTTP endpoint and browser actually see. proto3
	// omits an empty repeated field, so "no items" is carried by the absence of
	// `items` — which is only readable because the surrounding fields are
	// present. An unfilled snapshot marshals to `{}` and cannot be confused
	// with this one.
	if string(raw) == "{}" {
		t.Fatal("completed empty snapshot marshalled to {}; it is indistinguishable from an unset message")
	}

	if !strings.Contains(string(raw), `"observedAt"`) {
		t.Errorf("empty snapshot JSON has no observedAt: %s", raw)
	}

	if strings.Contains(string(raw), `"items"`) {
		t.Errorf("empty snapshot JSON invented an items field: %s", raw)
	}
}

// TestSnapshotPreservesItemOrder guards the property the map overlay depends on:
// mission items are a route, so their order is part of the data, not incidental.
func TestSnapshotPreservesItemOrder(t *testing.T) {
	t.Parallel()

	snap := snapshot(
		&gcsv1.MissionItem{Seq: 0, Command: gcsv1.MavCmd_MAV_CMD_NAV_TAKEOFF},
		&gcsv1.MissionItem{Seq: 1, Command: gcsv1.MavCmd_MAV_CMD_NAV_WAYPOINT},
		&gcsv1.MissionItem{Seq: 2, Command: gcsv1.MavCmd_MAV_CMD_NAV_LAND},
	)

	raw, err := protojson.Marshal(snap)
	if err != nil {
		t.Fatalf("marshalling snapshot: %v", err)
	}

	var back gcsv1.MissionSnapshot
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshalling snapshot: %v", err)
	}

	want := []gcsv1.MavCmd{
		gcsv1.MavCmd_MAV_CMD_NAV_TAKEOFF,
		gcsv1.MavCmd_MAV_CMD_NAV_WAYPOINT,
		gcsv1.MavCmd_MAV_CMD_NAV_LAND,
	}

	if len(back.GetItems()) != len(want) {
		t.Fatalf("items = %d, want %d", len(back.GetItems()), len(want))
	}

	for i, wantCmd := range want {
		item := back.GetItems()[i]

		if uint32(i) != item.GetSeq() {
			t.Errorf("items[%d].seq = %d, want %d", i, item.GetSeq(), i)
		}

		if item.GetCommand() != wantCmd {
			t.Errorf("items[%d].command = %v, want %v", i, item.GetCommand(), wantCmd)
		}
	}
}
