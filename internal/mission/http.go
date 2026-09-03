package mission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

// DownloadPattern is the read-only ordinary-mission endpoint.
const DownloadPattern = "GET /api/vehicles/{system_id}/{component_id}/mission"

// Stable transport error codes. Callers should branch on these rather than on
// the human-readable message or the HTTP status alone.
const (
	CodeInvalidVehicle = "mission_invalid_vehicle"
	CodeNoRoute        = "mission_no_route"
	CodeInFlight       = "mission_download_in_progress"
	CodeTimeout        = "mission_download_timeout"
	CodeCancelled      = "mission_download_cancelled"
	CodeRejected       = "mission_download_rejected"
	CodeWriteFailed    = "mission_link_write_failed"
	CodeInternal       = "mission_internal_error"
)

// statusClientClosedRequest is the conventional status for a request whose
// client disconnected. net/http deliberately does not define it.
const statusClientClosedRequest = 499

// Downloader is the part of Coordinator used by the HTTP transport.
type Downloader interface {
	Download(context.Context, codec.Target, uint32) (*gcsv1.MissionSnapshot, error)
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  string `json:"result,omitempty"`
}

// DownloadHandler downloads the selected component's ordinary mission.
func DownloadHandler(d Downloader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		systemID, ok := parseMAVLinkID(r.PathValue("system_id"))
		if !ok || systemID == 0 {
			writeHTTPError(w, http.StatusBadRequest, CodeInvalidVehicle,
				"system_id must be between 1 and 255", "")
			return
		}

		componentID, ok := parseMAVLinkID(r.PathValue("component_id"))
		if !ok || componentID == 0 {
			writeHTTPError(w, http.StatusBadRequest, CodeInvalidVehicle,
				"component_id must be between 1 and 255", "")
			return
		}

		snapshot, err := d.Download(r.Context(), codec.Target{
			SystemID: systemID, ComponentID: componentID,
		}, codec.MissionTypeMission)
		if err != nil {
			writeDownloadError(w, err)
			return
		}
		if snapshot == nil {
			writeHTTPError(w, http.StatusInternalServerError, CodeInternal,
				"mission download returned no snapshot", "")
			return
		}

		data, err := protojson.Marshal(snapshot)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, CodeInternal,
				"could not encode mission snapshot", "")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func parseMAVLinkID(raw string) (uint8, bool) {
	value, err := strconv.ParseUint(raw, 10, 8)
	return uint8(value), err == nil
}

func writeDownloadError(w http.ResponseWriter, err error) {
	var rejected *RejectedError

	switch {
	case errors.Is(err, ErrNoRoute):
		writeHTTPError(w, http.StatusNotFound, CodeNoRoute, "no route to vehicle", "")
	case errors.Is(err, ErrInFlight):
		writeHTTPError(w, http.StatusConflict, CodeInFlight,
			"a mission download is already in progress for this vehicle", "")
	case errors.Is(err, ErrTimeout):
		writeHTTPError(w, http.StatusGatewayTimeout, CodeTimeout,
			"vehicle stopped responding during mission download", "")
	case errors.Is(err, ErrCancelled):
		writeHTTPError(w, statusClientClosedRequest, CodeCancelled,
			"mission download was cancelled", "")
	case errors.As(err, &rejected):
		writeHTTPError(w, http.StatusUnprocessableEntity, CodeRejected,
			"vehicle rejected the mission download", rejected.Result.String())
	case errors.Is(err, ErrWriteFailed):
		writeHTTPError(w, http.StatusBadGateway, CodeWriteFailed,
			"could not write mission request to vehicle link", "")
	default:
		writeHTTPError(w, http.StatusInternalServerError, CodeInternal,
			"mission download failed", "")
	}
}

func writeHTTPError(w http.ResponseWriter, status int, code, message, result string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Code: code, Message: message, Result: result})
}
