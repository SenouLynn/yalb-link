package command

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

const testEpoch = "test-epoch"

// testClock is the registry's injected clock. Quarantine is measured on it, so
// tests move time explicitly rather than sleeping.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock { return &testClock{t: time.Now().UTC()} }

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type resolveFixture struct {
	registry  *Registry
	published *capture
	clock     *testClock
	table     *routes.Table
}

// freshen re-stamps the route at the current test time, standing in for the
// heartbeats that keep a real link reachable while the clock advances.
func (f *resolveFixture) freshen() {
	f.table.Upsert(
		routes.Entry{Key: routes.Key{SysID: 1, CompID: codec.AutopilotComponentID}, Link: "test"},
		f.clock.now().UnixMilli(),
	)
}

func resolveFixtureFor(t *testing.T, publisher Publisher, write func(message.Message) error) *resolveFixture {
	t.Helper()
	clock := newTestClock()
	f := &resolveFixture{table: routes.NewTable(), clock: clock}
	if c, ok := publisher.(*capture); ok {
		f.published = c
	}
	f.registry = &Registry{
		Source: &fakeSource{write: write}, Routes: f.table, Publisher: publisher,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: clock.now, Timeout: 30 * time.Millisecond,
		Quarantine: 5 * time.Second, Epoch: testEpoch,
	}
	f.freshen()
	return f
}

// silentWrite delivers no ACK, so the command times out and poisons its key.
func silentWrite(message.Message) error { return nil }

// timedOutArm drives one arm request to a timeout and returns its transaction.
func timedOutArm(t *testing.T, f *resolveFixture) *gcsv1.CommandTransaction {
	t.Helper()
	tx, err := f.registry.Arm(context.Background(), 1, true)
	if err != nil {
		t.Fatalf("Arm() error = %v", err)
	}
	if tx.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Fatalf("state = %v, want TIMED_OUT", tx.GetState())
	}
	return tx
}

func TestArmStampsRequestedStateLabelAndEpoch(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	tx := timedOutArm(t, f)

	if tx.GetRequestedState() != gcsv1.ArmState_ARM_STATE_ARMED {
		t.Errorf("requested_state = %v, want ARMED", tx.GetRequestedState())
	}
	if tx.GetOperatorLabel() != OperatorLabel {
		t.Errorf("operator_label = %q, want %q", tx.GetOperatorLabel(), OperatorLabel)
	}
	if tx.GetRegistryEpoch() != testEpoch {
		t.Errorf("registry_epoch = %q, want %q", tx.GetRegistryEpoch(), testEpoch)
	}
}

func TestDisarmRequestIsDistinguishableFromArm(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	tx, err := f.registry.Arm(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	// MAV_CMD 400 alone cannot say which was asked for.
	if tx.GetRequestedState() != gcsv1.ArmState_ARM_STATE_DISARMED {
		t.Fatalf("requested_state = %v, want DISARMED", tx.GetRequestedState())
	}
}

func TestPoisonedArmReturnsRetainedTransaction(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)

	retained, err := f.registry.Arm(context.Background(), 1, false)
	if !errors.Is(err, ErrPoisoned) {
		t.Fatalf("error = %v, want ErrPoisoned", err)
	}
	// Without the retained snapshot a reloaded browser cannot name the
	// ambiguity it must resolve.
	if retained == nil {
		t.Fatal("poisoned arm returned no transaction")
	}
	if retained.GetId() != first.GetId() || retained.GetRegistryEpoch() != testEpoch {
		t.Fatalf("retained ref = (%d,%q), want (%d,%q)",
			retained.GetId(), retained.GetRegistryEpoch(), first.GetId(), testEpoch)
	}
	if retained.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Fatalf("retained state = %v", retained.GetState())
	}
}

