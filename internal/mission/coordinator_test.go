package mission

import (
	"context"
	"errors"
	"io"
	"log/slog"
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

// vehicleUnderTest is the vehicle every test downloads from unless it is
// deliberately talking to a second one.
var vehicleUnderTest = codec.Target{SystemID: 1, ComponentID: codec.AutopilotComponentID}

type fakeSource struct {
	write func(message.Message) error

	mu   sync.Mutex
	sent []message.Message
}

func (*fakeSource) Events() <-chan gomavlib.Event { return nil }
func (*fakeSource) Close() error                  { return nil }

func (f *fakeSource) WriteTo(_ codec.LinkID, msg message.Message) error {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()

	return f.write(msg)
}

// requestedSeqs returns every sequence number the coordinator asked for, in
// order, including repeats. "Exactly once" is only checkable if repeats survive.
func (f *fakeSource) requestedSeqs() []uint16 {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []uint16

	for _, msg := range f.sent {
		if req, ok := msg.(*ardupilotmega.MessageMissionRequestInt); ok {
			out = append(out, req.Seq)
		}
	}

	return out
}

func (f *fakeSource) acks() []*ardupilotmega.MessageMissionAck {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []*ardupilotmega.MessageMissionAck

	for _, msg := range f.sent {
		if ack, ok := msg.(*ardupilotmega.MessageMissionAck); ok {
			out = append(out, ack)
		}
	}

	return out
}

func coordinatorFor(t *testing.T, write func(message.Message) error) (*Coordinator, *fakeSource) {
	t.Helper()

	table := routes.NewTable()
	for _, sysID := range []uint8{1, 2} {
		table.Upsert(routes.Entry{
			Key:  routes.Key{SysID: sysID, CompID: codec.AutopilotComponentID},
			Link: "test",
		}, time.Now().UnixMilli())
	}

	source := &fakeSource{write: write}

	return &Coordinator{
		Source:  source,
		Routes:  table,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Timeout: 50 * time.Millisecond,
	}, source
}

// protocolEvent wraps a mission payload as the bridge would deliver it: the
// envelope names the sending vehicle, the payload names the addressed GCS.
func protocolEvent(from codec.Target, payload any) vehicle.Event {
	ev := &gcsv1.ProtocolEvent{
		VehicleId: &gcsv1.VehicleId{
			SystemId:    uint32(from.SystemID),
			ComponentId: uint32(from.ComponentID),
		},
	}

	switch p := payload.(type) {
	case *gcsv1.MissionCount:
		ev.Payload = &gcsv1.ProtocolEvent_MissionCount{MissionCount: p}
	case *gcsv1.MissionItem:
		ev.Payload = &gcsv1.ProtocolEvent_MissionItem{MissionItem: p}
	case *gcsv1.MissionAck:
		ev.Payload = &gcsv1.ProtocolEvent_MissionAck{MissionAck: p}
	}

	return vehicle.Event{Protocol: ev}
}

func countFrom(from codec.Target, count uint32) vehicle.Event {
	return protocolEvent(from, &gcsv1.MissionCount{
		Count:           count,
		MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
		TargetSystem:    uint32(codec.GCSSystemID),
		TargetComponent: uint32(codec.GCSComponentID),
	})
}

func itemFrom(from codec.Target, seq uint32) vehicle.Event {
	return protocolEvent(from, &gcsv1.MissionItem{
		Seq:             seq,
		Frame:           gcsv1.MavFrame_MAV_FRAME_GLOBAL_RELATIVE_ALT_INT,
		Command:         gcsv1.MavCmd_MAV_CMD_NAV_WAYPOINT,
		MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
		X:               47.6062 + float64(seq),
		Y:               -122.3321,
		Z:               50,
		TargetSystem:    uint32(codec.GCSSystemID),
		TargetComponent: uint32(codec.GCSComponentID),
	})
}

// respondingVehicle simulates a well-behaved autopilot: it reports count items
// and answers each item request with the matching sequence.
func respondingVehicle(coord **Coordinator, from codec.Target, count uint32) func(message.Message) error {
	return func(msg message.Message) error {
		ctx := context.Background()

		switch m := msg.(type) {
		case *ardupilotmega.MessageMissionRequestList:
			return (*coord).Publish(ctx, countFrom(from, count))
		case *ardupilotmega.MessageMissionRequestInt:
			return (*coord).Publish(ctx, itemFrom(from, uint32(m.Seq)))
		}

		return nil
	}
}

func TestDownloadAssemblesAdvertisedItemsInOrder(t *testing.T) {
	t.Parallel()

	var coord *Coordinator
	coord, source := coordinatorFor(t, respondingVehicle(&coord, vehicleUnderTest, 3))

	snap, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if len(snap.GetItems()) != 3 {
		t.Fatalf("items = %d, want 3", len(snap.GetItems()))
	}

	for i, item := range snap.GetItems() {
		if item.GetSeq() != uint32(i) {
			t.Errorf("items[%d].seq = %d, want %d", i, item.GetSeq(), i)
		}
	}

	// Exactly the advertised sequences, each asked for once.
	if got := source.requestedSeqs(); len(got) != 3 ||
		got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Errorf("requested sequences = %v, want [0 1 2] exactly once each", got)
	}

	if snap.GetVehicleId().GetSystemId() != 1 {
		t.Errorf("snapshot vehicle = %v, want system 1", snap.GetVehicleId())
	}

	if snap.GetObservedAt() == nil {
		t.Error("snapshot has no observed_at")
	}
}

