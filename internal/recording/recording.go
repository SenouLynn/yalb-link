package recording

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// DefaultQueue bounds accepted events waiting for the writer.
	DefaultQueue = 1024
	// DefaultBatchSize is the maximum number of events in one transaction.
	DefaultBatchSize = 100
	// DefaultBatchInterval bounds how long the oldest accepted event waits.
	DefaultBatchInterval = 200 * time.Millisecond
	// DefaultMaxEvents caps events persisted in one recording.
	DefaultMaxEvents = int64(200_000)
	// DefaultMaxBytes caps protobuf payload bytes persisted in one recording.
	DefaultMaxBytes = int64(256 << 20)
	// DefaultMaxDuration caps the elapsed duration of one recording.
	DefaultMaxDuration = 30 * time.Minute
)

var (
	// ErrRecordingActive means a second recording cannot be started yet.
	ErrRecordingActive = errors.New("recording: a recording is already active")
	// ErrNoRecording means there is no active recording to stop.
	ErrNoRecording = errors.New("recording: no recording is active")
	// ErrClosed means the store can no longer accept lifecycle operations.
	ErrClosed = errors.New("recording: store is closed")
)

// Config controls a local SQLite recording store.
type Config struct {
	Path          string
	Log           *slog.Logger
	Now           func() time.Time
	Queue         int
	BatchSize     int
	BatchInterval time.Duration
	MaxEvents     int64
	MaxBytes      int64
	MaxDuration   time.Duration
}

