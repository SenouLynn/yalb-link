package recording

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// ReplayEvent is one persisted protobuf event in recording sequence order.
type ReplayEvent struct {
	Fleet      *gcsv1.FleetEvent
	Telemetry  *gcsv1.TelemetryEvent
	OccurredAt time.Time
	Seq        int64
}

// Replay loads and decodes all persisted events for one recording.
func (s *Store) Replay(ctx context.Context, recordingID int64) ([]ReplayEvent, error) {
	return s.ReplayFrom(ctx, recordingID, 0, 0)
}

// ReplayFrom loads one page of persisted events starting at fromSeq inclusive.
//
// A limit of zero or less is unbounded. Paging is by sequence number rather
// than by offset because sequence numbers are dense, immutable, and already the
// recording's ordering key: an offset would shift under a writer still
// committing batches for an active recording.
func (s *Store) ReplayFrom(ctx context.Context, recordingID, fromSeq int64, limit int) ([]ReplayEvent, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()

	// SQLite reads a negative LIMIT as unbounded, so one query serves both.
	bound := -1
	if limit > 0 {
		bound = limit
	}

	rows, err := s.db.QueryContext(opCtx, `SELECT seq, kind, occurred_at, payload
		FROM recording_events WHERE recording_id = ? AND seq >= ? ORDER BY seq LIMIT ?`,
		recordingID, fromSeq, bound)
	if err != nil {
		return nil, fmt.Errorf("recording: replay query: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err reports iteration failures.

	var out []ReplayEvent
	for rows.Next() {
		var event ReplayEvent
		var kind string
		var occurred int64
		var payload []byte
		if err := rows.Scan(&event.Seq, &kind, &occurred, &payload); err != nil {
			return nil, fmt.Errorf("recording: scanning replay: %w", err)
		}
		event.OccurredAt = time.UnixMilli(occurred).UTC()
		switch kind {
		case KindFleet:
			event.Fleet = &gcsv1.FleetEvent{}
			if err := proto.Unmarshal(payload, event.Fleet); err != nil {
				return nil, fmt.Errorf("recording: decoding fleet seq %d: %w", event.Seq, err)
			}
		case KindTelemetry:
			event.Telemetry = &gcsv1.TelemetryEvent{}
			if err := proto.Unmarshal(payload, event.Telemetry); err != nil {
				return nil, fmt.Errorf("recording: decoding telemetry seq %d: %w", event.Seq, err)
			}
		default:
			return nil, fmt.Errorf("recording: unknown event kind %q at seq %d", kind, event.Seq)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recording: iterating replay: %w", err)
	}
	return out, nil
}
