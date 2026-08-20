package bridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

// writeRecord is one captured outbound message.
type writeRecord struct {
	Link codec.LinkID
	Msg  message.Message
}

// recordingSource is a FrameSource that only records addressed writes.
type recordingSource struct {
	upstream codec.FrameSource
	err      error
	writes   []writeRecord
	events   chan gomavlib.Event
	mu       sync.Mutex
	failAt   int
	written  int
}

func newRecordingSource() *recordingSource {
	return &recordingSource{events: make(chan gomavlib.Event), failAt: -1}
}

func (s *recordingSource) Events() <-chan gomavlib.Event {
	if s.upstream != nil {
		return s.upstream.Events()
	}

	return s.events
}

func (s *recordingSource) Close() error { return nil }

func (s *recordingSource) WriteTo(link codec.LinkID, msg message.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failAt >= 0 && s.written == s.failAt {
		s.written++

		return s.err
	}

	s.written++
	s.writes = append(s.writes, writeRecord{Link: link, Msg: msg})

	return nil
}

func (s *recordingSource) snapshot() []writeRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]writeRecord(nil), s.writes...)
}

// intervalCommands projects captured writes down to the SET_MESSAGE_INTERVAL
// facts under test: who was addressed, which family, and what interval.
type intervalCommand struct {
	TargetSystem    uint8
	TargetComponent uint8
	MsgID           uint32
	IntervalUs      int32
}

func intervalCommands(t *testing.T, writes []writeRecord) []intervalCommand {
	t.Helper()

	out := make([]intervalCommand, 0, len(writes))

	for _, w := range writes {
		cmd, ok := w.Msg.(*ardupilotmega.MessageCommandLong)
		if !ok {
			t.Fatalf("expected COMMAND_LONG, got %T", w.Msg)
		}

		if got := uint32(cmd.Command); got != codec.CmdSetMessageInterval {
			t.Fatalf("command = %d, want %d", got, codec.CmdSetMessageInterval)
		}

		out = append(out, intervalCommand{
			TargetSystem:    cmd.TargetSystem,
			TargetComponent: cmd.TargetComponent,
			MsgID:           uint32(cmd.Param1),
			IntervalUs:      int32(cmd.Param2),
		})
	}

	return out
}

const rateNowMs int64 = 1_700_000_000_000

// newRequester wires a requester against a live route for sysid 1 / compid 1.
func newRequester(t *testing.T, src *recordingSource) *RateRequester {
	t.Helper()

	table := routes.NewTable()
	table.Upsert(routes.Entry{
		Key:  routes.Key{SysID: 1, CompID: 1},
		Link: "udp:10.0.0.5:14550",
	}, rateNowMs)

	return &RateRequester{
		Source: src,
		Routes: table,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() int64 { return rateNowMs },
	}
}

func fleet(kind gcsv1.FleetEventType, sysID, compID uint32) vehicle.Event {
	return vehicle.Event{Fleet: &gcsv1.FleetEvent{
		Type:      kind,
		VehicleId: &gcsv1.VehicleId{SystemId: sysID, ComponentId: compID},
	}}
}

