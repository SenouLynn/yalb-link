package mission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

type stubDownloader struct {
	snapshot   *gcsv1.MissionSnapshot
	err        error
	gotTarget  codec.Target
	gotMission uint32
}

func (d *stubDownloader) Download(_ context.Context, target codec.Target, missionType uint32) (*gcsv1.MissionSnapshot, error) {
	d.gotTarget = target
	d.gotMission = missionType

	return d.snapshot, d.err
}

func missionRequest(method string) *http.Request {
	return httptest.NewRequest(method, "/api/vehicles/7/42/mission", nil)
}

func decodeError(t *testing.T, response *httptest.ResponseRecorder) errorResponse {
	t.Helper()

	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	return body
}

func serveDownload(d Downloader, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc(DownloadPattern, DownloadHandler(d))
	mux.ServeHTTP(response, request)

	return response
}

func TestDownloadHandlerReturnsProtobufJSONSnapshot(t *testing.T) {
	t.Parallel()

	want := &gcsv1.MissionSnapshot{
		VehicleId:   &gcsv1.VehicleId{SystemId: 7, ComponentId: 42},
		MissionType: gcsv1.MavMissionType_MAV_MISSION_TYPE_MISSION,
		Items: []*gcsv1.MissionItem{{
			Seq: 0, Command: gcsv1.MavCmd_MAV_CMD_NAV_WAYPOINT,
		}},
		ObservedAt: timestamppb.New(time.Unix(1_700_000_000, 0)),
	}
	downloader := &stubDownloader{snapshot: want}
	mux := http.NewServeMux()
	mux.HandleFunc(DownloadPattern, DownloadHandler(downloader))

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, missionRequest(http.MethodGet))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	decoded := &gcsv1.MissionSnapshot{}
	if err := protojson.Unmarshal(response.Body.Bytes(), decoded); err != nil {
		t.Fatalf("response is not protobuf JSON: %v", err)
	}
	if !decoded.GetObservedAt().AsTime().Equal(want.GetObservedAt().AsTime()) || len(decoded.GetItems()) != 1 {
		t.Errorf("decoded snapshot = %v, want %v", decoded, want)
	}
	if downloader.gotTarget != (codec.Target{SystemID: 7, ComponentID: 42}) {
		t.Errorf("target = %+v, want 7:42", downloader.gotTarget)
	}
	if downloader.gotMission != codec.MissionTypeMission {
		t.Errorf("mission type = %d, want ordinary mission", downloader.gotMission)
	}
}

func TestDownloadHandlerMapsTerminalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err    error
		code   string
		result string
		status int
		name   string
	}{
		{name: "unknown vehicle", err: ErrNoRoute, status: http.StatusNotFound, code: CodeNoRoute},
		{name: "already downloading", err: ErrInFlight, status: http.StatusConflict, code: CodeInFlight},
		{name: "timeout", err: ErrTimeout, status: http.StatusGatewayTimeout, code: CodeTimeout},
		{name: "cancelled", err: ErrCancelled, status: statusClientClosedRequest, code: CodeCancelled},
		{
			name: "rejected", err: &RejectedError{Result: gcsv1.MavMissionResult_MAV_MISSION_DENIED},
			status: http.StatusUnprocessableEntity, code: CodeRejected, result: "MAV_MISSION_DENIED",
		},
		{name: "link write failure", err: ErrWriteFailed, status: http.StatusBadGateway, code: CodeWriteFailed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := serveDownload(&stubDownloader{err: tc.err}, missionRequest(http.MethodGet))

			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			body := decodeError(t, response)
			if body.Code != tc.code || body.Result != tc.result {
				t.Errorf("error = %+v, want code=%q result=%q", body, tc.code, tc.result)
			}
		})
	}
}

func TestDownloadRouteRejectsInvalidIdentityAndMutationMethods(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(DownloadPattern, DownloadHandler(&stubDownloader{}))

	for _, path := range []string{
		"/api/vehicles/0/1/mission",
		"/api/vehicles/256/1/mission",
		"/api/vehicles/1/0/mission",
		"/api/vehicles/1/256/mission",
		"/api/vehicles/not-a-number/1/mission",
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusBadRequest)
		}
	}

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, missionRequest(http.MethodPost))
	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST download route status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestCancelledRequestReleasesCoordinatorSlot(t *testing.T) {
	var coordinator *Coordinator
	coordinator, _ = coordinatorFor(t, func(message.Message) error { return nil })
	coordinator.Timeout = 5 * time.Second

	mux := http.NewServeMux()
	mux.HandleFunc(DownloadPattern, DownloadHandler(coordinator))

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(
		http.MethodGet, "/api/vehicles/1/1/mission", nil,
	).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(response, request)
	}()

	deadline := time.After(2 * time.Second)
	for coordinator.inFlightCount() != 1 {
		select {
		case <-done:
			t.Fatalf("handler returned before cancellation: status=%d body=%s", response.Code, response.Body.String())
		case <-deadline:
			t.Fatal("coordinator did not claim an in-flight slot")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after its request was cancelled")
	}

	if response.Code != statusClientClosedRequest {
		t.Errorf("status = %d, want %d", response.Code, statusClientClosedRequest)
	}
	if body := decodeError(t, response); body.Code != CodeCancelled {
		t.Errorf("code = %q, want %q", body.Code, CodeCancelled)
	}
	if got := coordinator.inFlightCount(); got != 0 {
		t.Errorf("in-flight slots = %d after cancellation, want 0", got)
	}
}

func TestDownloadHandlerUsesErrorsIsForWrappedFailures(t *testing.T) {
	t.Parallel()

	response := serveDownload(
		&stubDownloader{err: errors.Join(errors.New("detail"), ErrTimeout)},
		missionRequest(http.MethodGet),
	)
	if body := decodeError(t, response); body.Code != CodeTimeout {
		t.Errorf("code = %q, want %q", body.Code, CodeTimeout)
	}
}
