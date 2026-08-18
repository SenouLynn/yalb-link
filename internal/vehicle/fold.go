package vehicle

import (
	"maps"

	"google.golang.org/protobuf/proto"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// Inbound is one decoded frame with the transport facts the fold needs.
//
// The Tier 4 plan called this `*codec.DecodedMessage` carrying a source
// address. The codec deliberately does not know about source addresses — it
// decodes bytes and nothing else — so the transport fact is attached here, at
// the layer that has both. Heartbeat is separate from Decoded for the same
// reason it is separate in the codec: HEARTBEAT has no TelemetryEvent variant,
// it drives the fleet lifecycle.
type Inbound struct {
	// Heartbeat is codec.DecodeHeartbeat's result, non-nil only for HEARTBEAT.
	Heartbeat *gcsv1.HeartbeatState
	// SrcAddr is "<ip>:<port>" the packet arrived from, or "" when the
	// transport cannot attribute one. Diagnostics and conflict detection only.
	SrcAddr string
	// Decoded is the codec's envelope for this frame.
	Decoded codec.Decoded
	// MsgID is the MAVLink message ID, kept for per-family freshness. Decoded
	// does not carry it: an envelope that resolved to a payload has already
	// answered "which family", and one that resolved to nothing still needs
	// its arrival recorded.
	MsgID uint32
}

// Fold advances vehicle state by one message and reports what happened.
//
// It is pure: given the same state, message and clock it returns the same
// result, and the state passed in is never modified. Time arrives as nowMs
// because every interesting property here — discovery, loss, recovery,
// freshness — is a statement about time, and a fold that reads the wall clock
// can only be tested by waiting.
//
// State is passed and returned by value even though it is 96 bytes. A fold
// that took a pointer could not promise the caller's state is unchanged, and
// that promise is what makes replay and table-driven tests possible.
//
//nolint:gocritic // hugeParam: the copy is the contract, see above.
func Fold(state State, in Inbound, nowMs int64) (State, []Event) {
	next := state
	next.SysID = in.Decoded.SysID
	next.CompID = in.Decoded.CompID
	next.Snapshot = state.cloneSnapshot()
	next.Snapshot.Id = next.ID()
	next.LastMsgSeenMs = cloneFamilies(state.LastMsgSeenMs)

	events := make([]Event, 0, 3)
	events = append(events, lifecycle(&state, &next, in.Heartbeat, nowMs)...)
	events = append(events, sourceConflict(&state, &next, in.SrcAddr, nowMs)...)
	events = append(events, telemetry(&next, in.Decoded.Telemetry)...)

	// Liveness is measured from any traffic, not from HEARTBEAT alone, and is
	// updated after the lifecycle check reads the gap that just ended.
	next.LastSeenMs = nowMs
	next.LastMsgSeenMs[in.MsgID] = nowMs
	next.Snapshot.LastUpdatedAt = timestampOf(nowMs)

	return next, events
}

// Expire advances a vehicle that has received nothing.
//
// Fold only runs when a message arrives, so a vehicle that goes silent for
// good would never be declared lost. A caller sweeping known vehicles on a
// tick calls this; it emits VEHICLE_LOST at most once per outage.
//
//nolint:gocritic // hugeParam: the copy is the contract, as in Fold.
func Expire(state State, nowMs int64) (State, []Event) {
	if state.Lost || !state.Expired(nowMs) {
		return state, nil
	}

	next := state
	next.Lost = true

	return next, []Event{fleetEvent(
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST,
		state.ID(), state.Snapshot.GetHeartbeat(), nowMs,
	)}
}

// lifecycle emits the fleet events for this message and applies the heartbeat.
func lifecycle(prev, next *State, hb *gcsv1.HeartbeatState, nowMs int64) []Event {
	id := next.ID()

	var events []Event

	// The outage is detected from the gap that just ended, before LastSeenMs
	// moves. A message arriving after a two-minute silence reports both the
	// loss and the recovery: swallowing the loss because the vehicle came back
	// leaves the fleet stream claiming continuous contact that did not happen.
	if prev.Expired(nowMs) && !prev.Lost {
		events = append(events, fleetEvent(
			gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST,
			id, prev.Snapshot.GetHeartbeat(), nowMs,
		))

		next.Lost = true
	}

	if hb == nil {
		// Discovery and recovery both require a HEARTBEAT: identity, type and
		// armed state come from it, and announcing a vehicle we cannot yet
		// describe is worse than waiting for the next one at 1 Hz.
		return events
	}

	//nolint:forcetypeassert // proto.Clone returns the concrete type it was given.
	stamped := proto.Clone(hb).(*gcsv1.HeartbeatState)
	stamped.Id = id
	stamped.ObservedAt = timestampOf(nowMs)

	switch {
	case !prev.Known:
		next.Known = true
		next.FirstSeenMs = nowMs
		// MAV_TYPE is taken once. An airframe does not change type in flight,
		// and a glitched frame that says it did must not re-shape the
		// trajectory model downstream.
		next.VehicleType = stamped.GetType()

		events = append(events, fleetEvent(
			gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, id, stamped, nowMs))

	case next.Lost:
		next.Lost = false

		events = append(events, fleetEvent(
			gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_RECOVERED, id, stamped, nowMs))

	case heartbeatChanged(prev, stamped):
		events = append(events, fleetEvent(
			gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED, id, stamped, nowMs))
	}

	next.Armed = stamped.GetArmed()
	next.CustomMode = stamped.GetCustomMode()
	next.SystemStatus = stamped.GetSystemStatus()
	next.LastHeartbeatMs = nowMs
	next.Snapshot.Heartbeat = stamped

	return events
}

// heartbeatChanged reports whether this heartbeat says anything new.
//
// HEARTBEAT arrives at 1 Hz per vehicle and is identical almost every time.
// Emitting HEARTBEAT_UPDATED unconditionally would put one event per vehicle
// per second on the fleet stream to say nothing changed.
func heartbeatChanged(prev *State, hb *gcsv1.HeartbeatState) bool {
	return hb.GetArmed() != prev.Armed ||
		hb.GetCustomMode() != prev.CustomMode ||
		hb.GetSystemStatus() != prev.SystemStatus
}

// sourceConflict detects a system ID arriving from a new source address.
//
// One warning per transition, not one per packet and not one per lifetime: a
// vehicle flapping between two sources is a different fault from a vehicle
// that moved once, and SourceConflicts counts the difference.
func sourceConflict(prev, next *State, srcAddr string, nowMs int64) []Event {
	if srcAddr == "" || srcAddr == prev.LastSrcAddr {
		return nil
	}

	next.LastSrcAddr = srcAddr

	if prev.LastSrcAddr == "" {
		return nil // first attribution, not a conflict
	}

	next.SourceConflicts = prev.SourceConflicts + 1

	return []Event{{Warning: &Warning{
		VehicleID:  next.ID(),
		Previous:   prev.LastSrcAddr,
		Current:    srcAddr,
		OccurredMs: nowMs,
		Type:       WarningSourceConflict,
	}}}
}

// telemetry folds a telemetry payload into the snapshot and passes it through.
func telemetry(next *State, evt *gcsv1.TelemetryEvent) []Event {
	if evt == nil {
		return nil
	}

	if ekf := evt.GetEkfStatusReport(); ekf != nil {
		next.EkfFlags = ekf.GetFlags()
		next.EkfSeen = true
	}

	applySnapshot(next.Snapshot, evt)

	// Transactions (PARAM_VALUE, MISSION_*, COMMAND_ACK) are deliberately not
	// folded. They correlate against an in-flight request registry, which is
	// Tier 7's problem; routing them through vehicle state would make a
	// parameter read look like telemetry.
	return []Event{{Telemetry: evt}}
}

// applySnapshot writes the aggregate fields a payload owns.
//
// Only the four families the VehicleSnapshot contract names as their source
// are aggregated. GPS_RAW_INT also carries a position and BATTERY_STATUS also
// carries a voltage, but the snapshot's fields document exactly one origin
// each; a second writer makes "which message produced this number" unanswerable
// and merges MSL altitude with above-home altitude.
func applySnapshot(snap *gcsv1.VehicleSnapshot, evt *gcsv1.TelemetryEvent) {
	if a := evt.GetAttitude(); a != nil {
		snap.RollRad, snap.PitchRad, snap.YawRad = a.GetRollRad(), a.GetPitchRad(), a.GetYawRad()

		return
	}

	if p := evt.GetGlobalPosition(); p != nil {
		snap.LatitudeDeg, snap.LongitudeDeg = p.GetLatDeg(), p.GetLonDeg()
		snap.AltitudeMslM, snap.AltitudeRelativeM = p.GetAltMslM(), p.GetAltRelativeM()

		return
	}

	if v := evt.GetVfrHud(); v != nil {
		snap.AirspeedMS, snap.GroundspeedMS = v.GetAirspeedMS(), v.GetGroundspeedMS()
		// VFR_HUD's climb rate is already positive-up; the NED sign flip
		// applies to GLOBAL_POSITION_INT's vz and must not be applied twice.
		snap.ClimbRateMS = v.GetClimbMS()
		snap.HeadingDeg = normaliseHeading(v.GetHeadingDeg())

		return
	}

	if s := evt.GetSystemStatus(); s != nil {
		snap.VoltageBatteryMv = s.GetVoltageBatteryMv()
		snap.CurrentBatteryCa = s.GetCurrentBatteryCa()
		snap.BatteryRemainingPct = s.GetBatteryRemainingPct()
	}
}

// normaliseHeading maps VFR_HUD's signed heading onto 0-359.
//
// ArduPilot reports negative headings on this field even though MAVLink
// documents 0-360, and VehicleSnapshot.heading_deg promises 0-359. The
// conversion happens here, once.
func normaliseHeading(deg int32) int32 {
	return ((deg % 360) + 360) % 360
}

// cloneFamilies copies the freshness map so the returned state does not share
// it with the state that was passed in.
func cloneFamilies(m map[uint32]int64) map[uint32]int64 {
	if m == nil {
		return make(map[uint32]int64, 8)
	}

	return maps.Clone(m)
}
