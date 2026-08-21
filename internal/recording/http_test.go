package recording

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRecordingLifecycle(t *testing.T) {
	store := newTestStore(t, nil)

	startRequest := httptest.NewRequest(http.MethodPost, "/api/recordings/start", strings.NewReader(`{"name":"test flight"}`))
	startResponse := httptest.NewRecorder()
	StartHandler(store)(startResponse, startRequest)
	if startResponse.Code != http.StatusCreated {
		t.Fatalf("start status = %d, body = %s", startResponse.Code, startResponse.Body.String())
	}
	var started Recording
	if err := json.NewDecoder(startResponse.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}

	stopResponse := httptest.NewRecorder()
	StopHandler(store)(stopResponse, httptest.NewRequest(http.MethodPost, "/api/recordings/stop", nil))
	if stopResponse.Code != http.StatusOK {
		t.Fatalf("stop status = %d, body = %s", stopResponse.Code, stopResponse.Body.String())
	}

	listResponse := httptest.NewRecorder()
	ListHandler(store)(listResponse, httptest.NewRequest(http.MethodGet, "/api/recordings", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d", listResponse.Code)
	}
	var listed []Recording
	if err := json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != started.ID || listed[0].Status != "stopped" {
		t.Fatalf("listed = %+v", listed)
	}

	if recordings, err := store.ListRecordings(context.Background()); err != nil || len(recordings) != 1 {
		t.Fatalf("ListRecordings() = %+v, %v", recordings, err)
	}
}

func TestStartHandlerRejectsInvalidJSON(t *testing.T) {
	store := newTestStore(t, nil)
	response := httptest.NewRecorder()
	StartHandler(store)(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{")))
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.Code)
	}
}

func TestStartHandlerAcceptsEmptyBody(t *testing.T) {
	store := newTestStore(t, nil)
	response := httptest.NewRecorder()
	StartHandler(store)(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusCreated {
		t.Errorf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
