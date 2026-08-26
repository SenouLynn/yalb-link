package codec

import (
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// Decoded contains at most one streaming or transaction envelope.
type Decoded struct {
	Telemetry   *gcsv1.TelemetryEvent
	Transaction *gcsv1.ProtocolEvent
	SysID       uint8
	CompID      uint8
	Seq         uint8
}

// Handled reports whether the frame produced an envelope.
func (d Decoded) Handled() bool {
	return d.Telemetry != nil || d.Transaction != nil
}

// Unit conversions are applied once at the protobuf boundary.
const (
	degE7ToDeg  = 1e-7
	mmToM       = 1e-3
	cmPerSToMPS = 1e-2
	cGToG       = 1e-2 // centi-units to whole units (load, throttle percentages)
)

// telemetryDecoders is the mechanically checked streaming dispatch table.
// A nil value claims a non-telemetry family such as HEARTBEAT.
var telemetryDecoders = map[uint32]func(message.Message) *gcsv1.TelemetryEvent{
	0:   nil,
	1:   decodeSysStatus,
	24:  decodeGpsRaw,
	30:  decodeAttitude,
	33:  decodeGlobalPosition,
	42:  decodeMissionCurrent,
	62:  decodeNavControllerOutput,
	74:  decodeVfrHud,
	109: decodeRadioStatus,
	147: decodeBatteryStatus,
	193: decodeEkfStatusReport, // ardupilotmega, not common
	242: decodeHomePosition,
	253: decodeStatusText,
}

// protocolDecoders is the transaction-response dispatch table.
var protocolDecoders = map[uint32]func(message.Message) *gcsv1.ProtocolEvent{
	22: decodeParamValue,
	44: decodeMissionCount,
	47: decodeMissionAck,
	73: decodeMissionItemInt,
	77: decodeCommandAck,
}

// Decode converts a frame event into its envelope; unhandled IDs return zero.
func Decode(evt *gomavlib.EventFrame) Decoded {
	msg := evt.Message()

	d := Decoded{
		SysID:  evt.SystemID(),
		CompID: evt.ComponentID(),
		Seq:    evt.Frame.GetSequenceNumber(),
	}

	// Frame identity belongs to the envelope, not individual payloads.
	id := &gcsv1.VehicleId{
		SystemId:    uint32(d.SysID),
		ComponentId: uint32(d.CompID),
	}

	if decode, ok := telemetryDecoders[msg.GetID()]; ok {
		if decode == nil {
			return d
		}

		if evt := decode(msg); evt != nil {
			evt.VehicleId = id
			d.Telemetry = evt
		}

		return d
	}

	if decode, ok := protocolDecoders[msg.GetID()]; ok {
		if evt := decode(msg); evt != nil {
			evt.VehicleId = id
			d.Transaction = evt
		}
	}

	return d
}

// --- telemetry converters --------------------------------------------------

// DecodeHeartbeat converts a HEARTBEAT frame into fleet state.
//
// Separate from Decode because HEARTBEAT does not fold into telemetry: it
// establishes vehicle identity, armed state and flight mode, which is what the
// fleet layer tracks. Returns nil if the frame is not a heartbeat.
func DecodeHeartbeat(evt *gomavlib.EventFrame) *gcsv1.HeartbeatState {
	m, ok := evt.Message().(*ardupilotmega.MessageHeartbeat)
	if !ok {
		return nil
	}

	const (
		flagSafetyArmed = 128
		flagManualInput = 64
		flagHil         = 32
		flagStabilize   = 16
		flagGuided      = 8
		flagAuto        = 4
		flagTest        = 2
		flagCustomMode  = 1
	)

	base := uint8(m.BaseMode)

	return &gcsv1.HeartbeatState{
		Id: &gcsv1.VehicleId{
			SystemId:    uint32(evt.SystemID()),
			ComponentId: uint32(evt.ComponentID()),
		},
		Type:         gcsv1.MavType(m.Type),
		Autopilot:    gcsv1.MavAutopilot(m.Autopilot),
		SystemStatus: gcsv1.MavState(m.SystemStatus),
		// Armed is bit 7 of base_mode. It is surfaced as a bool because every
		// consumer wants the predicate, and re-deriving a bitmask at each call
		// site is where an inverted arm indicator comes from.
		Armed:              base&flagSafetyArmed != 0,
		ManualInputEnabled: base&flagManualInput != 0,
		HilEnabled:         base&flagHil != 0,
		StabilizeEnabled:   base&flagStabilize != 0,
		GuidedEnabled:      base&flagGuided != 0,
		AutoEnabled:        base&flagAuto != 0,
		TestEnabled:        base&flagTest != 0,
		CustomModeEnabled:  base&flagCustomMode != 0,
		CustomMode:         m.CustomMode,
		MavlinkVersion:     uint32(m.MavlinkVersion),
	}
}

func decodeSysStatus(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageSysStatus)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_SystemStatus{
			SystemStatus: &gcsv1.SystemStatus{
				SensorsPresent:      uint32(m.OnboardControlSensorsPresent),
				SensorsEnabled:      uint32(m.OnboardControlSensorsEnabled),
				SensorsHealth:       uint32(m.OnboardControlSensorsHealth),
				LoadDPct:            uint32(m.Load),
				VoltageBatteryMv:    uint32(m.VoltageBattery),
				CurrentBatteryCa:    int32(m.CurrentBattery),
				BatteryRemainingPct: int32(m.BatteryRemaining),
				DropRateCommCPct:    uint32(m.DropRateComm),
				ErrorsComm:          uint32(m.ErrorsComm),
			},
		},
	}
}

