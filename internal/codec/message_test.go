package codec

import (
	"math"
	"testing"

	"go.uber.org/goleak"
)

// TestMain asserts no goroutine leaks across the package.
//
// Node.Initialize starts three goroutines, so the claim is "none leaked", not
// "none running" — every node a test creates must reach Close.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// Assertions live as named functions rather than closures in the table. Inline
// closures push the table function's cognitive complexity past the gocognit
// limit in .golangci.yml, and a named function is what shows up in a failure
// trace anyway.

func assertAttitude(t *testing.T, d Decoded) {
	t.Helper()

	att := d.Telemetry.GetAttitude()
	if att == nil {
		t.Fatal("expected attitude payload")
	}

	// Radians in, radians out — this path converts nothing.
	assertClose(t, "roll", float64(att.GetRollRad()), 0.0174533, 1e-6)
	assertClose(t, "pitch", float64(att.GetPitchRad()), -0.0087266, 1e-6)
	assertClose(t, "yaw", float64(att.GetYawRad()), 1.5708, 1e-6)
}

func assertGlobalPosition(t *testing.T, d Decoded) {
	t.Helper()

	pos := d.Telemetry.GetGlobalPosition()
	if pos == nil {
		t.Fatal("expected global_position payload")
	}

	// NORM-LAT: int32 degE7 -> double degrees.
	assertClose(t, "lat", pos.GetLatDeg(), 47.6062, 1e-7)
	assertClose(t, "lon", pos.GetLonDeg(), -122.3321, 1e-7)
	// NORM-ALT-REL: int32 mm -> float metres.
	assertClose(t, "alt_msl", float64(pos.GetAltMslM()), 120.5, 1e-4)
	assertClose(t, "alt_rel", float64(pos.GetAltRelativeM()), 50.25, 1e-4)
	// NORM-VEL: int16 cm/s -> float m/s, sign preserved (NED, +down).
	assertClose(t, "vx", float64(pos.GetVxMS()), 12.0, 1e-5)
	assertClose(t, "vy", float64(pos.GetVyMS()), -3.0, 1e-5)
	assertClose(t, "vz", float64(pos.GetVzMS()), 1.5, 1e-5)

	// NORM-RETAINED: hdg keeps wire units. The 65535 sentinel is defined in
	// centidegrees and a divide here would turn it into 655.35, which no
	// consumer downstream can recognise as "unknown".
	if got := pos.GetHdgCdeg(); got != 9000 {
		t.Errorf("hdg_cdeg = %d, want 9000 (wire units retained)", got)
	}
}

func assertGpsRaw(t *testing.T, d Decoded) {
	t.Helper()

	gps := d.Telemetry.GetGpsRaw()
	if gps == nil {
		t.Fatal("expected gps_raw payload")
	}

	assertClose(t, "lat", gps.GetLatDeg(), 47.6062, 1e-7)
	// NORM-GPS-ALT: mm MSL -> metres MSL. This is MSL, never relative — the
	// datum differs from GLOBAL_POSITION_INT by the field elevation.
	assertClose(t, "alt_msl", float64(gps.GetAltMslM()), 120.5, 1e-4)

	if got := gps.GetVelCmS(); got != 1500 {
		t.Errorf("vel_cm_s = %d, want 1500 (wire units retained)", got)
	}

	if got := gps.GetSatellitesVisible(); got != 14 {
		t.Errorf("satellites = %d, want 14", got)
	}
}

func assertVfrHud(t *testing.T, d Decoded) {
	t.Helper()

	hud := d.Telemetry.GetVfrHud()
	if hud == nil {
		t.Fatal("expected vfr_hud payload")
	}

	// Climb is already positive-up on the wire and must not flip.
	assertClose(t, "climb", float64(hud.GetClimbMS()), 2.5, 1e-5)
	assertClose(t, "airspeed", float64(hud.GetAirspeedMS()), 18.5, 1e-5)

	if got := hud.GetHeadingDeg(); got != 90 {
		t.Errorf("heading = %d, want 90", got)
	}
}

func assertVfrHudNegativeHeading(t *testing.T, d Decoded) {
	t.Helper()

	hud := d.Telemetry.GetVfrHud()
	if hud == nil {
		t.Fatal("expected vfr_hud payload")
	}

	// The codec passes the signed value through; wrapping to [0,360) is the
	// resolver's job. VFR_HUD.heading is int16 and cannot hold the 65535
	// sentinel, so there is nothing to reject here.
	if got := hud.GetHeadingDeg(); got != -90 {
		t.Errorf("heading = %d, want -90 (sign preserved for the resolver)", got)
	}
}