func TestUncertainWriteFailureRetainsItsTransaction(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, func(message.Message) error { return errors.New("I/O uncertain") })
	first, err := f.registry.Arm(context.Background(), 1, true)
	if !errors.Is(err, ErrWriteFailed) {
		t.Fatalf("error = %v", err)
	}
	retained, err := f.registry.Arm(context.Background(), 1, true)
	if !errors.Is(err, ErrPoisoned) || retained.GetId() != first.GetId() {
		t.Fatalf("retained = %v, err = %v", retained, err)
	}
	if retained.GetState() != gcsv1.CommandState_COMMAND_STATE_SEND_FAILED {
		t.Fatalf("retained state = %v, want SEND_FAILED", retained.GetState())
	}
}

func TestResolveAttestsAndQuarantines(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)
	before := len(f.published.txs)

	out, err := f.registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_DISARMED)
	if err != nil {
		t.Fatalf("ResolveArm() error = %v", err)
	}

	// The ambiguity stays a historical fact; the attestation sits beside it.
	if out.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Errorf("state = %v, want TIMED_OUT to be preserved", out.GetState())
	}
	res := out.GetResolution()
	if res.GetObservedState() != gcsv1.ArmState_ARM_STATE_DISARMED {
		t.Errorf("observed_state = %v", res.GetObservedState())
	}
	if res.GetOperatorLabel() != OperatorLabel {
		t.Errorf("operator_label = %q", res.GetOperatorLabel())
	}
	if got, want := res.GetQuarantineUntil().AsTime(), f.clock.now().Add(5*time.Second); !got.Equal(want) {
		t.Errorf("quarantine_until = %v, want %v", got, want)
	}
	if got := len(f.published.txs) - before; got != 1 {
		t.Errorf("published %d snapshots for the resolution, want 1", got)
	}

	_, err = f.registry.Arm(context.Background(), 1, true)
	var quarantined *QuarantineError
	if !errors.As(err, &quarantined) || !errors.Is(err, ErrQuarantined) {
		t.Fatalf("post-resolution arm error = %v, want QuarantineError", err)
	}
	if quarantined.RetryAfter != 5*time.Second {
		t.Errorf("retry after = %v, want 5s", quarantined.RetryAfter)
	}
}

func TestResolveRejectsMismatchedReferences(t *testing.T) {
	tests := []struct {
		name  string
		epoch string
		delta uint32
		want  error
	}{
		{name: "stale epoch", epoch: "other-process", want: ErrStaleRef},
		{name: "stale transaction id", epoch: testEpoch, delta: 1, want: ErrStaleRef},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := resolveFixtureFor(t, &capture{}, silentWrite)
			first := timedOutArm(t, f)

			_, err := f.registry.ResolveArm(
				context.Background(), 1, tc.epoch, first.GetId()+tc.delta, gcsv1.ArmState_ARM_STATE_DISARMED)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			// Nothing changed: the key is still poisoned, not quarantined.
			if _, err := f.registry.Arm(context.Background(), 1, true); !errors.Is(err, ErrPoisoned) {
				t.Fatalf("key no longer poisoned: %v", err)
			}
		})
	}
}

func TestResolveRejectsUnpoisonedKey(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	_, err := f.registry.ResolveArm(context.Background(), 1, testEpoch, 1, gcsv1.ArmState_ARM_STATE_DISARMED)
	if !errors.Is(err, ErrNotPoisoned) {
		t.Fatalf("error = %v, want ErrNotPoisoned", err)
	}
}

func TestResolveRejectsUnspecifiedObservedState(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)

	// An attestation the operator never made must not be accepted as one.
	_, err := f.registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_UNSPECIFIED)
	if !errors.Is(err, ErrObserved) {
		t.Fatalf("error = %v, want ErrObserved", err)
	}
	if _, err := f.registry.Arm(context.Background(), 1, true); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("key no longer poisoned: %v", err)
	}
}