func decodeGpsRaw(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageGpsRawInt)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_GpsRaw{
			GpsRaw: &gcsv1.GpsRaw{
				TimeUsec: m.TimeUsec,
				FixType:  gcsv1.GpsFixType(m.FixType),
				LatDeg:   float64(m.Lat) * degE7ToDeg,
				LonDeg:   float64(m.Lon) * degE7ToDeg,
				// GPS_RAW_INT altitude is MSL. GLOBAL_POSITION_INT's
				// relative_alt is above home. They differ by field elevation,
				// so the two must never be merged into one field.
				AltMslM: float32(float64(m.Alt) * mmToM),
				Eph:     uint32(m.Eph),
				Epv:     uint32(m.Epv),
				// Vel and Cog keep wire units: the 65535 unknown sentinel is
				// defined in cm/s and centidegrees, and a divide here would
				// turn it into 655.35 with no way to recognise it downstream.
				VelCmS:            uint32(m.Vel),
				CogCdeg:           uint32(m.Cog),
				SatellitesVisible: uint32(m.SatellitesVisible),
			},
		},
	}
}

func decodeAttitude(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageAttitude)
	if !ok {
		return nil
	}

	// Radians on the wire and radians in the proto — this path converts
	// nothing. Degrees are a presentation concern and belong in the resolver.
	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_Attitude{
			Attitude: &gcsv1.Attitude{
				TimeBootMs:     m.TimeBootMs,
				RollRad:        m.Roll,
				PitchRad:       m.Pitch,
				YawRad:         m.Yaw,
				RollspeedRadS:  m.Rollspeed,
				PitchspeedRadS: m.Pitchspeed,
				YawspeedRadS:   m.Yawspeed,
			},
		},
	}
}

func decodeGlobalPosition(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageGlobalPositionInt)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_GlobalPosition{
			GlobalPosition: &gcsv1.GlobalPosition{
				TimeBootMs:   m.TimeBootMs,
				LatDeg:       float64(m.Lat) * degE7ToDeg,
				LonDeg:       float64(m.Lon) * degE7ToDeg,
				AltMslM:      float32(float64(m.Alt) * mmToM),
				AltRelativeM: float32(float64(m.RelativeAlt) * mmToM),
				// NED, positive down. The sign is preserved here; flipping it
				// into a climb rate is the resolver's job and happens once.
				VxMS: float32(float64(m.Vx) * cmPerSToMPS),
				VyMS: float32(float64(m.Vy) * cmPerSToMPS),
				VzMS: float32(float64(m.Vz) * cmPerSToMPS),
				// Centidegrees, kept in wire units for the 65535 sentinel.
				HdgCdeg: uint32(m.Hdg),
			},
		},
	}
}

func decodeMissionCurrent(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageMissionCurrent)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_MissionCurrent{
			MissionCurrent: &gcsv1.MissionCurrent{
				Seq:          uint32(m.Seq),
				Total:        uint32(m.Total),
				MissionState: gcsv1.MissionState(m.MissionState),
				MissionMode:  uint32(m.MissionMode),
			},
		},
	}
}