func TestZeroCountReturnsEmptySnapshotAndAcknowledges(t *testing.T) {
	t.Parallel()

	var coord *Coordinator
	coord, source := coordinatorFor(t, respondingVehicle(&coord, vehicleUnderTest, 0))

	snap, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if len(snap.GetItems()) != 0 {
		t.Fatalf("items = %d, want 0", len(snap.GetItems()))
	}

	if snap.GetObservedAt() == nil {
		t.Error("empty snapshot has no observed_at; it cannot be told from an unfilled message")
	}

	// A zero-item mission still has to be acknowledged or the vehicle keeps the
	// transaction open.
	acks := source.acks()
	if len(acks) != 1 {
		t.Fatalf("sent %d MISSION_ACKs, want 1", len(acks))
	}

	if acks[0].Type != ardupilotmega.MAV_MISSION_RESULT(gcsv1.MavMissionResult_MAV_MISSION_ACCEPTED) {
		t.Errorf("ack result = %v, want MAV_MISSION_ACCEPTED", acks[0].Type)
	}

	if source.requestedSeqs() != nil {
		t.Errorf("requested %v items for an empty mission, want none", source.requestedSeqs())
	}
}

func TestDuplicateDownloadForSameVehicleAndTypeIsRejected(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var coord *Coordinator

	coord, _ = coordinatorFor(t, func(msg message.Message) error {
		if _, ok := msg.(*ardupilotmega.MessageMissionRequestList); ok {
			// Hold the first download open with its slot claimed.
			<-release
			return (*coord).Publish(context.Background(), countFrom(vehicleUnderTest, 0))
		}

		return nil
	})

	first := make(chan error, 1)

	go func() {
		_, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
		first <- err
	}()

	// Wait until the first download owns the slot.
	waitFor(t, func() bool { return coord.inFlightCount() == 1 })

	_, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
	if !errors.Is(err, ErrInFlight) {
		t.Errorf("second download error = %v, want ErrInFlight", err)
	}

	close(release)

	if err := <-first; err != nil {
		t.Fatalf("first download: %v", err)
	}

	if got := coord.inFlightCount(); got != 0 {
		t.Errorf("in-flight slots = %d after completion, want 0", got)
	}
}

// waitFor polls a condition rather than sleeping a fixed interval, so the test
// is not tuned to machine speed.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("condition not met within 2s")
}

// --- correlation: what must never settle a transaction ----------------------

