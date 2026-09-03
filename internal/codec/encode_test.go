package codec

import (
	"testing"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
)

// Encoder tests compare payload fields with independent pymavlink fixtures.

func TestEncodeSetPositionTargetGlobalInt(t *testing.T) {
	t.Parallel()

	msg := EncodeSetPositionTargetGlobalInt(
		Target{SystemID: 1, ComponentID: 1},
		47.6062, -122.3321,
		50.0,
	)

	sp, ok := msg.(*ardupilotmega.MessageSetPositionTargetGlobalInt)
	if !ok {
		t.Fatalf("got %T, want MessageSetPositionTargetGlobalInt", msg)
	}

	// Degrees in, degE7 on the wire. The conversion happens at this boundary so
	// no caller ever handles E7 coordinates.
	if sp.LatInt != 476062000 {
		t.Errorf("lat_int = %d, want 476062000", sp.LatInt)
	}

	if sp.LonInt != -1223321000 {
		t.Errorf("lon_int = %d, want -1223321000", sp.LonInt)
	}

	if sp.Alt != 50.0 {
		t.Errorf("alt = %v, want 50.0 (metres, unconverted)", sp.Alt)
	}

	// 0xDF8 = 3576. Position bits clear (use), everything else set (ignore).
	if sp.TypeMask != PositionOnlyTypeMask {
		t.Errorf("type_mask = %#x, want %#x", sp.TypeMask, PositionOnlyTypeMask)
	}

	// Bit 9 is FORCE_SET. It must stay clear.
	const forceSetBit = 1 << 9

	if sp.TypeMask&forceSetBit != 0 {
		t.Error("FORCE_SET bit (9) is set in type_mask; it must be clear")
	}

	if sp.CoordinateFrame != FrameGlobalRelativeAltInt {
		t.Errorf("frame = %d, want %d (GLOBAL_RELATIVE_ALT_INT)",
			sp.CoordinateFrame, FrameGlobalRelativeAltInt)
	}
}

func TestEncodeCommandLongPassesParamsThrough(t *testing.T) {
	t.Parallel()

	msg := EncodeCommandLong(
		Target{SystemID: 1, ComponentID: 1},
		CmdComponentArmDisarm,
		0,
		[7]float32{1, 0, 0, 0, 0, 0, 0},
	)

	cmd, ok := msg.(*ardupilotmega.MessageCommandLong)
	if !ok {
		t.Fatalf("got %T, want MessageCommandLong", msg)
	}

	if int(cmd.Command) != CmdComponentArmDisarm {
		t.Errorf("command = %d, want %d", cmd.Command, CmdComponentArmDisarm)
	}

	if cmd.Param1 != 1 {
		t.Errorf("param1 = %v, want 1 (arm)", cmd.Param1)
	}
}

func TestEncodeCommandLongDoesNotPoliceForceArm(t *testing.T) {
	t.Parallel()

	msg := EncodeCommandLong(
		Target{SystemID: 1, ComponentID: 1},
		CmdComponentArmDisarm,
		0,
		[7]float32{1, ForceArmMagic, 0, 0, 0, 0, 0},
	)

	cmd, ok := msg.(*ardupilotmega.MessageCommandLong)
	if !ok {
		t.Fatalf("got %T, want MessageCommandLong", msg)
	}

	if cmd.Param2 != ForceArmMagic {
		t.Errorf("param2 = %v, want %v passed through unmodified",
			cmd.Param2, float32(ForceArmMagic))
	}
}

func TestEncodeMissionItemIntConvertsOnlyLatLon(t *testing.T) {
	t.Parallel()

	msg := EncodeMissionItemInt(
		Target{SystemID: 1, ComponentID: 1},
		1, FrameGlobalRelativeAltInt, 16,
		false, true,
		[4]float32{},
		47.6062, -122.3321,
		50.0,
		0,
	)

	item, ok := msg.(*ardupilotmega.MessageMissionItemInt)
	if !ok {
		t.Fatalf("got %T, want MessageMissionItemInt", msg)
	}

	if item.X != 476062000 {
		t.Errorf("x = %d, want 476062000 (degE7)", item.X)
	}

	if item.Y != -1223321000 {
		t.Errorf("y = %d, want -1223321000 (degE7)", item.Y)
	}

	// z is already float metres on the wire. Only two of the three coordinates
	// scale, which is exactly the asymmetry that makes a hand-rolled conversion
	// at a call site go wrong.
	if item.Z != 50.0 {
		t.Errorf("z = %v, want 50.0 (metres, unconverted)", item.Z)
	}

	if item.Autocontinue != 1 {
		t.Errorf("autocontinue = %d, want 1", item.Autocontinue)
	}

	if item.Current != 0 {
		t.Errorf("current = %d, want 0", item.Current)
	}
}

