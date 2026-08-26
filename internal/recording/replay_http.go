package recording

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// EventsPattern is where one recording's events are served.
const EventsPattern = "GET /api/recordings/{id}/events"

// ReplayPage is one bounded page of a recording's persisted events.
type ReplayPage struct {
	// NextSeq is the from_seq that continues this page, or null at the end of
	// the events currently available. It is what stops a paging client.
	NextSeq   *int64            `json:"next_seq"`
	Events    []ReplayPageEvent `json:"events"`
	Recording Recording         `json:"recording"`
}

// ReplayPageEvent is one recorded event as the browser reads it.
type ReplayPageEvent struct {
	// Event is protobuf JSON, not an ordinarily marshalled struct. It is the
	// same encoding the live SSE stream carries, so the browser parses a
	// replayed event with the code that parses a live one.
	Event        json.RawMessage `json:"event"`
	Kind         string          `json:"kind"`
	Seq          int64           `json:"seq"`
	OccurredAtMs int64           `json:"occurred_at_ms"`
}

// ReplayEventsHandler serves one page of a recording in sequence order.
func ReplayEventsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "recording id must be a positive integer", http.StatusBadRequest)
			return
		}
		query := r.URL.Query()
		fromSeq, err := resolveFromSeq(query.Get("from_seq"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		limit, err := resolvePageLimit(query.Get("limit"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// The row lookup is what distinguishes an unknown recording from one
		// that exists and has no events yet; the event query cannot.
		recording, err := store.GetRecording(r.Context(), id)
		if err != nil {
			if errors.Is(err, ErrRecordingNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, "could not read recording", http.StatusInternalServerError)
			return
		}

		events, err := store.ReplayFrom(r.Context(), id, fromSeq, limit)
		if err != nil {
			http.Error(w, "could not read recording events", http.StatusInternalServerError)
			return
		}

		page, err := buildReplayPage(recording, events, limit)
		if err != nil {
			http.Error(w, "could not encode recording events", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

// buildReplayPage renders decoded events as protobuf JSON and sets the cursor.
func buildReplayPage(recording Recording, events []ReplayEvent, limit int) (ReplayPage, error) {
	page := ReplayPage{Recording: recording, Events: make([]ReplayPageEvent, 0, len(events))}

	for _, event := range events {
		kind := KindTelemetry
		var message proto.Message = event.Telemetry
		if event.Fleet != nil {
			kind, message = KindFleet, event.Fleet
		} else if event.Command != nil {
			kind, message = KindCommand, event.Command
		}
		data, err := protojson.Marshal(message)
		if err != nil {
			return ReplayPage{}, fmt.Errorf("recording: encoding %s seq %d: %w", kind, event.Seq, err)
		}
		page.Events = append(page.Events, ReplayPageEvent{
			Event:        data,
			Kind:         kind,
			Seq:          event.Seq,
			OccurredAtMs: event.OccurredAt.UnixMilli(),
		})
	}

	// A short page is the end of what is available. A full one may not be, so
	// the cursor advances and the client spends one more request finding out.
	if len(events) == limit && limit > 0 {
		next := events[len(events)-1].Seq + 1
		page.NextSeq = &next
	}

	return page, nil
}

// resolveFromSeq reads the paging cursor. Absent means the start.
func resolveFromSeq(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("from_seq must be a non-negative integer")
	}
	return value, nil
}

// resolvePageLimit reads the page size, clamped rather than rejected above the
// maximum: a client asking for more than the server will send should get the
// most it can rather than an error it cannot act on.
func resolvePageLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultPageSize, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errors.New("limit must be a positive integer")
	}
	return min(value, MaxPageSize), nil
}