// TestRateRequestsOnDiscovery pins the whole policy: the exact families, the
// exact intervals, and the target they are addressed to.
func TestRateRequestsOnDiscovery(t *testing.T) {
	src := newRecordingSource()
	r := newRequester(t, src)

	if err := r.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	want := []intervalCommand{
		{TargetSystem: 1, TargetComponent: 1, MsgID: 30, IntervalUs: 100_000},    // ATTITUDE 10 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 33, IntervalUs: 200_000},    // GLOBAL_POSITION_INT 5 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 74, IntervalUs: 200_000},    // VFR_HUD 5 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 24, IntervalUs: 1_000_000},  // GPS_RAW_INT 1 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 1, IntervalUs: 1_000_000},   // SYS_STATUS 1 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 147, IntervalUs: 1_000_000}, // BATTERY_STATUS 1 Hz
		{TargetSystem: 1, TargetComponent: 1, MsgID: 193, IntervalUs: 1_000_000}, // EKF_STATUS_REPORT 1 Hz
	}

	got := intervalCommands(t, src.snapshot())

	if len(got) != len(want) {
		t.Fatalf("requested %d families, want %d: %+v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestRateRequestsAddressTheObservedLink keeps the requests on the link the
// vehicle was actually heard on rather than any open channel.
func TestRateRequestsAddressTheObservedLink(t *testing.T) {
	src := newRecordingSource()
	r := newRequester(t, src)

	if err := r.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	for _, w := range src.snapshot() {
		if w.Link != "udp:10.0.0.5:14550" {
			t.Fatalf("wrote to link %q, want the route's link", w.Link)
		}
	}
}

// TestHeartbeatLearnsRouteBeforeRequestingRates covers the assembly invariant:
// the same inbound frame that discovers the vehicle must make its link
// routable before the discovery event reaches RateRequester.
func TestHeartbeatLearnsRouteBeforeRequestingRates(t *testing.T) {
	testSide, nodeSide := net.Pipe()

	node, err := codec.NewNode([]gomavlib.EndpointConf{
		gomavlib.EndpointCustomClient{
			Connect: func(context.Context) (net.Conn, error) { return nodeSide, nil },
			Label:   "rate-integration",
		},
	})
	if err != nil {
		t.Fatalf("creating node: %v", err)
	}

	source := newRecordingSource()
	source.upstream = node
	table := routes.NewTable()
	sink := &captureSink{}

	requester := &RateRequester{
		Source: source,
		Routes: table,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() int64 { return rateNowMs },
	}

	b, err := New(Config{
		Source: source,
		Routes: table,
		Sink:   MultiSink{requester, sink},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() int64 { return rateNowMs },
	})
	if err != nil {
		t.Fatalf("creating bridge: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(ctx) }()

	drained := make(chan struct{})
	go func() {
		defer close(drained)

		buf := make([]byte, 4096)
		for {
			if _, err := testSide.Read(buf); err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		cancel()
		<-runErr
		_ = node.Close()
		_ = testSide.Close()
		<-drained
	})

	raw, meta := loadFixture(t, "heartbeat_v2")
	if meta.CompID != AutopilotComponentID {
		t.Fatalf("fixture compid = %d, want autopilot component %d", meta.CompID, AutopilotComponentID)
	}

	go func() {
		_ = testSide.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = testSide.Write(raw)
	}()

	sink.await(t, "vehicle discovery", fleetOfType(
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED,
		uint32(meta.SysID),
	))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		writes := source.snapshot()
		if len(writes) == len(DefaultRates) {
			for _, write := range writes {
				if write.Link != "rate-integration" {
					t.Fatalf("rate request used link %q, want learned link %q", write.Link, "rate-integration")
				}
			}

			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("captured %d rate requests, want %d", len(source.snapshot()), len(DefaultRates))
}

// TestRateRequestsOnRecovery covers a vehicle that came back after an outage:
// the autopilot may have rebooted, so the intervals have to be asked for again.
func TestRateRequestsOnRecovery(t *testing.T) {
	src := newRecordingSource()
	r := newRequester(t, src)

	if err := r.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_RECOVERED, 1, 1)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if got := len(src.snapshot()); got != len(DefaultRates) {
		t.Fatalf("recovery requested %d families, want %d", got, len(DefaultRates))
	}
}

// TestNoRateRequestsForOrdinaryEvents is the back-pressure guard: only the two
// reachability transitions may put commands on the link.
func TestNoRateRequestsForOrdinaryEvents(t *testing.T) {
	quiet := []struct {
		name  string
		event vehicle.Event
	}{
		{"heartbeat updated", fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED, 1, 1)},
		{"vehicle lost", fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST, 1, 1)},
		{"unspecified", fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_UNSPECIFIED, 1, 1)},
		{"telemetry", vehicle.Event{Telemetry: &gcsv1.TelemetryEvent{}}},
		{"warning", vehicle.Event{Warning: &vehicle.Warning{VehicleID: &gcsv1.VehicleId{SystemId: 1}}}},
	}

	for _, tc := range quiet {
		t.Run(tc.name, func(t *testing.T) {
			src := newRecordingSource()
			r := newRequester(t, src)

			if err := r.Publish(t.Context(), tc.event); err != nil {
				t.Fatalf("Publish: %v", err)
			}

			if n := len(src.snapshot()); n != 0 {
				t.Errorf("wrote %d messages, want none", n)
			}
		})
	}
}

// TestNoRateRequestsForNonAutopilotComponents keeps gimbals and companion
// computers off the request path even though they share the system ID.
func TestNoRateRequestsForNonAutopilotComponents(t *testing.T) {
	for _, compID := range []uint32{0, 2, 100, 190, 191} {
		src := newRecordingSource()
		r := newRequester(t, src)
		r.Routes.Upsert(routes.Entry{
			Key:  routes.Key{SysID: 1, CompID: uint8(compID)},
			Link: "udp:10.0.0.5:14550",
		}, rateNowMs)

		if err := r.Publish(t.Context(),
			fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, compID)); err != nil {
			t.Fatalf("Publish for compid %d: %v", compID, err)
		}

		if n := len(src.snapshot()); n != 0 {
			t.Errorf("compid %d: wrote %d messages, want none", compID, n)
		}
	}
}

// TestRateRequestMissingRouteFails propagates an unroutable target rather than
// leaving the operator with a link that was never asked for telemetry.
func TestRateRequestMissingRouteFails(t *testing.T) {
	src := newRecordingSource()

	r := &RateRequester{
		Source: src,
		Routes: routes.NewTable(), // no route recorded
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() int64 { return rateNowMs },
	}

	err := r.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	if !errors.Is(err, routes.ErrNoRoute) {
		t.Fatalf("err = %v, want it to wrap %v", err, routes.ErrNoRoute)
	}

	if n := len(src.snapshot()); n != 0 {
		t.Errorf("wrote %d messages despite having no route", n)
	}
}

// TestRateRequestWriteErrorFails stops at the first failed write and reports
// which family it was, so the log names the message that could not be asked for.
func TestRateRequestWriteErrorFails(t *testing.T) {
	sentinel := errors.New("link gone")

	src := newRecordingSource()
	src.failAt = 2
	src.err = sentinel

	r := newRequester(t, src)

	err := r.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap %v", err, sentinel)
	}

	// Writes stop at the failure rather than continuing through the policy.
	if n := len(src.snapshot()); n != 2 {
		t.Errorf("captured %d writes before the failure, want 2", n)
	}
}