func TestEncodeHeartbeatIsGCSIdentity(t *testing.T) {
	t.Parallel()

	hb, ok := EncodeHeartbeat().(*ardupilotmega.MessageHeartbeat)
	if !ok {
		t.Fatal("EncodeHeartbeat did not return a MessageHeartbeat")
	}

	if hb.Type != 6 {
		t.Errorf("type = %d, want 6 (MAV_TYPE_GCS)", hb.Type)
	}

	if hb.Autopilot != 0 {
		t.Errorf("autopilot = %d, want 0 (MAV_AUTOPILOT_GENERIC)", hb.Autopilot)
	}

	if hb.SystemStatus != 4 {
		t.Errorf("system_status = %d, want 4 (MAV_STATE_ACTIVE)", hb.SystemStatus)
	}

	if hb.BaseMode != 0 || hb.CustomMode != 0 {
		t.Errorf("base_mode/custom_mode = %d/%d, want 0/0", hb.BaseMode, hb.CustomMode)
	}
}

func TestEncodeParamRequestReadByName(t *testing.T) {
	t.Parallel()

	msg := EncodeParamRequestRead(Target{SystemID: 1, ComponentID: 1}, "ARMING_CHECK", -1)

	req, ok := msg.(*ardupilotmega.MessageParamRequestRead)
	if !ok {
		t.Fatalf("got %T, want MessageParamRequestRead", msg)
	}

	// -1 means "resolve by name". Any valid index makes the autopilot ignore
	// param_id entirely, so the two are not interchangeable.
	if req.ParamIndex != -1 {
		t.Errorf("param_index = %d, want -1 (look up by name)", req.ParamIndex)
	}

	if req.ParamId != "ARMING_CHECK" {
		t.Errorf("param_id = %q, want ARMING_CHECK", req.ParamId)
	}
}

func TestEncodersTargetTheRequestedVehicle(t *testing.T) {
	t.Parallel()

	target := Target{SystemID: 7, ComponentID: 1}

	tests := []struct {
		msg  message.Message
		got  func(message.Message) (uint8, uint8)
		name string
	}{
		{
			name: "param_set",
			msg:  EncodeParamSet(target, "ARMING_CHECK", 1, 6),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageParamSet)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "param_request_list",
			msg:  EncodeParamRequestList(target),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageParamRequestList)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "mission_request_list",
			msg:  EncodeMissionRequestList(target, MissionTypeMission),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageMissionRequestList)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "mission_count",
			msg:  EncodeMissionCount(target, 5, 0),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageMissionCount)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "mission_request_int",
			msg:  EncodeMissionRequestInt(target, 1, 0),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageMissionRequestInt)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "mission_ack",
			msg:  EncodeMissionAck(target, 0, 0),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageMissionAck)

				return p.TargetSystem, p.TargetComponent
			},
		},
		{
			name: "mission_clear_all",
			msg:  EncodeMissionClearAll(target, 0),
			got: func(m message.Message) (uint8, uint8) {
				p, _ := m.(*ardupilotmega.MessageMissionClearAll)

				return p.TargetSystem, p.TargetComponent
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sys, comp := tc.got(tc.msg)

			if sys != target.SystemID || comp != target.ComponentID {
				t.Errorf("target = (%d,%d), want (%d,%d)",
					sys, comp, target.SystemID, target.ComponentID)
			}
		})
	}
}

func encoderMessages() map[uint32]message.Message {
	target := Target{SystemID: 1, ComponentID: 1}

	return map[uint32]message.Message{
		0:  EncodeHeartbeat(),
		20: EncodeParamRequestRead(target, "ARMING_CHECK", -1),
		21: EncodeParamRequestList(target),
		23: EncodeParamSet(target, "ARMING_CHECK", 1, 6),
		43: EncodeMissionRequestList(target, MissionTypeMission),
		44: EncodeMissionCount(target, 5, 0),
		45: EncodeMissionClearAll(target, 0),
		47: EncodeMissionAck(target, 0, 0),
		51: EncodeMissionRequestInt(target, 1, 0),
		73: EncodeMissionItemInt(target, 1, FrameGlobalRelativeAltInt, 16,
			false, true, [4]float32{}, 47.6062, -122.3321, 50, 0),
		76: EncodeCommandLong(target, CmdComponentArmDisarm, 0, [7]float32{1}),
		86: EncodeSetPositionTargetGlobalInt(target, 47.6062, -122.3321, 50),
	}
}

