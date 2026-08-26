package recording

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestDeleteHandler(t *testing.T) {
	store := newTestStore(t, nil)
	started, err := store.StartRecording(context.Background(), "delete me")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("active", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodDelete, "/api/recordings/1", nil)
		request.SetPathValue("id", fmt.Sprint(started.ID))
		response := httptest.NewRecorder()
		DeleteHandler(store)(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})
	if _, err := store.StopRecording(context.Background()); err != nil {
		t.Fatal(err)
	}

	t.Run("invalid", func(t *testing.T) {
		for _, id := range []string{"nope", "0"} {
			request := httptest.NewRequest(http.MethodDelete, "/api/recordings/"+id, nil)
			request.SetPathValue("id", id)
			response := httptest.NewRecorder()
			DeleteHandler(store)(response, request)
			if response.Code != http.StatusBadRequest {
				t.Errorf("id %q status = %d", id, response.Code)
			}
		}
	})

	t.Run("success", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodDelete, "/api/recordings/1", nil)
		request.SetPathValue("id", fmt.Sprint(started.ID))
		response := httptest.NewRecorder()
		DeleteHandler(store)(response, request)
		if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
			t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
		}
	})

	t.Run("missing", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodDelete, "/api/recordings/999", nil)
		request.SetPathValue("id", "999")
		response := httptest.NewRecorder()
		DeleteHandler(store)(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})
}
