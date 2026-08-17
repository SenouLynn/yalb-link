package codec

import (
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
)

// Both dialect packages are imported because ardupilotmega re-exports the
// common *message structs* as type aliases but declares its own *enum types*.
// ardupilotmega.MAV_CMD and common.MAV_CMD are therefore distinct types over
// the same uint64, and a field on an aliased struct wants the common one. The
// compiler catches this, but the asymmetry is surprising enough to write down.

// Encoders for the 11 priority send families.
//
// Two constraints shape every function here.
//
// **Encoders are pure.** They return message.Message values, never bytes.
// gomavlib owns CRC_EXTRA and framing at write time. Never compute CRC_EXTRA
// by hand — a hand-rolled table is a silent wire incompatibility the moment
// the dialect moves.
//
// **Encoders do not choose a destination.** No function here takes a link.
// Addressing is the transport port's job (Node.WriteTo), which keeps the
// never-broadcast rule in exactly one place. target_system and
// target_component are MAVLink payload fields identifying the intended
// recipient; they are not the same thing as which radio the frame goes out on,
// and conflating the two is how a command for vehicle 2 ends up transmitted
// over vehicle 1's link.

// Target identifies the vehicle a command is addressed to, in MAVLink terms.
type Target struct {
	SystemID    uint8
	ComponentID uint8
}

// SendFamilies are the MAVLink message IDs this package can encode.
//
// This is the encode-side counterpart to the telemetryDecoders and
// protocolDecoders keys, and TestMatrixCoverage asserts the capability matrix's
// SEND-* rows equal it. Declared here rather than in a test so the matrix is
// checked against the package's own statement of what it encodes.
//
// TestSendFamilyCoverage separately builds one message per entry and asserts
// its GetID matches, which is what stops this list from being a wish: an ID
// here with no encoder behind it fails, as does an encoder that builds the
// wrong ID. Adding an encoder without adding its ID here is the one drift this
// cannot catch on its own — the matrix gate is the backstop for that.
var SendFamilies = []uint32{
	0,  // HEARTBEAT
	20, // PARAM_REQUEST_READ
	21, // PARAM_REQUEST_LIST
	23, // PARAM_SET
	44, // MISSION_COUNT
	45, // MISSION_CLEAR_ALL
	47, // MISSION_ACK
	51, // MISSION_REQUEST_INT
	73, // MISSION_ITEM_INT
	76, // COMMAND_LONG
	86, // SET_POSITION_TARGET_GLOBAL_INT
}

// CmdComponentArmDisarm is MAV_CMD_COMPONENT_ARM_DISARM.
const CmdComponentArmDisarm = 400

// ForceArmMagic is the param2 value that turns arm into force-arm, bypassing
// the autopilot's own preflight checks.
//
// Named here so the Tier 8 command registry can reject it by reference rather
// than by a retyped literal. Removing SetArmedRequest.force from the proto did
// not close this path: SendCommand accepts any MavCmd plus a raw_command
// passthrough, so force-arm remains reachable one RPC over and the control has
// to be server-side validation.
const ForceArmMagic = 21196

// PositionOnlyTypeMask is the type_mask for a position-only
// SET_POSITION_TARGET_GLOBAL_INT.
//
// 0xDF8 = 3576 = 0b110111111000: velocity, acceleration, yaw and yaw-rate bits
// all set (meaning "ignore"), position bits clear (meaning "use"). The
// FORCE_SET bit (9) is deliberately not set.
const PositionOnlyTypeMask = 0xDF8

// FrameGlobalRelativeAltInt is MAV_FRAME_GLOBAL_RELATIVE_ALT_INT — lat/lon in
// degE7, altitude in metres above home.
const FrameGlobalRelativeAltInt = 6

// EncodeHeartbeat builds the GCS keepalive.
//
// Present for completeness and for the capability matrix. In practice the node
// emits its own heartbeat from NodeConf — see HeartbeatPeriod in frame.go.
// Calling this on a timer as well would double-emit, which on a 57.6 kbps SiK
// link is bandwidth spent in the scarce direction for nothing.
func EncodeHeartbeat() message.Message {
	return &ardupilotmega.MessageHeartbeat{
		Type:           6, // MAV_TYPE_GCS
		Autopilot:      0, // MAV_AUTOPILOT_GENERIC
		BaseMode:       0,
		CustomMode:     0,
		SystemStatus:   4, // MAV_STATE_ACTIVE
		MavlinkVersion: 3,
	}
}

// EncodeCommandLong builds a COMMAND_LONG.
//
// The command ID and params are passed through unvalidated — this is a codec,
// not a policy layer. The allowlist that rejects force-arm and anything not
// explicitly permitted lives in the Tier 8 command registry, above this
// package, so that one check covers both the typed RPCs and the raw_command
// passthrough.
func EncodeCommandLong(
	target Target,
	command uint32,
	confirmation uint8,
	params [7]float32,
) message.Message {
	return &ardupilotmega.MessageCommandLong{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		Command:         common.MAV_CMD(command),
		Confirmation:    confirmation,
		Param1:          params[0],
		Param2:          params[1],
		Param3:          params[2],
		Param4:          params[3],
		Param5:          params[4],
		Param6:          params[5],
		Param7:          params[6],
	}
}