// Recording describes one recording lifecycle and its persisted event count.
type Recording struct {
	StoppedAt  *time.Time `json:"stopped_at,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	ID         int64      `json:"id"`
	EventCount int64      `json:"event_count"`
}

// Stats reports process-lifetime loss and persistence counters.
type Stats struct {
	Dropped      int64 `json:"dropped"`
	WriteErrors  int64 `json:"write_errors"`
	EncodeErrors int64 `json:"encode_errors"`
}

type activeRecording struct {
	started time.Time
	id      int64
	nextSeq int64
}

type writeRecord struct {
	payload     []byte
	kind        string
	recordingID int64
	seq         int64
	occurredAt  int64
}

type writerCommand struct {
	stopID int64
	reason string
	done   chan error
	close  bool
}

type writerItem struct {
	record  *writeRecord
	command *writerCommand
}

// Store owns recording lifecycle, asynchronous writes, and replay reads.
type Store struct {
	db            *sql.DB
	log           *slog.Logger
	now           func() time.Time
	items         chan writerItem
	done          chan struct{}
	batchSize     int
	batchInterval time.Duration
	maxEvents     int64
	maxBytes      int64
	maxDuration   time.Duration
	lifecycleMu   sync.Mutex
	mu            sync.Mutex
	current       *activeRecording
	closed        bool
	dropped       atomic.Int64
	writeErrors   atomic.Int64
	encodeErrors  atomic.Int64
	writeBatchFn  func([]writeRecord) error
}

// Open migrates a SQLite database and starts its bounded writer.
func Open(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("recording: database path is required")
	}
	defaultConfig(&cfg)

	db, err := sql.Open("sqlite", sqliteDSN(cfg.Path))
	if err != nil {
		return nil, fmt.Errorf("recording: opening database: %w", err)
	}
	// WAL permits a replay reader and the serialized writer to use separate
	// connections without forcing either operation through one pool slot.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck // the connection error is more useful.
		return nil, fmt.Errorf("recording: configuring database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close() //nolint:errcheck // the migration error is more useful.
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `UPDATE recordings SET stopped_at = ?, status = 'error',
		stop_reason = 'process_restart' WHERE status = 'active'`, cfg.Now().UTC().UnixMilli()); err != nil {
		db.Close() //nolint:errcheck // the recovery error is more useful.
		return nil, fmt.Errorf("recording: recovering interrupted recordings: %w", err)
	}

	store := &Store{
		db: db, log: cfg.Log, now: cfg.Now,
		items: make(chan writerItem, cfg.Queue), done: make(chan struct{}),
		batchSize: cfg.BatchSize, batchInterval: cfg.BatchInterval,
		maxEvents: cfg.MaxEvents, maxBytes: cfg.MaxBytes, maxDuration: cfg.MaxDuration,
	}
	go store.runWriter()

	return store, nil
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_busy_timeout=5000&_foreign_keys=on&_journal_mode=wal&_synchronous=normal"
}

func defaultConfig(cfg *Config) {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Queue <= 0 {
		cfg.Queue = DefaultQueue
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.BatchInterval <= 0 {
		cfg.BatchInterval = DefaultBatchInterval
	}
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = DefaultMaxEvents
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = DefaultMaxDuration
	}
}

// StartRecording creates and activates one empty recording.
func (s *Store) StartRecording(ctx context.Context, name string) (Recording, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Recording{}, ErrClosed
	}
	if s.current != nil {
		s.mu.Unlock()
		return Recording{}, ErrRecordingActive
	}
	s.mu.Unlock()

	started := s.now().UTC()
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO recordings(name, started_at, status) VALUES (?, ?, 'active')",
		name, started.UnixMilli())
	if err != nil {
		return Recording{}, fmt.Errorf("recording: starting: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Recording{}, fmt.Errorf("recording: reading new id: %w", err)
	}

	s.mu.Lock()
	s.current = &activeRecording{id: id, started: started}
	s.mu.Unlock()

	return Recording{ID: id, Name: name, StartedAt: started, Status: "active"}, nil
}

// StopRecording stops the active recording after flushing every accepted event.
func (s *Store) StopRecording(ctx context.Context) (Recording, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Recording{}, ErrClosed
	}
	active := s.current
	if active == nil {
		s.mu.Unlock()
		return Recording{}, ErrNoRecording
	}
	s.current = nil
	s.mu.Unlock()

	command := &writerCommand{stopID: active.id, reason: "requested", done: make(chan error, 1)}
	s.items <- writerItem{command: command}
	select {
	case err := <-command.done:
		if err != nil {
			return Recording{}, fmt.Errorf("recording: stopping writer: %w", err)
		}
	case <-ctx.Done():
		return Recording{}, fmt.Errorf("recording: stopping: %w", ctx.Err())
	}

	return s.recording(ctx, active.id)
}

// ListRecordings returns all lifecycle records in creation order.
func (s *Store) ListRecordings(ctx context.Context) ([]Recording, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.name, r.started_at, r.stopped_at,
		r.status, COALESCE(r.stop_reason, ''), COUNT(e.seq)
		FROM recordings r LEFT JOIN recording_events e ON e.recording_id = r.id
		GROUP BY r.id ORDER BY r.id`)
	if err != nil {
		return nil, fmt.Errorf("recording: listing: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err reports iteration failures.

	var out []Recording
	for rows.Next() {
		var rec Recording
		var started int64
		var stopped sql.NullInt64
		if err := rows.Scan(&rec.ID, &rec.Name, &started, &stopped, &rec.Status, &rec.StopReason, &rec.EventCount); err != nil {
			return nil, fmt.Errorf("recording: scanning list: %w", err)
		}
		rec.StartedAt = time.UnixMilli(started).UTC()
		if stopped.Valid {
			value := time.UnixMilli(stopped.Int64).UTC()
			rec.StoppedAt = &value
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recording: iterating list: %w", err)
	}
	return out, nil
}

func (s *Store) recording(ctx context.Context, id int64) (Recording, error) {
	var rec Recording
	var started int64
	var stopped sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT r.id, r.name, r.started_at, r.stopped_at,
		r.status, COALESCE(r.stop_reason, ''), COUNT(e.seq)
		FROM recordings r LEFT JOIN recording_events e ON e.recording_id = r.id WHERE r.id = ? GROUP BY r.id`, id).
		Scan(&rec.ID, &rec.Name, &started, &stopped, &rec.Status, &rec.StopReason, &rec.EventCount)
	if err != nil {
		return Recording{}, fmt.Errorf("recording: reading %d: %w", id, err)
	}
	rec.StartedAt = time.UnixMilli(started).UTC()
	if stopped.Valid {
		value := time.UnixMilli(stopped.Int64).UTC()
		rec.StoppedAt = &value
	}
	return rec, nil
}

// Stats returns a race-safe snapshot of process-lifetime writer counters.
func (s *Store) Stats() Stats {
	return Stats{Dropped: s.dropped.Load(), WriteErrors: s.writeErrors.Load(), EncodeErrors: s.encodeErrors.Load()}
}

// Close flushes accepted events, stops an active recording, and closes SQLite.
func (s *Store) Close() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	active := s.current
	s.current = nil
	s.mu.Unlock()

	command := &writerCommand{close: true, done: make(chan error, 1)}
	if active != nil {
		command.stopID = active.id
		command.reason = "shutdown"
	}
	s.items <- writerItem{command: command}
	writerErr := <-command.done
	<-s.done
	closeErr := s.db.Close()
	if writerErr != nil {
		return fmt.Errorf("recording: stopping writer during close: %w", writerErr)
	}
	if closeErr != nil {
		return fmt.Errorf("recording: closing database: %w", closeErr)
	}
	return nil
}
