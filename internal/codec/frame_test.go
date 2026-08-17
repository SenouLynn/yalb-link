package codec

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
)

// FRAME-V1-ACCEPT: a v1 frame decodes on a node configured with OutVersion V2.
//
// OutVersion governs what we transmit, not what we accept — gomavlib's parser
// handles both framings inbound. Asserting it rather than assuming it matters
// because a mixed fleet with an older radio or autopilot will send v1, and the
// failure mode would be a vehicle that silently never appears.
func TestFrameV1Accepted(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "v1_heartbeat")
	h := newHarness(t)

	frame := h.feed(raw)
	if frame == nil {
		t.Fatal("v1 frame produced no event; a V2-configured node must still accept v1 inbound")
	}

	if got := frame.SystemID(); got != meta.SysID {
		t.Errorf("sysID = %d, want %d", got, meta.SysID)
	}

	if hb := DecodeHeartbeat(frame); hb == nil {
		t.Error("v1 heartbeat did not decode to HeartbeatState")
	}
}

// FRAME-V2-UNSIGNED.
func TestFrameV2Unsigned(t *testing.T) {
	t.Parallel()

	raw, meta := loadFixture(t, "heartbeat_v2")
	h := newHarness(t)

	frame := h.feed(raw)
	if frame == nil {
		t.Fatal("v2 unsigned frame produced no event")
	}

	if got := frame.Frame.GetSequenceNumber(); got != meta.Seq {
		t.Errorf("seq = %d, want %d", got, meta.Seq)
	}
}

// FRAME-BAD-CRC and FRAME-TRUNCATED.
//
// Both assert the same three things: no frame surfaces, nothing panics, and the
// parse-error counter moves. Note what is deliberately *not* asserted — an
// error return from Decode. Decode returns a zero Decoded for a parse error and
// for an unhandled message ID alike, because it never sees either: gomavlib
// emits EventParseError in place of EventFrame. Asserting an error here would
// be asserting an API that does not and should not exist.
func TestFrameBadCRCDoesNotSurface(t *testing.T) {
	t.Parallel()

	raw, _ := loadFixture(t, "bad_crc")
	h := newHarness(t)

	before := h.node.ParseErrors()

	// feed returns nil on timeout, which is the expected outcome: a frame that
	// fails CRC validation produces no EventFrame at all.
	if frame := h.feed(raw); frame != nil {
		t.Errorf("corrupt frame surfaced as an EventFrame: %#v", frame.Message())
	}

	// The counter is incremented on the pump goroutine, so allow it to be
	// observed rather than reading it once and racing.
	deadline := time.Now().Add(2 * time.Second)
	for h.node.ParseErrors() == before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if got := h.node.ParseErrors(); got == before {
		t.Errorf("parse error counter did not move (still %d); "+
			"a dropped parse error is indistinguishable from silence", got)
	}
}

// FRAME-TRUNCATED.
//
// This does NOT behave like a bad CRC, and the difference was found by running
// it. gomavlib's parser is stream-oriented: a frame cut mid-payload with no
// following bytes leaves it waiting for the remainder, so it emits no
// EventParseError and no counter moves. The roadmap asserted the opposite.
//
// The observable contract is therefore only "no frame surfaces, no panic".
// Asserting a parse error here would be asserting behaviour the library does
// not have — and would have been "fixed" by weakening the bad-CRC path, which
// does work.
//
// Worth knowing operationally: the damaging case is a truncated frame followed
// by more traffic, where the parser consumes the next frame's bytes as this
// one's remainder and both are lost. That is a Tier 5 concern against a live
// link, not something this fixture can express.
func TestFrameTruncatedWaitsForContinuation(t *testing.T) {
	t.Parallel()

	raw, _ := loadFixture(t, "truncated")
	h := newHarness(t)

	before := h.node.ParseErrors()

	if frame := h.feed(raw); frame != nil {
		t.Errorf("truncated frame surfaced as an EventFrame: %#v", frame.Message())
	}

	if got := h.node.ParseErrors(); got != before {
		t.Errorf("parse errors = %d, want %d unchanged: a mid-payload truncation "+
			"leaves the parser waiting, it does not fail", got, before)
	}
}

// A vehicle is addressable only after it has been heard from.
func TestRoutingTableIsBuiltFromInboundFramesOnly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	// Nothing heard yet: sysid 1 is not addressable.
	if _, ok := h.node.LinkFor(1); ok {
		t.Error("sysid 1 was addressable before any frame arrived")
	}

	raw, meta := loadFixture(t, "heartbeat_v2")

	if frame := h.feed(raw); frame == nil {
		t.Fatal("no frame surfaced")
	}

	link, ok := h.node.LinkFor(meta.SysID)
	if !ok {
		t.Fatalf("sysid %d not addressable after a frame from it", meta.SysID)
	}

	if link == "" {
		t.Error("route resolved to an empty link ID")
	}
}

// An unaddressable target is rejected, never broadcast.
//
// gomavlib's only destination-free write is WriteMessageAll, so a port shaped
// WriteMessage(msg) could only be satisfied by transmitting to every channel:
// N-times the uplink bandwidth, and a command for one vehicle physically sent
// over another's radio. This test pins the rejection.
func TestWriteToUnknownLinkIsRejected(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	err := h.node.WriteTo(LinkID("no-such-link"), EncodeHeartbeat())
	if err == nil {
		t.Fatal("write to an unknown link succeeded; it must be rejected, not broadcast")
	}

	if !errors.Is(err, ErrUnknownLink) {
		t.Errorf("error = %v, want ErrUnknownLink", err)
	}
}

// Close must return even when nobody is draining Events.
//
// This is the deadlock the pump's select guards against: an unbuffered channel
// plus a consumer that stopped reading used to wedge the pump goroutine, and
// Close waits on that goroutine.
func TestCloseReturnsWithNoEventConsumer(t *testing.T) {
	t.Parallel()

	testSide, nodeSide := net.Pipe()

	node, err := NewNode([]gomavlib.EndpointConf{
		gomavlib.EndpointCustomClient{
			Connect: func(context.Context) (net.Conn, error) { return nodeSide, nil },
			Label:   "test",
		},
	})
	if err != nil {
		t.Fatalf("creating node: %v", err)
	}

	defer func() { _ = testSide.Close() }()

	// Deliberately never read node.Events(). The node emits its own heartbeat,
	// so the pump will have something to forward and will block on the send.
	drained := make(chan struct{})

	go func() {
		defer close(drained)

		buf := make([]byte, 1024)

		for {
			if _, err := testSide.Read(buf); err != nil {
				return
			}
		}
	}()

	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = node.Close()
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return with no event consumer; the pump is wedged")
	}

	_ = testSide.Close()
	<-drained
}