func decodeNavControllerOutput(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageNavControllerOutput)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_NavControllerOutput{
			NavControllerOutput: &gcsv1.NavControllerOutput{
				NavRollDeg:       m.NavRoll,
				NavPitchDeg:      m.NavPitch,
				NavBearingDeg:    int32(m.NavBearing),
				TargetBearingDeg: int32(m.TargetBearing),
				WpDistM:          uint32(m.WpDist),
				AltErrorM:        m.AltError,
				AspdErrorMS:      m.AspdError,
				XtrackErrorM:     m.XtrackError,
			},
		},
	}
}

func decodeVfrHud(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageVfrHud)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_VfrHud{
			VfrHud: &gcsv1.VfrHud{
				AirspeedMS:    m.Airspeed,
				GroundspeedMS: m.Groundspeed,
				// int16 on the wire, so it cannot carry the 65535 sentinel.
				// ArduPilot does emit negative values; wrapping to [0,360) is
				// the resolver's job, so the raw signed value passes through.
				HeadingDeg:  int32(m.Heading),
				ThrottlePct: uint32(m.Throttle),
				AltMslM:     m.Alt,
				// Already positive-up on the wire. Do not flip.
				ClimbMS: m.Climb,
			},
		},
	}
}

func decodeRadioStatus(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageRadioStatus)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_RadioStatus{
			RadioStatus: &gcsv1.RadioStatus{
				Rssi:     uint32(m.Rssi),
				Remrssi:  uint32(m.Remrssi),
				TxbufPct: uint32(m.Txbuf),
				Noise:    uint32(m.Noise),
				Remnoise: uint32(m.Remnoise),
				Rxerrors: uint32(m.Rxerrors),
				Fixed:    uint32(m.Fixed),
			},
		},
	}
}

func decodeBatteryStatus(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageBatteryStatus)
	if !ok {
		return nil
	}

	// Voltages is a fixed [10]uint16 where 65535 marks a cell that is not
	// present. Those slots are dropped rather than carried as 65.535 V.
	const cellAbsent = 65535

	cells := make([]uint32, 0, len(m.Voltages))

	for _, mv := range m.Voltages {
		if mv == cellAbsent {
			continue
		}

		cells = append(cells, uint32(mv))
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_BatteryStatus{
			BatteryStatus: &gcsv1.BatteryStatus{
				Id:                  uint32(m.Id),
				TemperatureCdeg:     int32(m.Temperature),
				CellVoltagesMv:      cells,
				CurrentBatteryCa:    int32(m.CurrentBattery),
				CurrentConsumedMah:  m.CurrentConsumed,
				EnergyConsumedHj:    m.EnergyConsumed,
				BatteryRemainingPct: int32(m.BatteryRemaining),
				TimeRemainingS:      m.TimeRemaining,
				ChargeState:         gcsv1.MavBatteryChargeState(m.ChargeState),
			},
		},
	}
}

func decodeEkfStatusReport(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageEkfStatusReport)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_EkfStatusReport{
			EkfStatusReport: &gcsv1.EkfStatusReport{
				Flags:              uint32(m.Flags),
				VelocityVariance:   m.VelocityVariance,
				PosHorizVariance:   m.PosHorizVariance,
				PosVertVariance:    m.PosVertVariance,
				CompassVariance:    m.CompassVariance,
				TerrainAltVariance: m.TerrainAltVariance,
				AirspeedVariance:   m.AirspeedVariance,
			},
		},
	}
}

func decodeHomePosition(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageHomePosition)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_HomePosition{
			HomePosition: &gcsv1.HomePosition{
				LatDeg:  float64(m.Latitude) * degE7ToDeg,
				LonDeg:  float64(m.Longitude) * degE7ToDeg,
				AltMslM: float32(float64(m.Altitude) * mmToM),
				LocalXM: m.X,
				LocalYM: m.Y,
				LocalZM: m.Z,
				QW:      m.Q[0],
				QX:      m.Q[1],
				QY:      m.Q[2],
				QZ:      m.Q[3],

				ApproachX: m.ApproachX,
				ApproachY: m.ApproachY,
				ApproachZ: m.ApproachZ,
				TimeUsec:  m.TimeUsec,
			},
		},
	}
}

