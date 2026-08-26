package recording

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const deleteChunkSize = 5000

type retentionCandidate struct {
	startedAt time.Time
	id        int64
}

func (s *Store) deleteRecording(ctx context.Context, id int64) error {
	for {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("recording: beginning delete of %d: %w", id, err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM recording_events
			WHERE recording_id = ? AND seq IN (
				SELECT seq FROM recording_events WHERE recording_id = ? ORDER BY seq LIMIT ?
			)`, id, id, deleteChunkSize)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("recording: deleting events for %d: %w", id, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("recording: counting deleted events for %d: %w", id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("recording: committing event delete for %d: %w", id, err)
		}
		if rows == 0 {
			break
		}
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM recordings WHERE id = ? AND status <> 'active'", id)
	if err != nil {
		return fmt.Errorf("recording: deleting %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("recording: counting deleted recording %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %d", ErrRecordingNotFound, id)
	}
	return nil
}

func (s *Store) liveBytes(ctx context.Context) (int64, error) {
	var pageCount, freelistCount, pageSize int64
	for query, destination := range map[string]*int64{
		"PRAGMA page_count":     &pageCount,
		"PRAGMA freelist_count": &freelistCount,
		"PRAGMA page_size":      &pageSize,
	} {
		if err := s.db.QueryRowContext(ctx, query).Scan(destination); err != nil {
			return 0, fmt.Errorf("recording: reading SQLite size (%s): %w", query, err)
		}
	}
	return (pageCount - freelistCount) * pageSize, nil
}

func (s *Store) retentionCandidates(ctx context.Context) ([]retentionCandidate, int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, started_at FROM recordings
		WHERE status <> 'active' ORDER BY started_at, id`)
	if err != nil {
		return nil, 0, fmt.Errorf("recording: selecting retention candidates: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var candidates []retentionCandidate
	for rows.Next() {
		var candidate retentionCandidate
		var started int64
		if err := rows.Scan(&candidate.id, &started); err != nil {
			return nil, 0, fmt.Errorf("recording: scanning retention candidate: %w", err)
		}
		candidate.startedAt = time.UnixMilli(started).UTC()
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("recording: iterating retention candidates: %w", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM recordings").Scan(&count); err != nil {
		return nil, 0, fmt.Errorf("recording: counting recordings: %w", err)
	}
	return candidates, count, nil
}

// sweep deletes oldest stopped recordings until every configured bound holds.
// It runs only before the writer starts or from within the writer goroutine.
func (s *Store) sweep(ctx context.Context) (int, error) {
	deleted := 0
	for {
		candidates, count, err := s.retentionCandidates(ctx)
		if err != nil {
			return deleted, err
		}
		if len(candidates) == 0 {
			return deleted, nil
		}
		var reasons []string
		oldest := candidates[0]
		if s.maxTotalAge > 0 && !oldest.startedAt.After(s.now().UTC().Add(-s.maxTotalAge)) {
			reasons = append(reasons, "age")
		}
		if s.maxRecordings > 0 && count > s.maxRecordings {
			reasons = append(reasons, "count")
		}
		if s.maxTotalBytes > 0 {
			bytes, err := s.liveBytes(ctx)
			if err != nil {
				return deleted, err
			}
			if bytes > s.maxTotalBytes {
				reasons = append(reasons, "size")
			}
		}
		if len(reasons) == 0 {
			return deleted, nil
		}
		if err := s.deleteRecording(ctx, oldest.id); err != nil {
			return deleted, err
		}
		deleted++
		s.log.Info("recording removed by retention", "recording_id", oldest.id, "bounds", strings.Join(reasons, ","))
	}
}