// TestRateRequesterStopsTheBridge is the end of that chain: a failing requester
// is a sink failure, and sink failures stop the receive loop.
func TestRateRequesterStopsTheBridge(t *testing.T) {
	sentinel := errors.New("link gone")

	src := newRecordingSource()
	src.failAt = 0
	src.err = sentinel

	r := newRequester(t, src)

	sink := MultiSink{r}

	err := sink.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	if !errors.Is(err, sentinel) {
		t.Fatalf("MultiSink err = %v, want it to wrap %v", err, sentinel)
	}
}

// TestRateRequestIntervalConversion pins the rate-to-microseconds conversion,
// including the two sentinel meanings.
func TestRateRequestIntervalConversion(t *testing.T) {
	cases := []struct {
		hz   float64
		want int32
	}{
		{hz: 10, want: 100_000},
		{hz: 5, want: 200_000},
		{hz: 1, want: 1_000_000},
		{hz: 4, want: 250_000},
		{hz: 0, want: codec.IntervalDefaultUs},
		{hz: -1, want: codec.IntervalDefaultUs},
	}

	for _, tc := range cases {
		if got := (RateRequest{Hz: tc.hz}).IntervalUs(); got != tc.want {
			t.Errorf("%.1f Hz = %d us, want %d", tc.hz, got, tc.want)
		}
	}
}

// TestMultiSinkPreservesOrder pins the fan-out contract the composite exists
// for: every sink sees the same events in the same sequence.
func TestMultiSinkPreservesOrder(t *testing.T) {
	first, second := &captureSink{}, &captureSink{}

	sink := MultiSink{first, second}

	events := []vehicle.Event{
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1),
		{Telemetry: &gcsv1.TelemetryEvent{}},
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED, 1, 1),
	}

	for _, ev := range events {
		if err := sink.Publish(t.Context(), ev); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	for name, s := range map[string]*captureSink{"first": first, "second": second} {
		got := s.snapshot()
		if len(got) != len(events) {
			t.Fatalf("%s sink saw %d events, want %d", name, len(got), len(events))
		}

		for i := range events {
			if got[i].Fleet.GetType() != events[i].Fleet.GetType() {
				t.Errorf("%s sink event %d out of order", name, i)
			}
		}
	}
}

// TestMultiSinkStopsAtFirstFailure keeps a failed publication from reaching
// later sinks, so no consumer records an event the pipeline is abandoning.
func TestMultiSinkStopsAtFirstFailure(t *testing.T) {
	sentinel := errors.New("sink down")

	failing := &captureSink{err: sentinel}
	after := &captureSink{}

	sink := MultiSink{&captureSink{}, failing, after}

	err := sink.Publish(t.Context(),
		fleet(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}

	if n := len(after.snapshot()); n != 0 {
		t.Errorf("sink after the failure saw %d events, want none", n)
	}
}
