package recording

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const maxConsecutiveWriteFailures = 5

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
	var batchTimer *time.Timer
	var batchTimerC <-chan time.Time
	var durationC <-chan time.Time
	var durationID int64
	var sweepC <-chan time.Time
	if s.sweepInterval > 0 {
		sweepC = s.after(s.sweepInterval)
	}

	stopBatchTimer := func() {
		if batchTimer != nil && !batchTimer.Stop() {
			select {
			case <-batchTimer.C:
			default:
			}
		}
		batchTimerC = nil
	}
	markTerminal := func(id int64, status, reason string) {
		terminal[id] = true
		ctx, cancel := s.operationContext(context.Background())
		defer cancel()
		if err := s.markStopped(ctx, id, status, reason); err != nil {
			s.log.Error("marking recording terminal", "recording_id", id, "status", status, "reason", reason, "err", err)
		}
		s.clearCurrent(id)
		if durationID == id {
			durationID = 0
			durationC = nil
		}
	}
	flush := func() {
		stopBatchTimer()
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
			ctx, cancel := s.operationContext(context.Background())
			write := s.writeBatch
			if s.writeBatchFn != nil {
				write = s.writeBatchFn
			}
			err := write(ctx, records)
			cancel()
			if err != nil {
				s.writeErrors.Add(1)
				consecutiveFailures[id]++
				s.log.Error("recording batch failed", "recording_id", id, "events", len(records), "err", err)
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
					consecutiveFailures[id] >= maxConsecutiveWriteFailures {
					markTerminal(id, "error", "writer_error")
				}
				continue
			}

			consecutiveFailures[id] = 0
			total := totals[id]
			if total == nil {
				total = &recordingTotals{started: s.now().UTC()}
				totals[id] = total
			}
			for _, record := range records {
				total.events++
				total.bytes += int64(len(record.payload))
			}
			if reason := s.limitReason(total); reason != "" {
				markTerminal(id, "limit_reached", reason)
				s.log.Info("recording limit reached", "recording_id", id, "reason", reason)
			}
		}
	}
	expireDuration := func() {
		id := durationID
		flush()
		if id != 0 && !terminal[id] {
			markTerminal(id, "limit_reached", "duration_limit")
			s.log.Info("recording limit reached", "recording_id", id, "reason", "duration_limit")
		}
	}

	for {
		// Give an elapsed duration deadline priority over a continuously ready
		// telemetry queue. Without this preflight, select could repeatedly choose
		// events while the deadline was also ready.
		if durationC != nil {
			select {
			case <-durationC:
				expireDuration()
				continue
			default:
			}
		}
		select {
		case item := <-s.items:
			if item.record != nil {
				if terminal[item.record.recordingID] {
					s.dropped.Add(1)
					continue
				}
				batch = append(batch, *item.record)
				if len(batch) == 1 {
					if batchTimer == nil {
						batchTimer = time.NewTimer(s.batchInterval)
					} else {
						batchTimer.Reset(s.batchInterval)
					}
					batchTimerC = batchTimer.C
				}
				if len(batch) >= s.batchSize {
					flush()
				}
				continue
			}

			command := item.command
			if command == nil {
				continue
			}
			if command.startID != 0 {
				if command.ctx.Err() != nil {
					markTerminal(command.startID, "error", "control_timeout")
				} else {
					totals[command.startID] = &recordingTotals{started: command.started}
					durationID = command.startID
					durationC = s.after(s.maxDuration)
				}
				close(command.done)
				continue
			}

			flush()
			if command.deleteID != 0 {
				ctx, cancel := s.operationContext(command.ctx)
				command.err = s.deleteRecording(ctx, command.deleteID)
				cancel()
				if command.err == nil {
					terminal[command.deleteID] = true
					delete(totals, command.deleteID)
					delete(consecutiveFailures, command.deleteID)
					if durationID == command.deleteID {
						durationID = 0
						durationC = nil
					}
				}
			}
			if command.stopID != 0 && !terminal[command.stopID] {
				ctx, cancel := s.operationContext(context.Background())
				command.err = s.markStopped(ctx, command.stopID, "stopped", command.reason)
				cancel()
				terminal[command.stopID] = true
				if durationID == command.stopID {
					durationID = 0
					durationC = nil
				}
			}
			close(command.done)
			if command.close {
				return
			}

		case <-batchTimerC:
			flush()

		case <-durationC:
			expireDuration()

		case <-sweepC:
			flush()
			ctx, cancel := s.operationContext(context.Background())
			if _, err := s.sweep(ctx); err != nil {
				s.log.Error("applying recording retention", "err", err)
			}
			cancel()
			sweepC = s.after(s.sweepInterval)
		}
	}
}

func (s *Store) writeBatch(ctx context.Context, records []writeRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit or the returned write error is authoritative.

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO recording_events
		(recording_id, seq, kind, occurred_at, payload) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // execution or commit errors are authoritative.

	for _, record := range records {
		if _, err := stmt.ExecContext(ctx, record.recordingID, record.seq,
			record.kind, record.occurredAt, record.payload); err != nil {
			return fmt.Errorf("insert seq %d: %w", record.seq, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) markStopped(ctx context.Context, id int64, status, reason string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE recordings
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