// EncodeSetPositionTargetGlobalInt builds a guided-mode reposition.
//
// Takes degrees and metres and converts to the wire's degE7 at this boundary,
// so no caller handles E7 coordinates. Altitude is metres above home, matching
// MAV_FRAME_GLOBAL_RELATIVE_ALT_INT.
func EncodeSetPositionTargetGlobalInt(
	target Target,
	latDeg, lonDeg float64,
	altRelativeM float32,
) message.Message {
	return &ardupilotmega.MessageSetPositionTargetGlobalInt{
		TimeBootMs:      0,
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		CoordinateFrame: FrameGlobalRelativeAltInt,
		TypeMask:        PositionOnlyTypeMask,
		LatInt:          int32(latDeg / degE7ToDeg),
		LonInt:          int32(lonDeg / degE7ToDeg),
		Alt:             altRelativeM,
	}
}

// EncodeParamSet builds a PARAM_SET.
func EncodeParamSet(
	target Target,
	paramID string,
	value float32,
	paramType uint32,
) message.Message {
	return &ardupilotmega.MessageParamSet{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		ParamId:         paramID,
		ParamValue:      value,
		ParamType:       ardupilotmega.MAV_PARAM_TYPE(paramType),
	}
}

// EncodeParamRequestList requests every parameter from a vehicle.
func EncodeParamRequestList(target Target) message.Message {
	return &ardupilotmega.MessageParamRequestList{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
	}
}

// EncodeParamRequestRead requests one parameter.
//
// paramIndex -1 means "look it up by name"; any valid index makes the
// autopilot ignore paramID entirely.
func EncodeParamRequestRead(
	target Target,
	paramID string,
	paramIndex int16,
) message.Message {
	return &ardupilotmega.MessageParamRequestRead{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		ParamId:         paramID,
		ParamIndex:      paramIndex,
	}
}

// EncodeMissionCount opens a mission upload by declaring the item count.
func EncodeMissionCount(
	target Target,
	count uint16,
	missionType uint32,
) message.Message {
	return &ardupilotmega.MessageMissionCount{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		Count:           count,
		MissionType:     ardupilotmega.MAV_MISSION_TYPE(missionType),
	}
}

// EncodeMissionItemInt builds one mission item.
//
// Takes degrees and converts to degE7 here. Note z stays float metres — only
// x and y scale, which is the asymmetry that makes a hand-written conversion
// at a call site error-prone.
func EncodeMissionItemInt(
	target Target,
	seq uint16,
	frame uint32,
	command uint32,
	current, autocontinue bool,
	params [4]float32,
	latDeg, lonDeg float64,
	altM float32,
	missionType uint32,
) message.Message {
	return &ardupilotmega.MessageMissionItemInt{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		Seq:             seq,
		Frame:           ardupilotmega.MAV_FRAME(frame),
		Command:         common.MAV_CMD(command),
		Current:         boolToU8(current),
		Autocontinue:    boolToU8(autocontinue),
		Param1:          params[0],
		Param2:          params[1],
		Param3:          params[2],
		Param4:          params[3],
		X:               int32(latDeg / degE7ToDeg),
		Y:               int32(lonDeg / degE7ToDeg),
		Z:               altM,
		MissionType:     ardupilotmega.MAV_MISSION_TYPE(missionType),
	}
}

// EncodeMissionRequestInt asks for one mission item by sequence number.
func EncodeMissionRequestInt(
	target Target,
	seq uint16,
	missionType uint32,
) message.Message {
	return &ardupilotmega.MessageMissionRequestInt{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		Seq:             seq,
		MissionType:     ardupilotmega.MAV_MISSION_TYPE(missionType),
	}
}

// EncodeMissionAck acknowledges a mission transfer.
func EncodeMissionAck(
	target Target,
	result uint32,
	missionType uint32,
) message.Message {
	return &ardupilotmega.MessageMissionAck{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		Type:            ardupilotmega.MAV_MISSION_RESULT(result),
		MissionType:     ardupilotmega.MAV_MISSION_TYPE(missionType),
	}
}

// EncodeMissionClearAll erases a vehicle's mission.
//
// Wired in Tier 8 and deliberately not surfaced in the mission panel until
// after Tier 10 — the protocol being available is not the same as the button
// existing.
func EncodeMissionClearAll(target Target, missionType uint32) message.Message {
	return &ardupilotmega.MessageMissionClearAll{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
		MissionType:     ardupilotmega.MAV_MISSION_TYPE(missionType),
	}
}

func boolToU8(b bool) uint8 {
	if b {
		return 1
	}

	return 0
}