func assertBatteryStatus(t *testing.T, d Decoded) {
	t.Helper()

	bat := d.Telemetry.GetBatteryStatus()
	if bat == nil {
		t.Fatal("expected battery_status payload")
	}

	// The fixture has 4 real cells and 6 slots of 65535. The absent slots are
	// dropped, not carried as 65.535 V.
	want := []uint32{4200, 4195, 4198, 4201}

	got := bat.GetCellVoltagesMv()
	if len(got) != len(want) {
		t.Fatalf("cells = %v, want %v (65535 slots dropped)", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %d, want %d", i, got[i], want[i])
		}
	}
}

func assertEkfStatusReport(t *testing.T, d Decoded) {
	t.Helper()

	ekf := d.Telemetry.GetEkfStatusReport()
	if ekf == nil {
		t.Fatal("expected ekf_status_report payload (ardupilotmega dialect)")
	}

	if got := ekf.GetFlags(); got != 831 {
		t.Errorf("flags = %d, want 831", got)
	}
}

func assertStatusText(t *testing.T, d Decoded) {
	t.Helper()

	st := d.Telemetry.GetStatusText()
	if st == nil {
		t.Fatal("expected status_text payload")
	}

	if got := st.GetText(); got != "EKF2 IMU0 is using GPS" {
		t.Errorf("text = %q", got)
	}
}

func assertSysStatus(t *testing.T, d Decoded) {
	t.Helper()

	sys := d.Telemetry.GetSystemStatus()
	if sys == nil {
		t.Fatal("expected system_status payload")
	}

	if got := sys.GetVoltageBatteryMv(); got != 12587 {
		t.Errorf("voltage = %d, want 12587", got)
	}
}

func assertRadioStatus(t *testing.T, d Decoded) {
	t.Helper()

	radio := d.Telemetry.GetRadioStatus()
	if radio == nil {
		t.Fatal("expected radio_status payload")
	}

	if got := radio.GetTxbufPct(); got != 98 {
		t.Errorf("txbuf = %d, want 98", got)
	}
}

func assertNavControllerOutput(t *testing.T, d Decoded) {
	t.Helper()

	nav := d.Telemetry.GetNavControllerOutput()
	if nav == nil {
		t.Fatal("expected nav_controller_output payload")
	}

	assertClose(t, "wp_dist", float64(nav.GetWpDistM()), 250, 1e-9)
}

func assertMissionCurrent(t *testing.T, d Decoded) {
	t.Helper()

	cur := d.Telemetry.GetMissionCurrent()
	if cur == nil {
		t.Fatal("expected mission_current payload")
	}

	if got := cur.GetSeq(); got != 3 {
		t.Errorf("seq = %d, want 3", got)
	}
}

func assertHomePosition(t *testing.T, d Decoded) {
	t.Helper()

	home := d.Telemetry.GetHomePosition()
	if home == nil {
		t.Fatal("expected home_position payload")
	}

	assertClose(t, "lat", home.GetLatDeg(), 47.6062, 1e-7)
	assertClose(t, "alt_msl", float64(home.GetAltMslM()), 120.0, 1e-4)
}

func TestDecodeTelemetryFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		assert  func(*testing.T, Decoded)
		fixture string
	}{
		{assertAttitude, "attitude_v2"},
		{assertGlobalPosition, "global_position_int_v2"},
		{assertGpsRaw, "gps_raw_int_v2"},
		{assertVfrHud, "vfr_hud_v2"},
		{assertVfrHudNegativeHeading, "vfr_hud_negative_heading_v2"},
		{assertBatteryStatus, "battery_status_v2"},
		{assertEkfStatusReport, "ekf_status_report_v2"},
		{assertStatusText, "statustext_v2"},
		{assertSysStatus, "sys_status_v2"},
		{assertRadioStatus, "radio_status_v2"},
		{assertNavControllerOutput, "nav_controller_output_v2"},
		{assertMissionCurrent, "mission_current_v2"},
		{assertHomePosition, "home_position_v2"},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			d, meta := decodeFixture(t, tc.fixture)

			if d.Telemetry == nil {
				t.Fatal("expected a TelemetryEvent, got none")
			}

			// Identity is on the envelope and only there.
			if got := d.Telemetry.GetVehicleId().GetSystemId(); got != uint32(meta.SysID) {
				t.Errorf("envelope vehicle_id.system_id = %d, want %d", got, meta.SysID)
			}

			tc.assert(t, d)
		})
	}
}

func assertParamValue(t *testing.T, d Decoded) {
	t.Helper()

	pv := d.Transaction.GetParamValue()
	if pv == nil {
		t.Fatal("expected param_value payload")
	}

	if got := pv.GetParamId(); got != "ARMING_CHECK" {
		t.Errorf("param_id = %q, want ARMING_CHECK", got)
	}

	if got := pv.GetParamCount(); got != 1372 {
		t.Errorf("param_count = %d, want 1372", got)
	}
}

