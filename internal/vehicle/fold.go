package vehicle

import (
	"maps"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// Inbound combines a decoded frame with fold-specific transport facts.
type Inbound struct {
	// Heartbeat is codec.DecodeHeartbeat's result, non-nil only for HEARTBEAT.
	Heartbeat *gcsv1.HeartbeatState
	// SrcAddr is "<ip>:<port>" the packet arrived from, or "" when the
	// transport cannot attribute one. Diagnostics and conflict detection only.
	SrcAddr string
	// Decoded is the codec's envelope for this frame.
	Decoded codec.Decoded
	// MsgID supports per-family freshness, including unhandled payloads.
	MsgID uint32
}

// Fold advances state without mutating its input. nowMs is injected so replay,
// liveness, and freshness remain deterministic.
//
//nolint:gocritic // hugeParam: the copy is the contract, see above.
func Fold(state State, in Inbound, nowMs int64) (State, []Event) {
	next := state
	next.SysID = in.Decoded.SysID
	next.CompID = in.Decoded.CompID
	next.Snapshot = state.cloneSnapshot()
	next.Snapshot.Id = next.ID()
	next.LastMsgSeenMs = cloneFamilies(state.LastMsgSeenMs)

	events := make([]Event, 0, 4)
	events = append(events, lifecycle(&state, &next, in.Heartbeat, nowMs)...)
	events = append(events, sourceConflict(&state, &next, in.SrcAddr, nowMs)...)
	events = append(events, telemetry(&next, in.Decoded.Telemetry, nowMs)...)
	events = append(events, protocol(in.Decoded.Transaction)...)

	// Liveness is measured from any traffic, not from HEARTBEAT alone, and is
	// updated after the lifecycle check reads the gap that just ended.
	next.LastSeenMs = nowMs
	next.LastMsgSeenMs[in.MsgID] = nowMs
	next.Snapshot.LastUpdatedAt = timestampOf(nowMs)

	return next, events
}

// Expire emits VEHICLE_LOST once for a silent vehicle.
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

	// Detect the completed outage before LastSeenMs advances.
	if prev.Expired(nowMs) && !prev.Lost {
		events = append(events, fleetEvent(
			gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST,
			id, prev.Snapshot.GetHeartbeat(), nowMs,
		))

		next.Lost = true
	}

	if hb == nil {
		// Discovery and recovery require HEARTBEAT identity.
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

// heartbeatChanged reports an operator-visible heartbeat change.
func heartbeatChanged(prev *State, hb *gcsv1.HeartbeatState) bool {
	return hb.GetArmed() != prev.Armed ||
		hb.GetCustomMode() != prev.CustomMode ||
		hb.GetSystemStatus() != prev.SystemStatus
}

// sourceConflict emits one warning per source-address transition.
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
//
// The emitted event is a stamped copy. The fold does not own the decoded event
// the caller handed it, and downstream sinks retain what they are given, so
// writing observed_at into the input would both break the no-mutation contract
// and hand every subscriber a pointer the next frame could still change.
func telemetry(next *State, evt *gcsv1.TelemetryEvent, nowMs int64) []Event {
	if evt == nil {
		return nil
	}

	//nolint:forcetypeassert // proto.Clone returns the concrete type it was given.
	stamped := proto.Clone(evt).(*gcsv1.TelemetryEvent)
	stampObservedAt(stamped, nowMs)

	if ekf := stamped.GetEkfStatusReport(); ekf != nil {
		next.EkfFlags = ekf.GetFlags()
		next.EkfSeen = true
	}

	applySnapshot(next.Snapshot, stamped)

	return []Event{{Telemetry: stamped}}
}

// Field and oneof names the stamping walk resolves by descriptor.
const (
	telemetryPayloadOneof = "payload"
	observedAtField       = "observed_at"
)

// stampObservedAt records when the fold observed this payload.
//
// Resolved through the descriptor rather than a type switch over the oneof: the
// timestamp is the operator's evidence that a reading is current, and a switch
// silently omits whichever family someone adds next. Every payload in
// telemetry.proto declares observed_at, and a payload that stopped doing so
// would be a contract change, not something to paper over here.
func stampObservedAt(evt *gcsv1.TelemetryEvent, nowMs int64) {
	msg := evt.ProtoReflect()

	field := msg.WhichOneof(msg.Descriptor().Oneofs().ByName(telemetryPayloadOneof))
	if field == nil {
		return
	}

	payload := msg.Mutable(field).Message()

	stamp := payload.Descriptor().Fields().ByName(observedAtField)
	if stamp == nil {
		return
	}

	payload.Set(stamp, protoreflect.ValueOfMessage(timestampOf(nowMs).ProtoReflect()))
}

// protocol passes a decoded transaction response through to the sinks.
//
// The fold keeps no transaction state — there is no request registry yet — but
// dropping these here would make a decoded COMMAND_ACK unobservable anywhere
// above the codec.
func protocol(evt *gcsv1.ProtocolEvent) []Event {
	if evt == nil {
		return nil
	}

	return []Event{{Protocol: evt}}
}

// applySnapshot updates fields with a single documented message source.
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
		// VFR_HUD climb is already positive-up.
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