func TestQuarantineEndsExactlyAtItsDeadline(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)
	if _, err := f.registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_DISARMED); err != nil {
		t.Fatal(err)
	}

	f.clock.advance(5*time.Second - time.Millisecond)
	f.freshen()
	if _, err := f.registry.Arm(context.Background(), 1, true); !errors.Is(err, ErrQuarantined) {
		t.Fatalf("one millisecond early: error = %v, want ErrQuarantined", err)
	}

	f.clock.advance(time.Millisecond)
	f.freshen()
	if _, err := f.registry.Arm(context.Background(), 1, true); errors.Is(err, ErrQuarantined) {
		t.Fatal("still quarantined at its deadline")
	}
}

// TestLateAckIsNotBufferedAcrossTransactions checks the half of the stale-ACK
// defence that is mechanically testable: a delayed ACK is discarded outright
// when no command is in flight, rather than being held and applied to the next
// one.
//
// This passes with the quarantine disabled, and deliberately so. The quarantine
// contributes the wall-clock window in which a late ACK can arrive while
// nothing is listening; that a real ACK arrives inside the window rather than
// after it is a property of the link, not something a test with an injected
// clock can establish. TestQuarantineEndsExactlyAtItsDeadline covers the part
// the registry does enforce.
func TestLateAckIsNotBufferedAcrossTransactions(t *testing.T) {
	var registry *Registry
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	registry = f.registry

	first := timedOutArm(t, f)
	if _, err := registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_DISARMED); err != nil {
		t.Fatal(err)
	}

	// The delayed answer to the first command finally arrives.
	if err := registry.Publish(
		context.Background(), ackEvent(255, 190, gcsv1.MavResult_MAV_RESULT_ACCEPTED)); err != nil {
		t.Fatal(err)
	}

	f.clock.advance(5 * time.Second)
	f.freshen()

	next, err := registry.Arm(context.Background(), 1, true)
	if err != nil {
		t.Fatalf("Arm() error = %v", err)
	}
	if next.GetState() == gcsv1.CommandState_COMMAND_STATE_ACCEPTED {
		t.Fatal("a stale ACK settled a later command")
	}
	if next.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Fatalf("state = %v, want TIMED_OUT", next.GetState())
	}
}

type failingPublisher struct {
	mu    sync.Mutex
	calls int
}

func (p *failingPublisher) Publish(context.Context, vehicle.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return errors.New("sink unavailable")
}

func TestPublisherErrorDoesNotRollQuarantineBack(t *testing.T) {
	publisher := &failingPublisher{}
	f := resolveFixtureFor(t, publisher, silentWrite)
	first := timedOutArm(t, f)

	out, err := f.registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_DISARMED)
	if err != nil {
		t.Fatalf("ResolveArm() error = %v, want the publish error to be logged only", err)
	}
	if out.GetResolution() == nil {
		t.Fatal("resolution missing from the returned snapshot")
	}
	// Rolling back would restore a poison that subscribers were already told
	// was resolved.
	if _, err := f.registry.Arm(context.Background(), 1, true); !errors.Is(err, ErrQuarantined) {
		t.Fatalf("quarantine was rolled back: %v", err)
	}
}

// reenteringPublisher calls back into the registry from inside Publish. If the
// registry published while holding its mutex, this deadlocks.
type reenteringPublisher struct {
	registry  **Registry
	reentered bool
}

func (p *reenteringPublisher) Publish(context.Context, vehicle.Event) error {
	(*p.registry).QuarantineRemaining(1)
	p.reentered = true
	return nil
}

func TestResolvePublishesOutsideTheRegistryMutex(t *testing.T) {
	var registry *Registry
	publisher := &reenteringPublisher{registry: &registry}
	f := resolveFixtureFor(t, publisher, silentWrite)
	registry = f.registry

	first := timedOutArm(t, f)
	if _, err := registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_ARMED); err != nil {
		t.Fatal(err)
	}
	if !publisher.reentered {
		t.Fatal("publisher never ran")
	}
}
