package codec

import (
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
)

// ardupilotmega aliases common message structs but defines distinct enum types.
// Encoders return messages; gomavlib owns framing and link selection.

// Target identifies the vehicle a command is addressed to, in MAVLink terms.
type Target struct {
	SystemID    uint8
	ComponentID uint8
}

// SendFamilies is the tested set of encoded MAVLink message IDs.
var SendFamilies = []uint32{
	0,  // HEARTBEAT
	20, // PARAM_REQUEST_READ
	21, // PARAM_REQUEST_LIST
	23, // PARAM_SET
	43, // MISSION_REQUEST_LIST
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

// CmdSetMessageInterval is MAV_CMD_SET_MESSAGE_INTERVAL.
const CmdSetMessageInterval = 511

// IntervalDefaultUs restores a message to the autopilot's own default rate.
const IntervalDefaultUs int32 = 0

// IntervalDisabledUs stops a message being streamed.
const IntervalDisabledUs int32 = -1

// ForceArmMagic bypasses autopilot preflight checks and must be rejected by
// command policy above the codec.
const ForceArmMagic = 21196

// PositionOnlyTypeMask ignores velocity, acceleration, yaw, and yaw rate.
const PositionOnlyTypeMask = 0xDF8

// Mission types name MAV_MISSION_TYPE wire values. They are the discriminator
// that keeps a flight-plan transfer from settling a geofence or rally transfer
// sharing the same link and vehicle.
const (
	MissionTypeMission uint32 = 0
	MissionTypeFence   uint32 = 1
	MissionTypeRally   uint32 = 2
)

// FrameGlobalRelativeAltInt is MAV_FRAME_GLOBAL_RELATIVE_ALT_INT — lat/lon in
// degE7, altitude in metres above home.
const FrameGlobalRelativeAltInt = 6

// EncodeHeartbeat builds a GCS heartbeat; Node emits its periodic heartbeat.
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

// EncodeCommandLong builds an unvalidated COMMAND_LONG payload.
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

// EncodeSetMessageInterval requests one message family at a fixed period.
//
// intervalUs is microseconds between messages, not a frequency: that is the
// wire unit, and converting here would put a rounding step between the caller's
// stated rate and what the autopilot is actually told. IntervalDefaultUs and
// IntervalDisabledUs carry the two sentinel meanings.
//
// param7 (response target) stays 0, meaning the flight stack's default: the
// COMMAND_ACK returns over the link the request arrived on, which is the link
// the bridge is already reading.
func EncodeSetMessageInterval(target Target, msgID uint32, intervalUs int32) message.Message {
	return EncodeCommandLong(target, CmdSetMessageInterval, 0, [7]float32{
		float32(msgID),
		float32(intervalUs),
		0, 0, 0, 0, 0,
	})
}

// EncodeSetPositionTargetGlobalInt converts degrees to degE7; altitude remains
// metres above home.
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

// EncodeMissionItemInt converts x/y degrees to degE7; z remains metres.
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

// EncodeMissionRequestList opens a mission download by asking the vehicle to
// report how many items it holds.
//
// This is the only message that begins a download. EncodeMissionRequestInt
// continues one that is already open, so without this encoder the codec can
// only join a transfer another ground station started.
func EncodeMissionRequestList(target Target, missionType uint32) message.Message {
	return &ardupilotmega.MessageMissionRequestList{
		TargetSystem:    target.SystemID,
		TargetComponent: target.ComponentID,
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

// EncodeMissionClearAll builds an unauthorised MISSION_CLEAR_ALL payload.
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
