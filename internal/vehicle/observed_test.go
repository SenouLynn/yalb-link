package vehicle

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// observedAtOf reads a telemetry payload's observed_at without naming the
// payload type, so the assertion works for every oneof variant.
func observedAtOf(t *testing.T, evt *gcsv1.TelemetryEvent) *timestamppb.Timestamp {
	t.Helper()

	msg := evt.ProtoReflect()

	field := msg.WhichOneof(msg.Descriptor().Oneofs().ByName(telemetryPayloadOneof))
	if field == nil {
		t.Fatalf("telemetry event carries no payload")
	}

	payload := msg.Get(field).Message()

	stamp := payload.Descriptor().Fields().ByName(observedAtField)
	if stamp == nil {
		t.Fatalf("payload %s has no %s field", payload.Descriptor().FullName(), observedAtField)
	}

	if !payload.Has(stamp) {
		return nil
	}

	//nolint:forcetypeassert // observed_at is declared as google.protobuf.Timestamp.
	return payload.Get(stamp).Message().Interface().(*timestamppb.Timestamp)
}

// newTelemetryEventFor builds a zero-valued event for one oneof variant, so the
// stamping test covers the payload set as the schema defines it rather than as
// a hand-maintained list.
func newTelemetryEventFor(field protoreflect.FieldDescriptor) *gcsv1.TelemetryEvent {
	evt := &gcsv1.TelemetryEvent{}
	msg := evt.ProtoReflect()
	msg.Set(field, msg.NewField(field))

	return evt
}

// TestFoldStampsObservedAtOnEveryPayload walks the oneof from the descriptor so
// a new telemetry family cannot be added without stamping.
func TestFoldStampsObservedAtOnEveryPayload(t *testing.T) {
	oneof := (&gcsv1.TelemetryEvent{}).ProtoReflect().
		Descriptor().Oneofs().ByName(telemetryPayloadOneof)

	if oneof == nil {
		t.Fatalf("TelemetryEvent has no %q oneof", telemetryPayloadOneof)
	}

	if oneof.Fields().Len() == 0 {
		t.Fatalf("TelemetryEvent %q oneof is empty", telemetryPayloadOneof)
	}

	for i := range oneof.Fields().Len() {
		field := oneof.Fields().Get(i)

		t.Run(string(field.Name()), func(t *testing.T) {
			in := telem(newTelemetryEventFor(field), uint32(field.Number()))

			_, events := Fold(State{}, in, t0)

			var stamped *gcsv1.TelemetryEvent

			for _, ev := range events {
				if ev.Telemetry != nil {
					stamped = ev.Telemetry
				}
			}

			if stamped == nil {
				t.Fatalf("fold emitted no telemetry event")
			}

			got := observedAtOf(t, stamped)
			if got == nil {
				t.Fatalf("observed_at was not stamped")
			}

			if want := timestampOf(t0); got.AsTime() != want.AsTime() {
				t.Errorf("observed_at = %v, want %v", got.AsTime(), want.AsTime())
			}
		})
	}
}

// TestFoldDoesNotMutateInboundTelemetry pins the fold's no-mutation contract:
// stamping must land on the emitted copy, not on the caller's decoded event.
func TestFoldDoesNotMutateInboundTelemetry(t *testing.T) {
	input := &gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_Attitude{Attitude: &gcsv1.Attitude{RollRad: 0.5}},
	}

	_, events := Fold(State{}, telem(input, 30), t0)

	if got := observedAtOf(t, input); got != nil {
		t.Errorf("fold stamped the inbound event in place: observed_at = %v", got.AsTime())
	}

	if len(events) != 1 || events[0].Telemetry == nil {
		t.Fatalf("expected exactly one telemetry event, got %d events", len(events))
	}

	if events[0].Telemetry == input {
		t.Error("fold emitted the inbound event pointer; it must emit a stamped copy")
	}
}

// TestFoldForwardsProtocolEvents keeps decoded transaction responses reachable
// by the sinks instead of being dropped between the codec and the bridge.
func TestFoldForwardsProtocolEvents(t *testing.T) {
	ack := &gcsv1.ProtocolEvent{
		VehicleId: &gcsv1.VehicleId{SystemId: 1, ComponentId: 1},
		Payload: &gcsv1.ProtocolEvent_CommandAck{
			CommandAck: &gcsv1.CommandAck{Command: 511},
		},
	}

	in := Inbound{
		SrcAddr: "10.0.0.5:14550",
		Decoded: codec.Decoded{Transaction: ack, SysID: 1, CompID: 1},
		MsgID:   77,
	}

	_, events := Fold(State{}, in, t0)

	var got *gcsv1.ProtocolEvent

	for _, ev := range events {
		if ev.Protocol != nil {
			got = ev.Protocol
		}
	}

	if got == nil {
		t.Fatalf("fold dropped the protocol event; got %d events", len(events))
	}

	if got.GetCommandAck().GetCommand() != 511 {
		t.Errorf("command = %d, want 511", got.GetCommandAck().GetCommand())
	}
}

// TestFoldEmitsNoProtocolEventWithoutTransaction guards the common path: an
// ordinary telemetry frame must not produce an empty protocol event.
func TestFoldEmitsNoProtocolEventWithoutTransaction(t *testing.T) {
	in := telem(&gcsv1.TelemetryEvent{
		Payload: &gcsv1.TelemetryEvent_Attitude{Attitude: &gcsv1.Attitude{}},
	}, 30)

	_, events := Fold(State{}, in, t0)

	for _, ev := range events {
		if ev.Protocol != nil {
			t.Errorf("fold emitted a protocol event for a telemetry frame")
		}
	}
}
