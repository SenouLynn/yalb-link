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
	// DefaultOperationTimeout bounds database and lifecycle control operations.
	DefaultOperationTimeout = 5 * time.Second
	// DefaultShutdownTimeout bounds Store.Close, including writer drain.
	DefaultShutdownTimeout = 10 * time.Second
	// DefaultPageSize is how many events one replay request returns unasked.
	DefaultPageSize = 2000
	// MaxPageSize caps one replay request, so a client cannot ask the server to
	// decode and hold an entire 200,000-event recording in a single response.
	MaxPageSize = 10000
)

// The persisted event kinds, matching the migration's CHECK constraint.
const (
	KindFleet     = "fleet"
	KindTelemetry = "telemetry"
)

var (
	// ErrRecordingActive means a second recording cannot be started yet.
	ErrRecordingActive = errors.New("recording: a recording is already active")
	// ErrNoRecording means there is no active recording to stop.
	ErrNoRecording = errors.New("recording: no recording is active")
	// ErrClosed means the store can no longer accept lifecycle operations.
	ErrClosed = errors.New("recording: store is closed")
	// ErrRecordingNotFound means no recording exists with the requested id.
	ErrRecordingNotFound = errors.New("recording: no such recording")
)

// Config controls a local SQLite recording store.
type Config struct {
	Path             string
	Log              *slog.Logger
	Now              func() time.Time
	Queue            int
	BatchSize        int
	BatchInterval    time.Duration
	MaxEvents        int64
	MaxBytes         int64
	MaxDuration      time.Duration
	OperationTimeout time.Duration
	ShutdownTimeout  time.Duration
	After            func(time.Duration) <-chan time.Time
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
	ctx     context.Context
	started time.Time
	startID int64
	stopID  int64
	reason  string
	done    chan struct{}
	err     error
	close   bool
}

type writerItem struct {
	record  *writeRecord
	command *writerCommand
}

// Store owns recording lifecycle, asynchronous writes, and replay reads.
type Store struct {
	db               *sql.DB
	log              *slog.Logger
	now              func() time.Time
	items            chan writerItem
	done             chan struct{}
	batchSize        int
	batchInterval    time.Duration
	maxEvents        int64
	maxBytes         int64
	maxDuration      time.Duration
	operationTimeout time.Duration
	shutdownTimeout  time.Duration
	after            func(time.Duration) <-chan time.Time
	lifecycle        chan struct{}
	mu               sync.Mutex
	current          *activeRecording
	closed           bool
	closeCommand     *writerCommand
	closeSent        bool
	closeComplete    bool
	closeDBOnce      sync.Once
	closeDone        chan struct{}
	closeErr         error
	dropped          atomic.Int64
	writeErrors      atomic.Int64
	encodeErrors     atomic.Int64
	writeBatchFn     func(context.Context, []writeRecord) error
}

// Open migrates a SQLite database and starts its bounded writer.
func Open(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("recording: database path is required")
	}
	defaultConfig(&cfg)

	db, err := sql.Open("sqlite", sqliteDSN(cfg.Path, cfg.OperationTimeout))
	if err != nil {
		return nil, fmt.Errorf("recording: opening database: %w", err)
	}
	// WAL permits a replay reader and the serialized writer to use separate
	// connections without forcing either operation through one pool slot.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
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
		items: make(chan writerItem, cfg.Queue), done: make(chan struct{}), closeDone: make(chan struct{}),
		batchSize: cfg.BatchSize, batchInterval: cfg.BatchInterval,
		maxEvents: cfg.MaxEvents, maxBytes: cfg.MaxBytes, maxDuration: cfg.MaxDuration,
		operationTimeout: cfg.OperationTimeout, shutdownTimeout: cfg.ShutdownTimeout, after: cfg.After,
		lifecycle: make(chan struct{}, 1),
	}
	store.lifecycle <- struct{}{}
	go store.runWriter()

	return store, nil
}

func sqliteDSN(path string, operationTimeout time.Duration) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	busyMillis := max(operationTimeout.Milliseconds(), 1)
	return fmt.Sprintf("%s%s_busy_timeout=%d&_foreign_keys=on&_journal_mode=wal&_synchronous=normal",
		path, separator, busyMillis)
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
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = DefaultOperationTimeout
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.After == nil {
		cfg.After = time.After
	}
}

func (s *Store) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, s.operationTimeout)
}

func (s *Store) acquireLifecycle(ctx context.Context) error {
	select {
	case <-s.lifecycle:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("recording: waiting for lifecycle operation: %w", ctx.Err())
	}
}

func (s *Store) releaseLifecycle() { s.lifecycle <- struct{}{} }

func (s *Store) sendCommand(ctx context.Context, command *writerCommand) error {
	select {
	case s.items <- writerItem{command: command}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("recording: queueing writer command: %w", ctx.Err())
	}
}

