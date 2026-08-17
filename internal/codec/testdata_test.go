package codec

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
)

// fixtureDir is the committed golden-byte corpus, regenerable via
// scripts/gen_mavlink_fixtures.py.
const fixtureDir = "../../contracts/mavlink"

// fixtureMeta mirrors the .json companion written by the generator.
type fixtureMeta struct {
	Fields        map[string]any `json:"fields"`
	ExpectedProto map[string]any `json:"expected_proto"`
	MessageName   string         `json:"message_name"`
	Note          string         `json:"note"`
	MessageID     uint32         `json:"message_id"`
	SysID         uint8          `json:"sys_id"`
	CompID        uint8          `json:"comp_id"`
	Seq           uint8          `json:"seq"`
}

func loadFixture(t *testing.T, name string) ([]byte, fixtureMeta) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(fixtureDir, name+".bin"))
	if err != nil {
		t.Fatalf("reading fixture %s.bin: %v", name, err)
	}

	metaRaw, err := os.ReadFile(filepath.Join(fixtureDir, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture %s.json: %v", name, err)
	}

	var meta fixtureMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("parsing fixture %s.json: %v", name, err)
	}

	return raw, meta
}

// harness runs a real gomavlib node over an in-memory pipe.
//
// Golden byte tests must run the real decoder. Hand-crafting gomavlib.EventFrame
// structs would bypass STX detection, length extraction, CRC_EXTRA validation
// and dialect struct population — that is, everything worth testing — and would
// pass against a codec that could not read a single real frame.
type harness struct {
	node *Node
	conn net.Conn
	t    *testing.T
	sent []byte
	mu   sync.Mutex
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	// EndpointCustomClient, not the deprecated EndpointCustom. EndpointCustomConn
	// does not exist in v3.
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

	h := &harness{node: node, conn: testSide, t: t}

	// Drain the node's outbound bytes.
	//
	// This is load-bearing, not hygiene. net.Pipe is unbuffered and fully
	// synchronous, and the node emits its own GCS heartbeat once a second
	// (HeartbeatDisable is false by default — see frame.go). With nothing
	// reading this side, that first heartbeat write blocks forever, the node's
	// writer goroutine wedges, and both the test and Close deadlock. Draining
	// also gives encoder tests something to assert against.
	done := make(chan struct{})

	go func() {
		defer close(done)

		buf := make([]byte, 4096)

		for {
			n, err := testSide.Read(buf)
			if n > 0 {
				h.mu.Lock()
				h.sent = append(h.sent, buf[:n]...)
				h.mu.Unlock()
			}

			if err != nil {
				return
			}
			_ = n
		}
	}()

	t.Cleanup(func() {
		_ = node.Close()
		_ = testSide.Close()
		<-done
	})

	return h
}

// feed writes raw frame bytes to the node and returns the next frame event.
//
// Returns nil if no frame surfaces before the deadline, which is the expected
// outcome for a corrupt frame: gomavlib emits EventParseError internally and no
// EventFrame is produced.
func (h *harness) feed(raw []byte) *gomavlib.EventFrame {
	h.t.Helper()

	go func() {
		_ = h.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = h.conn.Write(raw)
	}()

	deadline := time.After(2 * time.Second)

	for {
		select {
		case evt, ok := <-h.node.Events():
			if !ok {
				return nil
			}

			if frame, isFrame := evt.(*gomavlib.EventFrame); isFrame {
				return frame
			}

			// Channel-open and parse-error events are not frames; keep waiting.
			continue

		case <-deadline:
			return nil
		}
	}
}
