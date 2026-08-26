package recording

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// replayMux mounts the events route so its pattern, and the path value the
// handler reads, are exercised rather than assumed.
func replayMux(store *Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(EventsPattern, ReplayEventsHandler(store))
	return mux
}

func getReplayPage(t *testing.T, mux *http.ServeMux, target string) (ReplayPage, int) {
	t.Helper()
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	if response.Code != http.StatusOK {
		return ReplayPage{}, response.Code
	}
	var page ReplayPage
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatalf("decoding %s: %v", target, err)
	}
	return page, response.Code
}

func TestReplayEventsHandlerPagesAndTerminates(t *testing.T) {
	store := newTestStore(t, nil)
	recording := recordFlight(t, store, 5)
	mux := replayMux(store)

	var seqs []int64
	target := fmt.Sprintf("/api/recordings/%d/events?limit=2", recording.ID)
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("paging did not terminate")
		}
		page, code := getReplayPage(t, mux, target)
		if code != http.StatusOK {
			t.Fatalf("status = %d", code)
		}
		if page.Recording.ID != recording.ID {
			t.Fatalf("page recording = %d, want %d", page.Recording.ID, recording.ID)
		}
		for _, event := range page.Events {
			seqs = append(seqs, event.Seq)
		}
		if page.NextSeq == nil {
			break
		}
		target = fmt.Sprintf("/api/recordings/%d/events?limit=2&from_seq=%d", recording.ID, *page.NextSeq)
	}

	if len(seqs) != 5 {
		t.Fatalf("collected %d events, want 5", len(seqs))
	}
	for i, seq := range seqs {
		if seq != int64(i+1) {
			t.Errorf("event %d seq = %d, want %d", i, seq, i+1)
		}
	}
}

func TestReplayEventsHandlerEmitsProtobufJSON(t *testing.T) {
	store := newTestStore(t, nil)
	clock := time.Unix(1_800_000_000, 0).UTC()
	source := deterministicEvents(clock)

	recording, err := store.StartRecording(context.Background(), "protojson flight")
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	recorder := &Recorder{Store: store}
	for _, event := range source {
		if err := recorder.Publish(context.Background(), event); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}

	page, code := getReplayPage(t, replayMux(store), fmt.Sprintf("/api/recordings/%d/events", recording.ID))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(page.Events) != len(source) {
		t.Fatalf("events = %d, want %d", len(page.Events), len(source))
	}

	// The browser parses these with the same protobuf-JSON reader it uses for
	// the live SSE stream, so each payload has to survive that round trip.
	for i, got := range page.Events {
		want := source[i]
		switch got.Kind {
		case KindFleet:
			if want.Fleet == nil {
				t.Fatalf("event %d kind = fleet, want telemetry", i)
			}
			decoded := &gcsv1.FleetEvent{}
			if err := protojson.Unmarshal(got.Event, decoded); err != nil {
				t.Fatalf("event %d protojson.Unmarshal: %v", i, err)
			}
			if !proto.Equal(decoded, want.Fleet) {
				t.Errorf("event %d fleet mismatch:\n got %v\nwant %v", i, decoded, want.Fleet)
			}
		case KindTelemetry:
			if want.Telemetry == nil {
				t.Fatalf("event %d kind = telemetry, want fleet", i)
			}
			decoded := &gcsv1.TelemetryEvent{}
			if err := protojson.Unmarshal(got.Event, decoded); err != nil {
				t.Fatalf("event %d protojson.Unmarshal: %v", i, err)
			}
			if !proto.Equal(decoded, want.Telemetry) {
				t.Errorf("event %d telemetry mismatch:\n got %v\nwant %v", i, decoded, want.Telemetry)
			}
		default:
			t.Fatalf("event %d kind = %q", i, got.Kind)
		}
	}
}

func TestReplayEventsHandlerServesActiveRecordingPrefix(t *testing.T) {
	store := newTestStore(t, nil)
	recording := recordFlight(t, store, 3)

	// A second recording left running: its committed prefix must be readable
	// without stopping it, so an operator can check a recording is working.
	if _, err := store.StartRecording(context.Background(), "still flying"); err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}

	page, code := getReplayPage(t, replayMux(store), fmt.Sprintf("/api/recordings/%d/events", recording.ID))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(page.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(page.Events))
	}

	active, code := getReplayPage(t, replayMux(store), fmt.Sprintf("/api/recordings/%d/events", recording.ID+1))
	if code != http.StatusOK {
		t.Fatalf("active status = %d", code)
	}
	if active.Recording.Status != "active" {
		t.Errorf("active status = %q, want active", active.Recording.Status)
	}
	if active.Events == nil {
		t.Error("events = null, want an empty array")
	}
	if active.NextSeq != nil {
		t.Errorf("next_seq = %d, want null", *active.NextSeq)
	}
}

func TestReplayEventsHandlerUnknownRecording(t *testing.T) {
	store := newTestStore(t, nil)
	response := httptest.NewRecorder()
	replayMux(store).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recordings/404/events", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestReplayEventsHandlerRejectsBadParameters(t *testing.T) {
	store := newTestStore(t, nil)
	recording := recordFlight(t, store, 1)
	mux := replayMux(store)

	targets := []string{
		"/api/recordings/not-a-number/events",
		"/api/recordings/0/events",
		"/api/recordings/-1/events",
		fmt.Sprintf("/api/recordings/%d/events?from_seq=abc", recording.ID),
		fmt.Sprintf("/api/recordings/%d/events?from_seq=-1", recording.ID),
		fmt.Sprintf("/api/recordings/%d/events?limit=abc", recording.ID),
		fmt.Sprintf("/api/recordings/%d/events?limit=0", recording.ID),
		fmt.Sprintf("/api/recordings/%d/events?limit=-5", recording.ID),
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
}

func TestResolvePageLimit(t *testing.T) {
	tests := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: DefaultPageSize},
		{raw: "1", want: 1},
		{raw: "500", want: 500},
		// Clamped rather than rejected: a client asking for more than the
		// server will send gets the most it can, not a failure.
		{raw: fmt.Sprint(MaxPageSize + 1), want: MaxPageSize},
		{raw: "0", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "abc", wantErr: true},
	}
	for _, tc := range tests {
		t.Run("limit="+tc.raw, func(t *testing.T) {
			got, err := resolvePageLimit(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolvePageLimit(%q) = %d, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePageLimit(%q) error = %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("resolvePageLimit(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}
