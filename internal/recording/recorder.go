package recording

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"yalb.gcs/internal/bridge"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

// Recorder adapts a Store to the non-blocking bridge sink contract.
type Recorder struct{ Store *Store }

var _ bridge.Sink = (*Recorder)(nil)

// Publish snapshots a persistable event and attempts a non-blocking enqueue.
func (r *Recorder) Publish(ctx context.Context, event vehicle.Event) error {
	if r == nil || r.Store == nil {
		return nil
	}

	kind, message, occurred, err := persistable(event)
	if err != nil {
		r.Store.encodeErrors.Add(1)
		r.Store.log.ErrorContext(ctx, "recording event rejected", "err", err)
		return nil
	}
	if message == nil {
		return nil
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		r.Store.encodeErrors.Add(1)
		r.Store.log.ErrorContext(ctx, "recording event encode failed", "kind", kind, "err", err)
		return nil
	}

	r.Store.enqueue(writeRecord{kind: kind, occurredAt: occurred.UnixMilli(), payload: payload})
	return nil
}

func (s *Store) enqueue(record writeRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || s.closed {
		return
	}
	s.current.nextSeq++
	record.recordingID = s.current.id
	record.seq = s.current.nextSeq
	select {
	case s.items <- writerItem{record: &record}:
	default:
		s.dropped.Add(1)
	}
}

func persistable(event vehicle.Event) (string, proto.Message, time.Time, error) {
	switch {
	case event.Fleet != nil:
		timestamp := event.Fleet.GetOccurredAt()
		if timestamp == nil || !timestamp.IsValid() {
			return "", nil, time.Time{}, errors.New("recording: fleet event has an invalid occurred_at timestamp")
		}
		return KindFleet, event.Fleet, timestamp.AsTime().UTC(), nil
	case event.Telemetry != nil:
		timestamp, err := telemetryTimestamp(event.Telemetry)
		if err != nil {
			return "", nil, time.Time{}, err
		}
		return KindTelemetry, event.Telemetry, timestamp, nil
	default:
		return "", nil, time.Time{}, nil
	}
}

func telemetryTimestamp(event *gcsv1.TelemetryEvent) (time.Time, error) {
	message := event.ProtoReflect()
	oneof := message.Descriptor().Oneofs().ByName("payload")
	field := message.WhichOneof(oneof)
	if field == nil {
		return time.Time{}, errors.New("recording: telemetry payload is not set")
	}
	payload := message.Get(field).Message()
	timestampField := payload.Descriptor().Fields().ByName("observed_at")
	if timestampField == nil || timestampField.Kind() != protoreflect.MessageKind {
		return time.Time{}, errors.New("recording: telemetry payload has no observed_at timestamp")
	}
	timestamp, ok := payload.Get(timestampField).Message().Interface().(*timestamppb.Timestamp)
	if !ok || timestamp == nil || !timestamp.IsValid() {
		return time.Time{}, errors.New("recording: telemetry payload has an invalid observed_at timestamp")
	}
	return timestamp.AsTime().UTC(), nil
}
