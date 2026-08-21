package recording

import (
	"context"
	"fmt"
	"time"
)

type recordingTotals struct {
	started time.Time
	events  int64
	bytes   int64
}

func (s *Store) runWriter() { //nolint:funlen,gocognit,cyclop // A single owner keeps batching and terminal transitions ordered.
	defer close(s.done)

	batch := make([]writeRecord, 0, s.batchSize)
	totals := make(map[int64]*recordingTotals)
	terminal := make(map[int64]bool)
	consecutiveFailures := make(map[int64]int)
	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
	}
	flush := func() {
		stopTimer()
		if len(batch) == 0 {
			return
		}

		byID := make(map[int64][]writeRecord)
		for _, record := range batch {
			if !terminal[record.recordingID] {
				byID[record.recordingID] = append(byID[record.recordingID], record)
			}
		}
		batch = batch[:0]

		for id, records := range byID {
			write := s.writeBatch
			if s.writeBatchFn != nil {
				write = s.writeBatchFn
			}
			if err := write(records); err != nil {
				s.writeErrors.Add(1)
				consecutiveFailures[id]++
				s.log.Error("recording batch failed", "recording_id", id, "events", len(records), "err", err)
				if consecutiveFailures[id] >= 5 {
					terminal[id] = true
					if markErr := s.markStopped(id, "error", "writer_error"); markErr != nil {
						s.log.Error("marking recording failed", "recording_id", id, "err", markErr)
					}
					s.clearCurrent(id)
				}
				continue
			}

			consecutiveFailures[id] = 0
			total := totals[id]
			if total == nil {
				total = &recordingTotals{started: s.startedAt(id)}
				totals[id] = total
			}
			for _, record := range records {
				total.events++
				total.bytes += int64(len(record.payload))
			}
			if reason := s.limitReason(total); reason != "" {
				terminal[id] = true
				if err := s.markStopped(id, "limit_reached", reason); err != nil {
					s.log.Error("marking recording limit failed", "recording_id", id, "err", err)
				} else {
					s.log.Info("recording limit reached", "recording_id", id, "reason", reason)
				}
				s.clearCurrent(id)
			}
		}
	}

	for {
		select {
		case item := <-s.items:
			if item.record != nil {
				if terminal[item.record.recordingID] {
					s.dropped.Add(1)
					continue
				}
				batch = append(batch, *item.record)
				if len(batch) == 1 {
					if timer == nil {
						timer = time.NewTimer(s.batchInterval)
					} else {
						timer.Reset(s.batchInterval)
					}
					timerC = timer.C
				}
				if len(batch) >= s.batchSize {
					flush()
				}
				continue
			}

			if item.command != nil {
				flush()
				var err error
				if item.command.stopID != 0 && !terminal[item.command.stopID] {
					err = s.markStopped(item.command.stopID, "stopped", item.command.reason)
					terminal[item.command.stopID] = true
				}
				item.command.done <- err
				if item.command.close {
					return
				}
			}

		case <-timerC:
			flush()
		}
	}
}

func (s *Store) writeBatch(records []writeRecord) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit or the returned write error is authoritative.

	stmt, err := tx.PrepareContext(context.Background(), `INSERT INTO recording_events
		(recording_id, seq, kind, occurred_at, payload) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // execution or commit errors are authoritative.

	for _, record := range records {
		if _, err := stmt.ExecContext(context.Background(), record.recordingID, record.seq,
			record.kind, record.occurredAt, record.payload); err != nil {
			return fmt.Errorf("insert seq %d: %w", record.seq, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) markStopped(id int64, status, reason string) error {
	result, err := s.db.ExecContext(context.Background(), `UPDATE recordings
		SET stopped_at = ?, status = ?, stop_reason = ? WHERE id = ? AND status = 'active'`,
		s.now().UTC().UnixMilli(), status, reason, id)
	if err != nil {
		return fmt.Errorf("recording: marking %d %s: %w", id, status, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("recording: checking stop of %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("recording: active recording %d not found", id)
	}
	return nil
}

func (s *Store) clearCurrent(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && s.current.id == id {
		s.current = nil
	}
}

func (s *Store) startedAt(id int64) time.Time {
	var millis int64
	if err := s.db.QueryRowContext(context.Background(), "SELECT started_at FROM recordings WHERE id = ?", id).Scan(&millis); err != nil {
		s.log.Error("reading recording start time", "recording_id", id, "err", err)
		return s.now().UTC()
	}
	return time.UnixMilli(millis).UTC()
}

func (s *Store) limitReason(total *recordingTotals) string {
	if total.events >= s.maxEvents {
		return "event_limit"
	}
	if total.bytes >= s.maxBytes {
		return "size_limit"
	}
	if s.now().Sub(total.started) >= s.maxDuration {
		return "duration_limit"
	}
	return ""
}
