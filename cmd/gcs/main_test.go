package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"yalb.gcs/internal/mission"
)

func TestMissionRouteIsMountedWhenCommandsAreDisabled(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coordinator := &mission.Coordinator{}
	mux := newHTTPMux(log, nil, nil, nil, coordinator)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet, "/api/vehicles/1/1/mission", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("mission status = %d, want %d; body=%s",
			response.Code, http.StatusNotFound, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	unsupported := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodPost, "/api/commands/arm", http.StatusNotFound},
		{http.MethodPost, "/api/vehicles/1/1/mission", http.StatusMethodNotAllowed},
		{http.MethodPut, "/api/vehicles/1/1/mission", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/vehicles/1/1/mission", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/vehicles/1/1/mission/clear", http.StatusNotFound},
		{http.MethodPost, "/api/vehicles/1/1/mission/start", http.StatusNotFound},
		{http.MethodPost, "/api/vehicles/1/1/mission/current", http.StatusNotFound},
	}
	for _, tc := range unsupported {
		got := httptest.NewRecorder()
		mux.ServeHTTP(got, httptest.NewRequest(tc.method, tc.path, nil))
		if got.Code != tc.status {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, got.Code, tc.status)
		}
	}
}
