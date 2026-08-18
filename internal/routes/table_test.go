package routes

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.uber.org/goleak"

	"yalb.gcs/internal/codec"
)

// TestMain asserts no goroutine leaks. The table deliberately has no
// background goroutine — eviction happens on a call, not on a timer — and this
// is what would notice if one appeared.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

const t0 int64 = 1_000_000

func key(sysID uint8) Key { return Key{SysID: sysID, CompID: 1} }

func entry(sysID uint8, link codec.LinkID) Entry {
	return Entry{
		Key:     key(sysID),
		Link:    link,
		SrcIP:   "10.0.0.5",
		SrcPort: 14550,
	}
}

func TestUpsertThenResolve(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "udp:10.0.0.5:14550"), t0)

	link, err := table.Resolve(key(1), t0)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if link != "udp:10.0.0.5:14550" {
		t.Errorf("link = %q, want the link the vehicle was heard on", link)
	}
}

// An unaddressable target is rejected. The alternative gomavlib offers is
// WriteMessageAll, which puts a command for one vehicle on every radio.
func TestResolveUnknownTargetIsRejected(t *testing.T) {
	table := NewTable()

	_, err := table.Resolve(key(2), t0)
	if !errors.Is(err, ErrNoRoute) {
		t.Fatalf("Resolve of an unheard vehicle returned %v, want ErrNoRoute", err)
	}

	// The failure names the target; a rejection nobody can act on is a
	// rejection that gets replaced with a broadcast.
	if got := err.Error(); !strings.Contains(got, "2:1") {
		t.Errorf("error %q does not name the target", got)
	}
}

// Component ID is part of the key. A gimbal and an autopilot on the same
// system can live on different links.
func TestComponentsRouteIndependently(t *testing.T) {
	table := NewTable()
	table.Upsert(Entry{Key: Key{SysID: 1, CompID: 1}, Link: "serial"}, t0)
	table.Upsert(Entry{Key: Key{SysID: 1, CompID: 154}, Link: "udp"}, t0)

	autopilot, err := table.Resolve(Key{SysID: 1, CompID: 1}, t0)
	if err != nil {
		t.Fatalf("Resolve autopilot: %v", err)
	}

	gimbal, err := table.Resolve(Key{SysID: 1, CompID: 154}, t0)
	if err != nil {
		t.Fatalf("Resolve gimbal: %v", err)
	}

	if autopilot == gimbal {
		t.Errorf("both components resolved to %q; the key collapsed", autopilot)
	}
}

// A vehicle heard on a new link moves. Two entries would mean two answers to
// "where do I send".
func TestUpsertMovesRatherThanDuplicates(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "link-a"), t0)
	table.Upsert(Entry{Key: key(1), Link: "link-b"}, t0+1_000)

	if table.Len() != 1 {
		t.Fatalf("table holds %d entries after a move, want 1", table.Len())
	}

	link, err := table.Resolve(key(1), t0+1_000)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if link != "link-b" {
		t.Errorf("link = %q, want link-b", link)
	}
}

// Diagnostics survive an update that cannot supply them, rather than being
// blanked by it.
func TestUpsertKeepsKnownSourceAddress(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "link-a"), t0)
	table.Upsert(Entry{Key: key(1), Link: "link-a"}, t0+1_000)

	got, ok := table.Lookup(key(1), t0+1_000)
	if !ok {
		t.Fatal("route gone after update")
	}

	if got.SrcIP != "10.0.0.5" || got.SrcPort != 14550 {
		t.Errorf("source address = %s:%d, want the previously observed one", got.SrcIP, got.SrcPort)
	}
}

func TestLookupEvictsStaleEntryLazily(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "link-a"), t0)

	if _, ok := table.Lookup(key(1), t0+routeTTLMs); !ok {
		t.Error("route gone at exactly the TTL; the boundary is not yet stale")
	}

	if _, ok := table.Lookup(key(1), t0+routeTTLMs+1); ok {
		t.Error("stale route still resolvable")
	}

	if table.Len() != 0 {
		t.Errorf("stale entry left behind: %d entries remain", table.Len())
	}
}

// A fresh vehicle is untouched by the sweep that removes a stale one.
func TestEvictSweepsOnlyStaleEntries(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "link-a"), t0)
	table.Upsert(entry(2, "link-a"), t0+routeTTLMs)

	if n := table.Evict(t0 + routeTTLMs + 1); n != 1 {
		t.Fatalf("Evict removed %d entries, want 1", n)
	}

	if _, ok := table.Lookup(key(2), t0+routeTTLMs+1); !ok {
		t.Error("the fresh route was swept")
	}
}

// A closed channel takes its routes with it: a route through a dead link makes
// an unreachable vehicle look addressable.
func TestForgetLinkDropsItsRoutes(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(1, "link-a"), t0)
	table.Upsert(entry(2, "link-a"), t0)
	table.Upsert(entry(3, "link-b"), t0)

	if n := table.ForgetLink("link-a"); n != 2 {
		t.Fatalf("ForgetLink dropped %d routes, want 2", n)
	}

	if _, err := table.Resolve(key(1), t0); !errors.Is(err, ErrNoRoute) {
		t.Error("vehicle still addressable through a closed link")
	}

	if _, err := table.Resolve(key(3), t0); err != nil {
		t.Errorf("unrelated link lost its route: %v", err)
	}
}

func TestSnapshotIsOrdered(t *testing.T) {
	table := NewTable()
	table.Upsert(entry(3, "link"), t0)
	table.Upsert(entry(1, "link"), t0)
	table.Upsert(entry(2, "link"), t0)

	snapshot := table.Snapshot()

	got := make([]uint8, 0, len(snapshot))
	for _, e := range snapshot {
		got = append(got, e.Key.SysID)
	}

	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Snapshot order = %v, want ascending system IDs", got)
	}
}

// The table is written by the receive path and read by the send path. This is
// the -race detector's test, not an assertion about results.
func TestConcurrentAccess(t *testing.T) {
	table := NewTable()

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Add(3)

		sysID := uint8(i + 1)

		go func() {
			defer wg.Done()

			for tick := range int64(100) {
				table.Upsert(entry(sysID, codec.LinkID(fmt.Sprintf("link-%d", sysID))), t0+tick)
			}
		}()

		go func() {
			defer wg.Done()

			for tick := range int64(100) {
				_, _ = table.Resolve(key(sysID), t0+tick)
			}
		}()

		go func() {
			defer wg.Done()

			for tick := range int64(100) {
				table.Evict(t0 + tick)
				_ = table.Snapshot()
			}
		}()
	}

	wg.Wait()

	if table.Len() != 8 {
		t.Errorf("table holds %d routes after the storm, want 8", table.Len())
	}
}