// foreignResponses covers every way a response can look plausible and still
// belong to a different transfer. Each must leave the download unsatisfied
// rather than completing it with the wrong data, so each ends in a timeout.
func TestForeignResponsesCannotCompleteADownload(t *testing.T) {
	t.Parallel()

	otherVehicle := codec.Target{SystemID: 2, ComponentID: codec.AutopilotComponentID}
	otherComponent := codec.Target{SystemID: 1, ComponentID: 42}

	tests := []struct {
		reply func(*Coordinator) vehicle.Event
		name  string
		why   string
	}{
		{
			name: "another vehicle",
			why:  "same mission type and GCS target, different system ID",
			reply: func(*Coordinator) vehicle.Event {
				return countFrom(otherVehicle, 3)
			},
		},
		{
			name: "another component on the same vehicle",
			why:  "components sharing a system ID hold separate missions",
			reply: func(*Coordinator) vehicle.Event {
				return countFrom(otherComponent, 3)
			},
		},
		{
			name: "another mission type",
			why:  "a fence transfer shares the link and the vehicle",
			reply: func(*Coordinator) vehicle.Event {
				return protocolEvent(vehicleUnderTest, &gcsv1.MissionCount{
					Count:           3,
					MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_FENCE,
					TargetSystem:    uint32(codec.GCSSystemID),
					TargetComponent: uint32(codec.GCSComponentID),
				})
			},
		},
		{
			name: "addressed to another ground station",
			why:  "right vehicle, right mission type, answering somebody else",
			reply: func(*Coordinator) vehicle.Event {
				return protocolEvent(vehicleUnderTest, &gcsv1.MissionCount{
					Count:           3,
					MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
					TargetSystem:    42,
					TargetComponent: 99,
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var coord *Coordinator
			coord, source := coordinatorFor(t, func(msg message.Message) error {
				if _, ok := msg.(*ardupilotmega.MessageMissionRequestList); ok {
					return coord.Publish(context.Background(), tc.reply(coord))
				}

				return nil
			})

			_, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
			if !errors.Is(err, ErrTimeout) {
				t.Errorf("error = %v, want ErrTimeout (%s)", err, tc.why)
			}

			// The timeout alone proves nothing: a wrongly accepted count also
			// ends in a timeout once the items fail to arrive. What separates
			// "ignored" from "accepted, then stalled" is whether the
			// coordinator ever acted on the count. Each foreign reply
			// advertises items, so a single MISSION_REQUEST_INT means the
			// response was believed.
			if got := source.requestedSeqs(); got != nil {
				t.Errorf("requested item sequences %v after a foreign count; "+
					"the response was accepted (%s)", got, tc.why)
			}

			if got := coord.inFlightCount(); got != 0 {
				t.Errorf("in-flight slots = %d, want 0", got)
			}
		})
	}
}

func TestDuplicateItemDoesNotCorruptTheSnapshot(t *testing.T) {
	t.Parallel()

	var coord *Coordinator
	coord, source := coordinatorFor(t, func(msg message.Message) error {
		ctx := context.Background()

		switch m := msg.(type) {
		case *ardupilotmega.MessageMissionRequestList:
			return coord.Publish(ctx, countFrom(vehicleUnderTest, 2))
		case *ardupilotmega.MessageMissionRequestInt:
			// Answer, then retransmit the same item — the shape a link with
			// duplicate delivery produces.
			if err := coord.Publish(ctx, itemFrom(vehicleUnderTest, uint32(m.Seq))); err != nil {
				return err
			}

			return coord.Publish(ctx, itemFrom(vehicleUnderTest, uint32(m.Seq)))
		}

		return nil
	})

	snap, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if len(snap.GetItems()) != 2 {
		t.Fatalf("items = %d, want 2 — a duplicate was stored", len(snap.GetItems()))
	}

	for i, item := range snap.GetItems() {
		if item.GetSeq() != uint32(i) {
			t.Errorf("items[%d].seq = %d, want %d", i, item.GetSeq(), i)
		}
	}

	if got := source.requestedSeqs(); len(got) != 2 {
		t.Errorf("requested sequences = %v, want each asked once", got)
	}
}

func TestOutOfRangeSequenceCannotCompleteADownload(t *testing.T) {
	t.Parallel()

	var coord *Coordinator
	coord, _ = coordinatorFor(t, func(msg message.Message) error {
		ctx := context.Background()

		switch msg.(type) {
		case *ardupilotmega.MessageMissionRequestList:
			return coord.Publish(ctx, countFrom(vehicleUnderTest, 2))
		case *ardupilotmega.MessageMissionRequestInt:
			// Advertised 2 items, answers with sequence 9.
			return coord.Publish(ctx, itemFrom(vehicleUnderTest, 9))
		}

		return nil
	})

	_, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("error = %v, want ErrTimeout", err)
	}
}

// --- terminal failures ------------------------------------------------------

func TestTerminalFailuresAreDistinctAndReleaseTheSlot(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("link down")

	tests := []struct {
		ctx   func() (context.Context, context.CancelFunc)
		write func(*Coordinator) func(message.Message) error
		want  error
		name  string
	}{
		{
			name: "timeout",
			want: ErrTimeout,
			write: func(*Coordinator) func(message.Message) error {
				return func(message.Message) error { return nil } // silent vehicle
			},
		},
		{
			name: "link write failure",
			want: ErrWriteFailed,
			write: func(*Coordinator) func(message.Message) error {
				return func(message.Message) error { return writeErr }
			},
		},
		{
			name: "negative mission ack",
			want: ErrRejected,
			write: func(c *Coordinator) func(message.Message) error {
				return func(msg message.Message) error {
					if _, ok := msg.(*ardupilotmega.MessageMissionRequestList); ok {
						return c.Publish(context.Background(), protocolEvent(
							vehicleUnderTest,
							&gcsv1.MissionAck{
								Result:          gcsv1.MavMissionResult_MAV_MISSION_DENIED,
								MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
								TargetSystem:    uint32(codec.GCSSystemID),
								TargetComponent: uint32(codec.GCSComponentID),
							}))
					}

					return nil
				}
			},
		},
		{
			name: "cancellation",
			want: ErrCancelled,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx, func() {}
			},
			write: func(*Coordinator) func(message.Message) error {
				return func(message.Message) error { return nil }
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var coord *Coordinator
			coord, _ = coordinatorFor(t, func(msg message.Message) error {
				return tc.write(coord)(msg)
			})

			ctx := context.Background()

			if tc.ctx != nil {
				var cancel context.CancelFunc
				ctx, cancel = tc.ctx()

				defer cancel()
			}

			_, err := coord.Download(ctx, vehicleUnderTest, codec.MissionTypeMission)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}

			// A terminal failure must not strand the slot, or the vehicle
			// becomes permanently un-downloadable without a restart.
			if got := coord.inFlightCount(); got != 0 {
				t.Errorf("in-flight slots = %d, want 0", got)
			}
		})
	}
}

