package stream

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

// sseFrame is one parsed `event:`/`data:` pair.
type sseFrame struct {
	Name string
	Data string
}

// readFrames reads exactly n frames, ignoring comment keepalives.
func readFrames(t *testing.T, r *bufio.Reader, n int) []sseFrame {
	t.Helper()

	frames := make([]sseFrame, 0, n)

	var current sseFrame

	for len(frames) < n {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading stream after %d frames: %v", len(frames), err)
		}

		line = strings.TrimSuffix(line, "\n")

		switch {
		case strings.HasPrefix(line, ":"):
			// Keepalive comment.
		case strings.HasPrefix(line, "event: "):
			current.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.Data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if current.Name != "" {
				frames = append(frames, current)
				current = sseFrame{}
			}
		default:
			t.Fatalf("unrecognised SSE line %q", line)
		}
	}

	return frames
}

// startStream serves the handler and returns a reader over a live response.
func startStream(t *testing.T, hub *Hub) (*bufio.Reader, context.CancelFunc) {
	t.Helper()

	srv := httptest.NewServer(Handler(hub, discard()))

	ctx, cancel := context.WithCancel(t.Context())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+Path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("opening stream: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = resp.Body.Close()
		srv.Close()
	})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	return bufio.NewReader(resp.Body), cancel
}

// TestHandlerStreamsBootstrapAndLive covers the contract end to end: the
// browser gets retained state, then whatever happens next, without polling.
func TestHandlerStreamsBootstrapAndLive(t *testing.T) {
	hub := newTestHub(t, 16)

	publishAll(t, hub, fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	stream, _ := startStream(t, hub)

	// The first frame arriving at all proves the handler flushed rather than
	// buffering until the response closed.
	got := readFrames(t, stream, 1)
	if got[0].Name != EventFleet {
		t.Fatalf("first event = %q, want %q", got[0].Name, EventFleet)
	}

	publishAll(t, hub, attitudeEvent(1, 1, 0.5))

	live := readFrames(t, stream, 1)
	if live[0].Name != EventTelemetry {
		t.Fatalf("live event = %q, want %q", live[0].Name, EventTelemetry)
	}
}

// TestHandlerRoundTripsProtobufJSON is the wire-format assertion: what the
// handler writes must parse back into the same message the hub published.
func TestHandlerRoundTripsProtobufJSON(t *testing.T) {
	hub := newTestHub(t, 16)

	publishAll(t, hub, vfrEvent(7, 1, 13.5))

	stream, _ := startStream(t, hub)

	frames := readFrames(t, stream, 1)

	var decoded gcsv1.TelemetryEvent
	if err := protojson.Unmarshal([]byte(frames[0].Data), &decoded); err != nil {
		t.Fatalf("parsing %q as TelemetryEvent: %v", frames[0].Data, err)
	}

	if got := decoded.GetVehicleId().GetSystemId(); got != 7 {
		t.Errorf("sysid = %d, want 7", got)
	}

	if got := decoded.GetVfrHud().GetGroundspeedMS(); got != 13.5 {
		t.Errorf("groundspeed = %v, want 13.5", got)
	}
}

// TestHandlerRoundTripsZeroValuedPayload proves payload presence, rather than
// JSON member verbosity, distinguishes a level aircraft from no ATTITUDE.
func TestHandlerRoundTripsZeroValuedPayload(t *testing.T) {
	hub := newTestHub(t, 16)

	publishAll(t, hub, attitudeEvent(1, 1, 0))

	stream, _ := startStream(t, hub)

	frames := readFrames(t, stream, 1)

	var decoded gcsv1.TelemetryEvent
	if err := protojson.Unmarshal([]byte(frames[0].Data), &decoded); err != nil {
		t.Fatalf("parsing %q: %v", frames[0].Data, err)
	}

	if decoded.GetAttitude() == nil {
		t.Fatal("zero-valued ATTITUDE payload was lost")
	}

	if got := decoded.GetAttitude().GetRollRad(); got != 0 {
		t.Errorf("roll = %v, want 0", got)
	}
}

// TestHandlerWritesSingleLineData keeps each event to one SSE frame. A newline
// inside data would split the payload and deliver truncated JSON.
func TestHandlerWritesSingleLineData(t *testing.T) {
	hub := newTestHub(t, 16)

	// STATUSTEXT is the family most likely to carry awkward characters.
	publishAll(t, hub, vehicle.Event{Telemetry: &gcsv1.TelemetryEvent{
		VehicleId: &gcsv1.VehicleId{SystemId: 1, ComponentId: 1},
		Payload: &gcsv1.TelemetryEvent_StatusText{
			StatusText: &gcsv1.StatusText{
				Severity: gcsv1.MavSeverity_MAV_SEVERITY_WARNING,
				Text:     "line one\nline two\ttabbed",
			},
		},
	}})

	stream, _ := startStream(t, hub)

	frames := readFrames(t, stream, 1)

	if strings.Contains(frames[0].Data, "\n") {
		t.Fatalf("data field contains a raw newline: %q", frames[0].Data)
	}

	var decoded gcsv1.TelemetryEvent
	if err := protojson.Unmarshal([]byte(frames[0].Data), &decoded); err != nil {
		t.Fatalf("parsing %q: %v", frames[0].Data, err)
	}

	if got := decoded.GetStatusText().GetText(); got != "line one\nline two\ttabbed" {
		t.Errorf("text = %q, want the newline preserved inside the JSON string", got)
	}
}

// TestHandlerSendsKeepalives proves a quiet stream still produces bytes, so a
// proxy does not close a connection that is merely waiting for a slow vehicle.
func TestHandlerSendsKeepalives(t *testing.T) {
	hub := newTestHub(t, 16)

	srv := httptest.NewServer(handler(hub, discard(), 10*time.Millisecond))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+Path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("opening stream: %v", err)
	}

	t.Cleanup(func() { _ = resp.Body.Close() })

	reader := bufio.NewReader(resp.Body)

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading keepalive: %v", err)
	}

	if !strings.HasPrefix(line, ":") {
		t.Errorf("first line on an idle stream = %q, want an SSE comment", line)
	}
}

