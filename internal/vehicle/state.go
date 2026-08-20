// Package vehicle holds the deterministic per-vehicle state fold.
package vehicle

import (
	"time"

	"google.golang.org/protobuf/proto"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// heartbeatTTLMs expresses the codec timeout in the fold's unit.
const heartbeatTTLMs = int64(codec.HeartbeatTTL / time.Millisecond)

// EKF status bits that gate arming. Values from MAVLink's EKF_STATUS_FLAGS.
const (
	// EkfAttitude indicates a usable attitude estimate.
	EkfAttitude uint32 = 1 << 0
	// EkfUninitialized must be clear: the filter has not converged.
	EkfUninitialized uint32 = 1 << 10
)

// State is the accumulated model of one vehicle. Fold clones reference fields
// and never mutates its input.
type State struct {
	// Snapshot is the canonical serializable view.
	Snapshot *gcsv1.VehicleSnapshot

	// LastMsgSeenMs supports per-message-family freshness.
	LastMsgSeenMs map[uint32]int64

	// LastSrcAddr is diagnostic input for source-conflict detection.
	LastSrcAddr string

	// LastSeenMs is the last time any message arrived from this vehicle. This
	// is what the heartbeat TTL measures against.
	LastSeenMs int64
	// LastHeartbeatMs records HEARTBEAT separately from general traffic.
	LastHeartbeatMs int64
	// FirstSeenMs is when this vehicle was discovered.
	FirstSeenMs int64
	// CustomMode is HEARTBEAT.custom_mode: the firmware-specific flight mode.
	CustomMode uint32
	// EkfFlags is the last EKF_STATUS_REPORT.flags bitmask.
	EkfFlags uint32
	// SourceConflicts counts source-address transitions.
	SourceConflicts uint32

	// VehicleType is fixed from the first HEARTBEAT.
	VehicleType gcsv1.MavType
	// SystemStatus is MAV_STATE from the most recent HEARTBEAT.
	SystemStatus gcsv1.MavState

	SysID  uint8
	CompID uint8

	// Armed is base_mode's MAV_MODE_FLAG_SAFETY_ARMED predicate.
	Armed bool
	// EkfSeen distinguishes zero flags from no report.
	EkfSeen bool
	// Known is set by the first HEARTBEAT.
	Known bool
	// Lost suppresses duplicate loss events within one outage.
	Lost bool
}

// ID returns this vehicle's identity as the proto envelopes carry it.
func (s *State) ID() *gcsv1.VehicleId {
	return &gcsv1.VehicleId{
		SystemId:    uint32(s.SysID),
		ComponentId: uint32(s.CompID),
	}
}

// AgeMs reports how long it has been since any message from this vehicle.
func (s *State) AgeMs(nowMs int64) int64 {
	return nowMs - s.LastSeenMs
}

// FamilyAgeMs reports how long it has been since a specific MAVLink message ID
// was received, and whether it has ever been received at all.
func (s *State) FamilyAgeMs(msgID uint32, nowMs int64) (int64, bool) {
	seen, ok := s.LastMsgSeenMs[msgID]
	if !ok {
		return 0, false
	}

	return nowMs - seen, true
}

// Expired reports whether a known vehicle exceeded HeartbeatTTL.
func (s *State) Expired(nowMs int64) bool {
	return s.Known && nowMs-s.LastSeenMs > heartbeatTTLMs
}

// ArmAllowed reports whether local EKF evidence permits offering arm.
func (s *State) ArmAllowed() bool {
	return s.ArmBlockedReason() == ""
}

// ArmBlockedReason names the local arming gate, or returns "".
func (s *State) ArmBlockedReason() string {
	switch {
	case !s.Known:
		return "no heartbeat received"
	case !s.EkfSeen:
		return "no EKF status received"
	case s.EkfFlags&EkfUninitialized != 0:
		return "EKF not initialised"
	case s.EkfFlags&EkfAttitude == 0:
		return "EKF has no attitude solution"
	default:
		return ""
	}
}

// cloneSnapshot returns a snapshot the fold may write to without touching the
// caller's copy, creating one if this vehicle has none yet.
func (s *State) cloneSnapshot() *gcsv1.VehicleSnapshot {
	if s.Snapshot == nil {
		return &gcsv1.VehicleSnapshot{Id: s.ID()}
	}

	//nolint:forcetypeassert // proto.Clone returns the concrete type it was given.
	return proto.Clone(s.Snapshot).(*gcsv1.VehicleSnapshot)
}