func assertMissionCountPayload(t *testing.T, d Decoded) {
	t.Helper()

	mc := d.Transaction.GetMissionCount()
	if mc == nil {
		t.Fatal("expected mission_count payload")
	}

	if got := mc.GetCount(); got != 5 {
		t.Errorf("count = %d, want 5", got)
	}
}

func assertMissionAckPayload(t *testing.T, d Decoded) {
	t.Helper()

	if d.Transaction.GetMissionAck() == nil {
		t.Fatal("expected mission_ack payload")
	}
}

func assertMissionItem(t *testing.T, d Decoded) {
	t.Helper()

	item := d.Transaction.GetMissionItem()
	if item == nil {
		t.Fatal("expected mission_item payload")
	}

	// NORM-MISSION-XY: x and y are degE7 and convert; z is already metres and
	// does not. Only two of the three coordinates scale, which is the asymmetry
	// that makes a hand-rolled conversion at a call site go wrong.
	assertClose(t, "x", item.GetX(), 47.6062, 1e-7)
	assertClose(t, "y", item.GetY(), -122.3321, 1e-7)
	assertClose(t, "z", float64(item.GetZ()), 50.0, 1e-5)

	if !item.GetAutocontinue() {
		t.Error("autocontinue = false, want true")
	}
}

func assertCommandAck(t *testing.T, d Decoded) {
	t.Helper()

	ack := d.Transaction.GetCommandAck()
	if ack == nil {
		t.Fatal("expected command_ack payload")
	}

	if got := int32(ack.GetCommand()); got != 400 {
		t.Errorf("command = %d, want 400", got)
	}
}

func TestDecodeTransactionFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		assert  func(*testing.T, Decoded)
		fixture string
	}{
		{assertParamValue, "param_value_v2"},
		{assertMissionCountPayload, "mission_count_v2"},
		{assertMissionAckPayload, "mission_ack_v2"},
		{assertMissionItem, "mission_item_int_v2"},
		{assertCommandAck, "command_ack_v2"},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			d, meta := decodeFixture(t, tc.fixture)

			if d.Transaction == nil {
				t.Fatal("expected a ProtocolEvent, got none")
			}

			// A transaction response must never also be routed as telemetry: it
			// correlates against the in-flight registry instead of folding into
			// vehicle state, and conflating the two leaves the mission and
			// parameter protocols nowhere to land.
			if d.Telemetry != nil {
				t.Error("transaction response also produced a TelemetryEvent")
			}

			if got := d.Transaction.GetVehicleId().GetSystemId(); got != uint32(meta.SysID) {
				t.Errorf("envelope vehicle_id.system_id = %d, want %d", got, meta.SysID)
			}

			tc.assert(t, d)
		})
	}
}

// decodeFixture feeds a fixture through a real node and decodes the result,
// asserting the frame header matches the fixture's companion metadata.
func decodeFixture(t *testing.T, fixture string) (Decoded, fixtureMeta) {
	t.Helper()

	raw, meta := loadFixture(t, fixture)
	h := newHarness(t)

	frame := h.feed(raw)
	if frame == nil {
		t.Fatalf("no frame surfaced for %s", fixture)
	}

	d := Decode(frame)

	if d.SysID != meta.SysID {
		t.Errorf("sysID = %d, want %d", d.SysID, meta.SysID)
	}

	if d.Seq != meta.Seq {
		t.Errorf("seq = %d, want %d", d.Seq, meta.Seq)
	}

	return d, meta
}

func TestDecodeHeartbeatDrivesFleetState(t *testing.T) {
	t.Parallel()

	raw, _ := loadFixture(t, "heartbeat_v2")
	h := newHarness(t)

	frame := h.feed(raw)
	if frame == nil {
		t.Fatal("no frame surfaced")
	}

	// HEARTBEAT has no TelemetryEvent variant — it drives the fleet fold. Its
	// dispatch-table entry is nil for exactly this reason.
	d := Decode(frame)
	if d.Telemetry != nil {
		t.Error("HEARTBEAT produced a TelemetryEvent; it has no oneof variant")
	}

	hb := DecodeHeartbeat(frame)
	if hb == nil {
		t.Fatal("DecodeHeartbeat returned nil for a heartbeat frame")
	}

	// base_mode 209 = 128|64|16|1: armed, manual input, stabilize, custom mode.
	if !hb.GetArmed() {
		t.Error("armed = false; base_mode bit 7 (128) is set in the fixture")
	}

	if !hb.GetCustomModeEnabled() {
		t.Error("custom_mode_enabled = false; bit 0 is set")
	}

	if hb.GetGuidedEnabled() {
		t.Error("guided_enabled = true; bit 3 is NOT set in the fixture")
	}

	if got := hb.GetCustomMode(); got != 4 {
		t.Errorf("custom_mode = %d, want 4", got)
	}
}

func assertClose(t *testing.T, name string, got, want, tol float64) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (tolerance %v)", name, got, want, tol)
	}
}
