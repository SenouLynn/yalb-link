package vehicle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"

	"yalb.gcs/internal/codec"
)

func attitudeEvent(roll, pitch, yaw float32) *gcsv1.TelemetryEvent {
	return &gcsv1.TelemetryEvent{Payload: &gcsv1.TelemetryEvent_Attitude{
		Attitude: &gcsv1.Attitude{RollRad: roll, PitchRad: pitch, YawRad: yaw},
	}}
}

func ekfEvent(flags uint32) *gcsv1.TelemetryEvent {
	return &gcsv1.TelemetryEvent{Payload: &gcsv1.TelemetryEvent_EkfStatusReport{
		EkfStatusReport: &gcsv1.EkfStatusReport{Flags: flags},
	}}
}

func TestFoldAggregatesSnapshot(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)
	state, _ = Fold(state, telem(attitudeEvent(0.1, -0.2, 1.5), 30), t0+10)

	state, _ = Fold(state, telem(&gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_GlobalPosition{GlobalPosition: &gcsv1.GlobalPosition{
			LatDeg: 47.6062, LonDeg: -122.3321, AltMslM: 120.5, AltRelativeM: 50.25,
		}},
	}, 33), t0+20)

	state, _ = Fold(state, telem(&gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_SystemStatus{SystemStatus: &gcsv1.SystemStatus{
			VoltageBatteryMv: 12_600, CurrentBatteryCa: 350, BatteryRemainingPct: 87,
		}},
	}, 1), t0+30)

	snap := state.Snapshot
	if snap.GetRollRad() != 0.1 || snap.GetPitchRad() != -0.2 || snap.GetYawRad() != 1.5 {
		t.Errorf("attitude = %v/%v/%v, want 0.1/-0.2/1.5",
			snap.GetRollRad(), snap.GetPitchRad(), snap.GetYawRad())
	}

	if snap.GetLatitudeDeg() != 47.6062 || snap.GetAltitudeRelativeM() != 50.25 {
		t.Errorf("position = %v, %v", snap.GetLatitudeDeg(), snap.GetAltitudeRelativeM())
	}

	if snap.GetBatteryRemainingPct() != 87 {
		t.Errorf("battery remaining = %d, want 87", snap.GetBatteryRemainingPct())
	}

	if got := snap.GetId().GetSystemId(); got != 1 {
		t.Errorf("snapshot identity = %d, want 1", got)
	}

	if got := snap.GetLastUpdatedAt().AsTime().UnixMilli(); got != t0+30 {
		t.Errorf("last_updated_at = %d, want %d", got, t0+30)
	}

	if !snap.GetHeartbeat().GetObservedAt().IsValid() {
		t.Error("heartbeat observed_at not stamped from the injected clock")
	}
}

// VehicleSnapshot.heading_deg promises 0-359 and ArduPilot reports negatives
// on VFR_HUD. The normalisation happens in the fold, once.
func TestFoldNormalisesVfrHudHeading(t *testing.T) {
	for _, tc := range []struct {
		wire int32
		want int32
	}{
		{wire: 350, want: 350},
		{wire: -10, want: 350},
		{wire: -370, want: 350},
		{wire: 360, want: 0},
	} {
		state, _ := Fold(State{}, telem(&gcsv1.TelemetryEvent{
			Payload: &gcsv1.TelemetryEvent_VfrHud{VfrHud: &gcsv1.VfrHud{
				HeadingDeg: tc.wire, ClimbMS: -1.5, GroundspeedMS: 4,
			}},
		}, 74), t0)

		if got := state.Snapshot.GetHeadingDeg(); got != tc.want {
			t.Errorf("heading %d normalised to %d, want %d", tc.wire, got, tc.want)
		}

		// VFR_HUD's climb rate is already positive-up. The NED sign flip
		// belongs to GLOBAL_POSITION_INT and must not be applied twice.
		if got := state.Snapshot.GetClimbRateMS(); got != -1.5 {
			t.Errorf("climb rate = %v, want -1.5 (sign preserved)", got)
		}
	}
}

// The fold is a fold: the state handed in comes back unmodified, including the
// two reference-typed fields.
func TestFoldDoesNotMutateInput(t *testing.T) {
	before, _ := Fold(State{}, beat(false, 3), t0)
	before, _ = Fold(before, telem(attitudeEvent(0.1, 0.2, 0.3), 30), t0+10)

	snapshot := before.Snapshot
	roll := snapshot.GetRollRad()
	families := len(before.LastMsgSeenMs)

	after, _ := Fold(before, telem(attitudeEvent(9, 9, 9), 74), t0+20)

	if before.Snapshot != snapshot {
		t.Error("input state's snapshot pointer was replaced")
	}

	if got := snapshot.GetRollRad(); got != roll {
		t.Errorf("input snapshot mutated: roll %v -> %v", roll, got)
	}

	if len(before.LastMsgSeenMs) != families {
		t.Errorf("input freshness map mutated: %d -> %d entries", families, len(before.LastMsgSeenMs))
	}

	if after.Snapshot == snapshot {
		t.Error("returned state shares the input snapshot pointer")
	}
}