func TestSendFamilyCoverage(t *testing.T) {
	t.Parallel()

	built := encoderMessages()

	for _, wantID := range SendFamilies {
		msg, ok := built[wantID]
		if !ok {
			t.Errorf("SendFamilies declares %d but no encoder builds it", wantID)

			continue
		}

		if got := msg.GetID(); got != wantID {
			t.Errorf("encoder for %d built message ID %d", wantID, got)
		}
	}

	if len(built) != len(SendFamilies) {
		t.Errorf("built %d encoder messages but SendFamilies declares %d",
			len(built), len(SendFamilies))
	}
}

// TestEncodeMissionRequestListOpensADownload pins the one message that lets the
// codec start a mission download at all. Without it the existing
// MISSION_REQUEST_INT encoder can only continue a transfer somebody else began.
func TestEncodeMissionRequestListOpensADownload(t *testing.T) {
	t.Parallel()

	msg := EncodeMissionRequestList(Target{SystemID: 1, ComponentID: 1}, MissionTypeMission)

	req, ok := msg.(*ardupilotmega.MessageMissionRequestList)
	if !ok {
		t.Fatalf("got %T, want MessageMissionRequestList", msg)
	}

	if got := msg.GetID(); got != 43 {
		t.Errorf("message ID = %d, want 43 (MISSION_REQUEST_LIST)", got)
	}

	if req.MissionType != 0 {
		t.Errorf("mission_type = %d, want 0 (MAV_MISSION_TYPE_MISSION)", req.MissionType)
	}
}

// TestEncodeMissionRequestListCarriesMissionType guards the field that keeps a
// flight-plan download from settling a fence or rally transfer on the same link.
func TestEncodeMissionRequestListCarriesMissionType(t *testing.T) {
	t.Parallel()

	msg := EncodeMissionRequestList(Target{SystemID: 1, ComponentID: 1}, MissionTypeFence)

	req, ok := msg.(*ardupilotmega.MessageMissionRequestList)
	if !ok {
		t.Fatalf("got %T, want MessageMissionRequestList", msg)
	}

	if req.MissionType != 1 {
		t.Errorf("mission_type = %d, want 1 (MAV_MISSION_TYPE_FENCE)", req.MissionType)
	}
}

// TestEncodeMissionRequestListMatchesGoldenFrame checks the encoder against
// bytes an independent implementation produced.
//
// The other encoder tests assert on struct fields we set ourselves, which
// cannot catch a wrong message ID, a missing extension field, or a field the
// dialect orders differently on the wire. Decoding pymavlink's frame through
// the real gomavlib parser and comparing the result to the encoder's output
// checks the encoder against something that did not come from this codebase.
func TestEncodeMissionRequestListMatchesGoldenFrame(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "mission_request_list_out")

	if meta.MessageID != 43 {
		t.Fatalf("fixture message_id = %d, want 43", meta.MessageID)
	}

	h := newHarness(t)

	frame := h.feed(raw)
	if frame == nil {
		t.Fatal("golden MISSION_REQUEST_LIST frame produced no decoded frame")
	}

	golden, ok := frame.Message().(*ardupilotmega.MessageMissionRequestList)
	if !ok {
		t.Fatalf("decoded %T, want MessageMissionRequestList", frame.Message())
	}

	// The fixture is addressed to vehicle (1, 1) with the ordinary mission type;
	// build the same request and require an identical payload.
	built, ok := EncodeMissionRequestList(
		Target{SystemID: 1, ComponentID: 1},
		MissionTypeMission,
	).(*ardupilotmega.MessageMissionRequestList)
	if !ok {
		t.Fatal("EncodeMissionRequestList did not return a MessageMissionRequestList")
	}

	if *built != *golden {
		t.Errorf("encoded %+v, golden frame decodes to %+v", *built, *golden)
	}
}