func waitCommand(ctx context.Context, command *writerCommand) error {
	select {
	case <-command.done:
		return command.err
	case <-ctx.Done():
		return fmt.Errorf("recording: waiting for writer command: %w", ctx.Err())
	}
}

// StartRecording creates and activates one empty recording.
func (s *Store) StartRecording(ctx context.Context, name string) (Recording, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.acquireLifecycle(opCtx); err != nil {
		return Recording{}, err
	}
	defer s.releaseLifecycle()

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
	result, err := s.db.ExecContext(opCtx,
		"INSERT INTO recordings(name, started_at, status) VALUES (?, ?, 'active')",
		name, started.UnixMilli())
	if err != nil {
		return Recording{}, fmt.Errorf("recording: starting: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Recording{}, fmt.Errorf("recording: reading new id: %w", err)
	}
	command := &writerCommand{ctx: opCtx, startID: id, started: started, done: make(chan struct{})}
	if err := s.sendCommand(opCtx, command); err != nil {
		s.markStartFailure(opCtx, id)
		return Recording{}, err
	}
	if err := waitCommand(opCtx, command); err != nil {
		s.markStartFailure(opCtx, id)
		return Recording{}, fmt.Errorf("recording: starting writer: %w", err)
	}

	s.mu.Lock()
	s.current = &activeRecording{id: id}
	s.mu.Unlock()

	return Recording{ID: id, Name: name, StartedAt: started, Status: "active"}, nil
}

// StopRecording stops the active recording after flushing every accepted event.
func (s *Store) StopRecording(ctx context.Context) (Recording, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.acquireLifecycle(opCtx); err != nil {
		return Recording{}, err
	}
	defer s.releaseLifecycle()

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

	command := &writerCommand{stopID: active.id, reason: "requested", done: make(chan struct{})}
	if err := s.sendCommand(opCtx, command); err != nil {
		s.mu.Lock()
		if !s.closed && s.current == nil {
			s.current = active
		}
		s.mu.Unlock()
		return Recording{}, err
	}
	if err := waitCommand(opCtx, command); err != nil {
		return Recording{}, fmt.Errorf("recording: stopping writer: %w", err)
	}

	return s.recording(opCtx, active.id)
}

// ListRecordings returns all lifecycle records in creation order.
func (s *Store) ListRecordings(ctx context.Context) ([]Recording, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()
	rows, err := s.db.QueryContext(opCtx, `SELECT r.id, r.name, r.started_at, r.stopped_at,
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

// GetRecording returns one recording's lifecycle metadata.
func (s *Store) GetRecording(ctx context.Context, id int64) (Recording, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()

	rec, err := s.recording(opCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Recording{}, fmt.Errorf("%w: %d", ErrRecordingNotFound, id)
	}

	return rec, err
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
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// Shutdown flushes accepted events and closes the store within ctx's deadline.
func (s *Store) Shutdown(ctx context.Context) error {
	if err := s.acquireLifecycle(ctx); err != nil {
		return err
	}
	defer s.releaseLifecycle()

	s.mu.Lock()
	if s.closeComplete {
		s.mu.Unlock()
		return nil
	}
	if s.closeCommand == nil {
		s.closed = true
		active := s.current
		s.current = nil
		s.closeCommand = &writerCommand{close: true, done: make(chan struct{})}
		if active != nil {
			s.closeCommand.stopID = active.id
			s.closeCommand.reason = "shutdown"
		}
	}
	command := s.closeCommand
	sent := s.closeSent
	s.mu.Unlock()

	if !sent {
		if err := s.sendCommand(ctx, command); err != nil {
			return err
		}
		s.mu.Lock()
		s.closeSent = true
		s.mu.Unlock()
	}
	writerErr := waitCommand(ctx, command)
	if writerErr != nil && ctx.Err() != nil {
		return fmt.Errorf("recording: stopping writer during shutdown: %w", writerErr)
	}
	select {
	case <-s.done:
	case <-ctx.Done():
		return fmt.Errorf("recording: waiting for writer shutdown: %w", ctx.Err())
	}

	s.closeDBOnce.Do(func() {
		go func() {
			s.closeErr = s.db.Close()
			close(s.closeDone)
		}()
	})
	select {
	case <-s.closeDone:
		if s.closeErr != nil {
			return fmt.Errorf("recording: closing database: %w", s.closeErr)
		}
		s.mu.Lock()
		s.closeComplete = true
		s.mu.Unlock()
		if writerErr != nil {
			return fmt.Errorf("recording: finalizing active recording during shutdown: %w", writerErr)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("recording: waiting for database close: %w", ctx.Err())
	}
}

func (s *Store) markStartFailure(ctx context.Context, id int64) {
	if err := s.markStopped(ctx, id, "error", "control_timeout"); err != nil {
		s.log.Error("marking failed recording start", "recording_id", id, "err", err)
	}
}