// TestHandlerUnsubscribesOnDisconnect keeps a closed browser tab from leaking a
// subscription that the hub would keep filling forever.
func TestHandlerUnsubscribesOnDisconnect(t *testing.T) {
	hub := newTestHub(t, 16)

	publishAll(t, hub, fleetEvent(gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED, 1, 1))

	stream, cancel := startStream(t, hub)

	readFrames(t, stream, 1)

	if got := hub.Subscribers(); got != 1 {
		t.Fatalf("hub has %d subscribers while one stream is open, want 1", got)
	}

	cancel()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Subscribers() == 0 {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("hub still holds %d subscribers after the client disconnected", hub.Subscribers())
}

// TestHandlerEndsWhenEvicted closes the response when the hub drops a slow
// subscriber, so the browser sees a disconnect and reconnects for a fresh
// bootstrap rather than hanging on a stream it will never receive again.
func TestHandlerEndsWhenEvicted(t *testing.T) {
	hub := newTestHub(t, 1)

	srv := httptest.NewServer(Handler(hub, discard()))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+Path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("opening stream: %v", err)
	}

	t.Cleanup(func() { _ = resp.Body.Close() })

	// Wait for the subscription to register, then overrun it without reading.
	deadline := time.Now().Add(3 * time.Second)
	for hub.Subscribers() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	for i := range 4096 {
		publishAll(t, hub, attitudeEvent(1, 1, float32(i)))

		if hub.Subscribers() == 0 {
			break
		}
	}

	if got := hub.Subscribers(); got != 0 {
		t.Fatalf("subscriber was not evicted; hub holds %d", got)
	}

	// The response terminates rather than hanging open.
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("reading the evicted stream to completion: %v", err)
	}
}
