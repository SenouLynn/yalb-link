// Package vehicle holds the GCS-side model of a single vehicle: the state it
// accumulates and the pure fold that advances it.
//
// Nothing here opens a socket, talks to Redis or reads a clock. State advances
// only through Fold and Expire, both of which take the current time in
// milliseconds as a parameter. That is the whole design: liveness, TTL expiry
// and freshness are the properties most likely to be wrong, and they are only
// testable when the clock is an input. `time.Now()` must never appear in this
// package — TestNoWallClock enforces it against the source.
package vehicle

import (
	"time"

	"google.golang.org/protobuf/proto"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// heartbeatTTLMs is codec.HeartbeatTTL in the unit the fold works in.
//
// The duration lives in codec next to HeartbeatPeriod and LinkIdleTimeout so
// the three link-timing values cannot drift apart; this is a restatement of
// one of them, not a second source of truth.
const heartbeatTTLMs = int64(codec.HeartbeatTTL / time.Millisecond)

// expireThrottleMs bounds how often a caller refreshes the Redis TTL on a
// vehicle key. At 50 Hz telemetry an unthrottled EXPIRE per message is 50
// round trips per second per vehicle to restate a value that has not changed.
const expireThrottleMs int64 = 30_000

// EKF status bits that gate arming. Values from MAVLink's EKF_STATUS_FLAGS.
const (
	// EkfAttitude must be set: without an attitude solution the vehicle has no
	// usable estimate of which way is up.
	EkfAttitude uint32 = 1 << 0
	// EkfUninitialized must be clear: the filter has not converged.
	EkfUninitialized uint32 = 1 << 10
)

// State is the accumulated model of one vehicle.
//
// It is a value type and every field is copied by assignment except Snapshot
// and LastMsgSeenMs, which Fold clones before touching. A State handed to Fold
// is never mutated: the caller can keep the old value, replay the same message
// against it, and get the same answer. That property is what makes the fold
// testable without a running system, and it is worth the two clones.
//
// Field order is pointers first, then size-descending, which is what govet's
// fieldalignment asks for. Methods take a pointer receiver for the same
// reason: the struct is 96 bytes and read-only accessors have no business
// copying it. The value semantics that matter are Fold's, not the getters'.
type State struct {
	// Snapshot is the canonical serializable view of this vehicle: the thing
	// ListVehicles returns and the UI renders. Fold replaces it with a clone
	// on every call rather than mutating in place.
	Snapshot *gcsv1.VehicleSnapshot

	// LastMsgSeenMs maps a MAVLink message ID to when it was last received.
	// Per-family freshness — an attitude indicator that has not had an
	// ATTITUDE in four seconds must be able to say so without guessing from a
	// single vehicle-wide timestamp.
	LastMsgSeenMs map[uint32]int64

	// LastSrcAddr is "<ip>:<port>" of the last packet attributed to this
	// vehicle. Diagnostics only — it is never a send address. See
	// routes.Table for how a reply is actually addressed.
	LastSrcAddr string

	// LastSeenMs is the last time any message arrived from this vehicle. This
	// is what the heartbeat TTL measures against.
	LastSeenMs int64
	// LastHeartbeatMs is the last HEARTBEAT specifically. Telemetry can keep
	// flowing from a vehicle whose heartbeat has stopped; the two questions
	// have different answers and so get different fields.
	LastHeartbeatMs int64
	// FirstSeenMs is when this vehicle was discovered.
	FirstSeenMs int64
	// LastExpireMs is when the caller last refreshed this vehicle's Redis TTL.
	// Read through ShouldRefreshTTL / MarkTTLRefreshed.
	LastExpireMs int64

	// CustomMode is HEARTBEAT.custom_mode: the firmware-specific flight mode.
	CustomMode uint32
	// EkfFlags is the last EKF_STATUS_REPORT.flags bitmask.
	EkfFlags uint32
	// SourceConflicts counts how many times this vehicle's source address has
	// changed. A vehicle that flaps between two sources reports a rising
	// count, which a single boolean would hide.
	SourceConflicts uint32

	// VehicleType is MAV_TYPE from the first HEARTBEAT. It selects the
	// trajectory model downstream and is not updated afterwards — an airframe
	// does not change type mid-flight, and a decoder glitch that says it did
	// must not silently re-shape the prediction.
	VehicleType gcsv1.MavType
	// SystemStatus is MAV_STATE from the most recent HEARTBEAT.
	SystemStatus gcsv1.MavState

	SysID  uint8
	CompID uint8

	// Armed is base_mode's MAV_MODE_FLAG_SAFETY_ARMED bit, expanded by the
	// codec. The predicate is stored, never the bitmask: re-deriving the mask
	// at each call site is where an inverted arm indicator comes from.
	Armed bool
	// EkfSeen records whether any EKF_STATUS_REPORT has arrived. Without it,
	// zero flags and "no report yet" are indistinguishable, and the arm gate
	// cannot tell "EKF says no" from "EKF has not spoken".
	EkfSeen bool
	// Known is set by the first HEARTBEAT. Telemetry alone does not make a
	// vehicle known: identity, type and armed state all come from HEARTBEAT,
	// and announcing a vehicle we cannot describe is worse than waiting a
	// second for the next one.
	Known bool
	// Lost is set when the heartbeat TTL has been exceeded and cleared by the
	// next message. It exists so VEHICLE_LOST is emitted once per outage
	// rather than once per late message.
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

// Expired reports whether the heartbeat TTL has elapsed for a known vehicle.
//
// This is the authoritative liveness question. The transport's own idle
// timeout is deliberately set longer (codec.LinkIdleTimeout) so channel
// teardown never front-runs the answer given here.
func (s *State) Expired(nowMs int64) bool {
	return s.Known && nowMs-s.LastSeenMs > heartbeatTTLMs
}

// ShouldRefreshTTL reports whether enough time has passed to justify another
// Redis EXPIRE for this vehicle. The caller performs the write and then calls
// MarkTTLRefreshed; splitting the decision from the effect keeps the throttle
// in the pure layer where it can be tested, and keeps the two from disagreeing
// about whether a refresh actually happened.
func (s *State) ShouldRefreshTTL(nowMs int64) bool {
	return nowMs-s.LastExpireMs > expireThrottleMs
}

// MarkTTLRefreshed records that the caller has refreshed the TTL.
func (s *State) MarkTTLRefreshed(nowMs int64) {
	s.LastExpireMs = nowMs
}

// ArmAllowed reports whether the EKF is in a state where arming may be
// offered. It is a gate, not a permission: the vehicle runs its own arming
// checks and is the final authority.
func (s *State) ArmAllowed() bool {
	return s.ArmBlockedReason() == ""
}

// ArmBlockedReason names why arming is gated, or returns "" when it is not.
//
// The reason is returned rather than a bare bool because a disabled arm button
// with no explanation is the thing operators file bugs about.
func (s *State) ArmBlockedReason() string {
	switch {
	case !s.Known:
		return "no heartbeat received"
	case !s.EkfSeen:
		// Fail closed. ArduPilot streams EKF_STATUS_REPORT on every vehicle
		// type we target, so silence here means the link is incomplete, not
		// that the estimator is fine.
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