func decodeStatusText(msg message.Message) *gcsv1.TelemetryEvent {
	m, ok := msg.(*ardupilotmega.MessageStatustext)
	if !ok {
		return nil
	}

	return &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_StatusText{
			StatusText: &gcsv1.StatusText{
				Severity: gcsv1.MavSeverity(m.Severity),
				Text:     m.Text,
				Id:       uint32(m.Id),
				ChunkSeq: uint32(m.ChunkSeq),
			},
		},
	}
}

// --- transaction converters ------------------------------------------------

func decodeParamValue(msg message.Message) *gcsv1.ProtocolEvent {
	m, ok := msg.(*ardupilotmega.MessageParamValue)
	if !ok {
		return nil
	}

	return &gcsv1.ProtocolEvent{
		Payload: &gcsv1.ProtocolEvent_ParamValue{
			// ParameterValue, MissionItem and MissionAck each still carry a
			// vehicle_id field of their own. They are deliberately left unset:
			// identity is on the envelope and duplicating it creates a second
			// source of truth that can disagree with the first.
			ParamValue: &gcsv1.ParameterValue{
				ParamId:    m.ParamId,
				ParamValue: m.ParamValue,
				ParamType:  gcsv1.MavParamType(m.ParamType),
				ParamCount: uint32(m.ParamCount),
				ParamIndex: uint32(m.ParamIndex),
			},
		},
	}
}

func decodeMissionCount(msg message.Message) *gcsv1.ProtocolEvent {
	m, ok := msg.(*ardupilotmega.MessageMissionCount)
	if !ok {
		return nil
	}

	return &gcsv1.ProtocolEvent{
		Payload: &gcsv1.ProtocolEvent_MissionCount{
			MissionCount: &gcsv1.MissionCount{
				Count:       uint32(m.Count),
				MissionType: gcsv1.MavMissionType(m.MissionType),
			},
		},
	}
}

func decodeMissionAck(msg message.Message) *gcsv1.ProtocolEvent {
	m, ok := msg.(*ardupilotmega.MessageMissionAck)
	if !ok {
		return nil
	}

	return &gcsv1.ProtocolEvent{
		Payload: &gcsv1.ProtocolEvent_MissionAck{
			MissionAck: &gcsv1.MissionAck{
				Result:      gcsv1.MavMissionResult(m.Type),
				MissionType: gcsv1.MavMissionType(m.MissionType),
			},
		},
	}
}

func decodeMissionItemInt(msg message.Message) *gcsv1.ProtocolEvent {
	m, ok := msg.(*ardupilotmega.MessageMissionItemInt)
	if !ok {
		return nil
	}

	return &gcsv1.ProtocolEvent{
		Payload: &gcsv1.ProtocolEvent_MissionItem{
			MissionItem: &gcsv1.MissionItem{
				Seq:          uint32(m.Seq),
				Frame:        gcsv1.MavFrame(m.Frame),
				Command:      gcsv1.MavCmd(m.Command),
				Current:      m.Current != 0,
				Autocontinue: m.Autocontinue != 0,
				Param1:       m.Param1,
				Param2:       m.Param2,
				Param3:       m.Param3,
				Param4:       m.Param4,
				// X and Y are degE7 and convert; Z is already float metres and
				// does not. Only two of the three coordinates scale.
				X:           float64(m.X) * degE7ToDeg,
				Y:           float64(m.Y) * degE7ToDeg,
				Z:           m.Z,
				MissionType: gcsv1.MavMissionType(m.MissionType),
			},
		},
	}
}

func decodeCommandAck(msg message.Message) *gcsv1.ProtocolEvent {
	m, ok := msg.(*ardupilotmega.MessageCommandAck)
	if !ok {
		return nil
	}

	return &gcsv1.ProtocolEvent{
		Payload: &gcsv1.ProtocolEvent_CommandAck{
			CommandAck: &gcsv1.CommandAck{
				Command:         gcsv1.MavCmd(m.Command),
				Result:          gcsv1.MavResult(m.Result),
				Progress:        uint32(m.Progress),
				ResultParam2:    m.ResultParam2,
				TargetSystem:    uint32(m.TargetSystem),
				TargetComponent: uint32(m.TargetComponent),
			},
		},
	}
}
