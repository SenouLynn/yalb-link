package vehicle

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// Event is one observation the fold produced from a single message.
//
// Exactly one field is non-nil. The three go to different places — a
// FleetEvent to the fleet stream and the fleet:active set, a TelemetryEvent to
// the per-vehicle pub/sub fan-out, a Warning to the operator log — so a single
// merged type would only push the discrimination downstream.
type Event struct {
	Fleet     *gcsv1.FleetEvent
	Telemetry *gcsv1.TelemetryEvent
	Warning   *Warning
}

// WarningType classifies an operator-visible anomaly the fold detected.
//
// These are Go types rather than protos on purpose. A warning is a statement
// about the local transport ("two hosts are claiming to be system 2"), not
// vehicle state, and nothing streams it to a client yet. Adding a proto
// message commits the wire contract in Tier 0 terms — buf breaking then owns
// it forever — for a shape Tier 5 has not yet had to use. Promote it when a
// client needs to render it.
type WarningType uint8

const (
	// WarningUnspecified is the zero value and never emitted.
	WarningUnspecified WarningType = iota
	// WarningSourceConflict fires when frames claiming one system ID arrive
	// from a different source address than before. Either a second vehicle is
	// misconfigured with a duplicate system ID, a vehicle moved to a new link,
	// or someone is injecting frames. All three deserve a line in the log; see
	// the transport threat model in docs/roadmap/tier-4-bridge-core.md.
	WarningSourceConflict
)

// String names the warning for logs.
func (t WarningType) String() string {
	switch t {
	case WarningSourceConflict:
		return "SOURCE_CONFLICT"
	case WarningUnspecified:
		return "UNSPECIFIED"
	default:
		return fmt.Sprintf("WarningType(%d)", uint8(t))
	}
}

// Warning is an operator-visible anomaly with the evidence attached.
type Warning struct {
	VehicleID *gcsv1.VehicleId
	Previous  string
	Current   string

	OccurredMs int64
	Type       WarningType
}

// String renders the warning as a single log line.
func (w Warning) String() string {
	return fmt.Sprintf("%s sysid=%d compid=%d previous=%q current=%q",
		w.Type, w.VehicleID.GetSystemId(), w.VehicleID.GetComponentId(), w.Previous, w.Current)
}

// fleetEvent builds a FleetEvent stamped with the injected clock.
//
// occurred_at comes from nowMs, not from time.Now(): a replayed log and a live
// link must produce identical events, and a timestamp read from the wall clock
// inside the fold would make every test non-deterministic.
func fleetEvent(t gcsv1.FleetEventType, id *gcsv1.VehicleId, hb *gcsv1.HeartbeatState, nowMs int64) Event {
	return Event{Fleet: &gcsv1.FleetEvent{
		Type:       t,
		VehicleId:  id,
		Heartbeat:  hb,
		OccurredAt: timestampOf(nowMs),
	}}
}

func timestampOf(nowMs int64) *timestamppb.Timestamp {
	return timestamppb.New(time.UnixMilli(nowMs).UTC())
}
