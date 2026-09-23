package stream

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"yalb.gcs/internal/connection"
)

func TestAcquisitionRetainsReleasedStateAndOwnsSnapshot(t *testing.T) {
	h := NewHub(nil, 1)
	defer h.Close()
	s := connection.Status{ID: "serial:a", State: connection.Reporting, VehicleKeys: []connection.VehicleKey{{SystemID: 1, ComponentID: 1}}}
	h.PublishAcquisition(s)
	s.VehicleKeys[0].SystemID = 99
	ch, cancel := h.Subscribe()
	defer cancel()
	ev := <-ch
	var got connection.Status
	if err := json.Unmarshal(ev.JSON, &got); err != nil {
		t.Fatal(err)
	}
	if got.VehicleKeys[0].SystemID != 1 {
		t.Fatal("retained state mutated")
	}
	w := httptest.NewRecorder()
	if err := writeEvent(w, ev); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(w.Body.String(), "event: acquisition\ndata: {") {
		t.Fatal(w.Body.String())
	}
	s.State = connection.Released
	h.PublishAcquisition(s)
	late, unsub := h.Subscribe()
	defer unsub()
	ev = <-late
	if err := json.Unmarshal(ev.JSON, &got); err != nil {
		t.Fatal(err)
	}
	if got.State != connection.Released {
		t.Fatal(got.State)
	}
}