func TestRejectionCarriesTheVehiclesReason(t *testing.T) {
	t.Parallel()

	var coord *Coordinator
	coord, _ = coordinatorFor(t, func(msg message.Message) error {
		if _, ok := msg.(*ardupilotmega.MessageMissionRequestList); ok {
			return coord.Publish(context.Background(), protocolEvent(
				vehicleUnderTest,
				&gcsv1.MissionAck{
					Result:          gcsv1.MavMissionResult_MAV_MISSION_UNSUPPORTED,
					MissionType:     gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
					TargetSystem:    uint32(codec.GCSSystemID),
					TargetComponent: uint32(codec.GCSComponentID),
				}))
		}

		return nil
	})

	_, err := coord.Download(context.Background(), vehicleUnderTest, codec.MissionTypeMission)

	var rejected *RejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("error = %v, want a *RejectedError", err)
	}

	if rejected.Result != gcsv1.MavMissionResult_MAV_MISSION_UNSUPPORTED {
		t.Errorf("result = %v, want MAV_MISSION_UNSUPPORTED", rejected.Result)
	}
}

func TestUnknownVehicleHasNoRoute(t *testing.T) {
	t.Parallel()

	coord, _ := coordinatorFor(t, func(message.Message) error { return nil })

	_, err := coord.Download(
		context.Background(),
		codec.Target{SystemID: 200, ComponentID: codec.AutopilotComponentID},
		codec.MissionTypeMission,
	)
	if !errors.Is(err, ErrNoRoute) {
		t.Errorf("error = %v, want ErrNoRoute", err)
	}
}

// --- concurrency ------------------------------------------------------------

// TestSeparateVehiclesDownloadConcurrently is the other half of the in-flight
// rule: the slot is per vehicle and mission type, so one vehicle's download
// must not exclude another's.
func TestSeparateVehiclesDownloadConcurrently(t *testing.T) {
	t.Parallel()

	first := codec.Target{SystemID: 1, ComponentID: codec.AutopilotComponentID}
	second := codec.Target{SystemID: 2, ComponentID: codec.AutopilotComponentID}

	// Neither download may proceed until both hold a slot, so the test fails
	// rather than deadlocks if the slots are not independent.
	both := make(chan struct{})
	var once sync.Once
	var held int
	var mu sync.Mutex

	var coord *Coordinator
	coord, _ = coordinatorFor(t, func(msg message.Message) error {
		ctx := context.Background()

		switch m := msg.(type) {
		case *ardupilotmega.MessageMissionRequestList:
			mu.Lock()
			held++
			if held == 2 {
				once.Do(func() { close(both) })
			}
			mu.Unlock()

			select {
			case <-both:
			case <-time.After(2 * time.Second):
				t.Error("both downloads never held a slot at once")
			}

			return coord.Publish(ctx, countFrom(
				codec.Target{SystemID: m.TargetSystem, ComponentID: m.TargetComponent}, 1))

		case *ardupilotmega.MessageMissionRequestInt:
			return coord.Publish(ctx, itemFrom(
				codec.Target{SystemID: m.TargetSystem, ComponentID: m.TargetComponent},
				uint32(m.Seq)))
		}

		return nil
	})

	type result struct {
		snap *gcsv1.MissionSnapshot
		err  error
	}

	results := make(chan result, 2)

	for _, target := range []codec.Target{first, second} {
		go func() {
			snap, err := coord.Download(context.Background(), target, codec.MissionTypeMission)
			results <- result{snap: snap, err: err}
		}()
	}

	seen := map[uint32]bool{}

	for range 2 {
		r := <-results
		if r.err != nil {
			t.Fatalf("Download: %v", r.err)
		}

		seen[r.snap.GetVehicleId().GetSystemId()] = true
	}

	if !seen[1] || !seen[2] {
		t.Errorf("downloaded systems = %v, want both 1 and 2", seen)
	}

	if got := coord.inFlightCount(); got != 0 {
		t.Errorf("in-flight slots = %d, want 0", got)
	}
}
