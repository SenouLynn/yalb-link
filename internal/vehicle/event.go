package vehicle

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// Event contains exactly one fold output.
type Event struct {
	Fleet     *gcsv1.FleetEvent
	Telemetry *gcsv1.TelemetryEvent
	Protocol  *gcsv1.ProtocolEvent
	Command   *gcsv1.CommandTransaction
	Warning   *Warning
}

// WarningType classifies a local transport anomaly.
type WarningType uint8

const (
	// WarningUnspecified is the zero value and never emitted.
	WarningUnspecified WarningType = iota
	// WarningSourceConflict marks a system ID changing source address.
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

// fleetEvent stamps a FleetEvent with the injected clock.
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