// One warning per transition. A vehicle that moves once is a different fault
// from one that flaps, and the counter is what tells them apart.
func TestSourceConflictWarnsPerTransition(t *testing.T) {
	from := func(addr string, ms int64, state State) (State, []Event) {
		in := beat(false, 3)
		in.SrcAddr = addr

		return Fold(state, in, ms)
	}

	state, events := from("10.0.0.5:14550", t0, State{})
	if warnings(events) != 0 {
		t.Fatal("first attribution warned; it is not a conflict")
	}

	state, events = from("10.0.0.5:14550", t0+1_000, state)
	if warnings(events) != 0 {
		t.Fatal("unchanged source warned")
	}

	state, events = from("10.0.0.9:14550", t0+2_000, state)
	if warnings(events) != 1 {
		t.Fatalf("changed source produced %d warnings, want exactly 1", warnings(events))
	}

	w := events[0].Warning
	if w.Type != WarningSourceConflict || w.Previous != "10.0.0.5:14550" || w.Current != "10.0.0.9:14550" {
		t.Errorf("warning = %+v, want a SOURCE_CONFLICT naming both addresses", w)
	}

	if w.OccurredMs != t0+2_000 {
		t.Errorf("warning occurred_ms = %d, want %d", w.OccurredMs, t0+2_000)
	}

	state, events = from("10.0.0.9:14550", t0+3_000, state)
	if warnings(events) != 0 {
		t.Fatal("settled source warned again")
	}

	state, _ = from("10.0.0.5:14550", t0+4_000, state)
	if state.SourceConflicts != 2 {
		t.Errorf("SourceConflicts = %d after a flap, want 2", state.SourceConflicts)
	}

	if state.LastSrcAddr != "10.0.0.5:14550" {
		t.Errorf("LastSrcAddr = %q, want the most recent source", state.LastSrcAddr)
	}
}

// A conflicting source is reported, never fatal: the frame still folds.
func TestSourceConflictStillFolds(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	in := beat(true, 4)
	in.SrcAddr = "192.168.1.7:14550"

	state, events := Fold(state, in, t0+1_000)

	if warnings(events) != 1 {
		t.Fatalf("expected one warning, got %d", warnings(events))
	}

	if !state.Armed || state.CustomMode != 4 {
		t.Error("heartbeat from the conflicting source was not applied")
	}
}

func warnings(events []Event) int {
	var n int

	for _, e := range events {
		if e.Warning != nil {
			n++
		}
	}

	return n
}

func TestArmGateFollowsEkfFlags(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	// Fail closed: no EKF report yet is not the same as a healthy EKF.
	if state.ArmAllowed() {
		t.Error("arming allowed before any EKF_STATUS_REPORT")
	}

	for _, tc := range []struct {
		reason string
		flags  uint32
		allow  bool
	}{
		{reason: "EKF not initialised", flags: EkfAttitude | EkfUninitialized, allow: false},
		{reason: "EKF has no attitude solution", flags: 0, allow: false},
		{reason: "", flags: EkfAttitude | 0x1e, allow: true},
	} {
		next, _ := Fold(state, telem(ekfEvent(tc.flags), 193), t0+100)

		if next.ArmAllowed() != tc.allow {
			t.Errorf("flags %#x: ArmAllowed = %v, want %v", tc.flags, next.ArmAllowed(), tc.allow)
		}

		if got := next.ArmBlockedReason(); got != tc.reason {
			t.Errorf("flags %#x: reason = %q, want %q", tc.flags, got, tc.reason)
		}
	}
}

func TestFamilyFreshness(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)
	state, _ = Fold(state, telem(attitudeEvent(0, 0, 0), 30), t0+500)
	state, _ = Fold(state, beat(false, 3), t0+4_000)

	// Vehicle-wide age says the link is healthy...
	if got := state.AgeMs(t0 + 4_000); got != 0 {
		t.Errorf("AgeMs = %d, want 0", got)
	}

	// ...while ATTITUDE specifically has been silent for 3.5s. An instrument
	// that cannot tell these apart renders stale data as live.
	age, ok := state.FamilyAgeMs(30, t0+4_000)
	if !ok || age != 3_500 {
		t.Errorf("ATTITUDE age = %d (seen %v), want 3500", age, ok)
	}

	if _, ok := state.FamilyAgeMs(147, t0+4_000); ok {
		t.Error("BATTERY_STATUS reported as seen; it never arrived")
	}
}

func TestTTLRefreshThrottle(t *testing.T) {
	state, _ := Fold(State{}, beat(false, 3), t0)

	if !state.ShouldRefreshTTL(t0) {
		t.Fatal("first refresh not due")
	}

	state.MarkTTLRefreshed(t0)

	if state.ShouldRefreshTTL(t0 + expireThrottleMs) {
		t.Error("refresh due again at exactly the throttle interval")
	}

	if !state.ShouldRefreshTTL(t0 + expireThrottleMs + 1) {
		t.Error("refresh not due past the throttle interval")
	}
}

// The fold's contract is that time is an input. This asserts it against the
// parsed source rather than against intent — the rule is one careless import
// away from being false, and nothing else in the suite would notice. Comments
// are excluded by parsing rather than grepping, or the prose explaining the
// rule would fail it.
func TestNoWallClock(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}

	fset := token.NewFileSet()

	var checked int

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		// ParseFile rather than the deprecated ParseDir, and comments are
		// dropped so the prose explaining this rule does not trip it.
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}

		checked++

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "time" {
				return true
			}

			if sel.Sel.Name == "Now" || sel.Sel.Name == "Since" {
				t.Errorf("%s calls time.%s; the fold takes its clock as a parameter",
					name, sel.Sel.Name)
			}

			return true
		})
	}

	if checked == 0 {
		t.Fatal("no source files found — this gate would pass vacuously")
	}
}

// One clock, not two: gomavlib must not tear a channel down before the fold
// has had the chance to declare the vehicle lost.
func TestLinkIdleTimeoutOutlivesHeartbeatTTL(t *testing.T) {
	if codec.LinkIdleTimeout <= codec.HeartbeatTTL {
		t.Errorf("LinkIdleTimeout %v <= HeartbeatTTL %v: channel teardown can front-run VEHICLE_LOST",
			codec.LinkIdleTimeout, codec.HeartbeatTTL)
	}
}
