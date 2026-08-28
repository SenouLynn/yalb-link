package command

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"google.golang.org/protobuf/encoding/protojson"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

const (
	ArmPattern     = "POST /api/commands/arm"
	ResolvePattern = "POST /api/commands/arm/resolve"
)

// Response codes for the two 409s a caller must tell apart. A poisoned key
// needs an attestation; a quarantined one only needs waiting.
const (
	CodeUnresolved  = "command_unresolved"
	CodeQuarantined = "command_quarantined"
)

// Wire vocabulary for the observed arm state. Deliberately not a bool: an
// omitted field would decode as false, turning "no attestation" into "the
// operator observed it disarmed".
const (
	observedArmed    = "ARMED"
	observedDisarmed = "DISARMED"
)

// guard applies the checks shared by both command routes.
//
// These are local-development CSRF defenses. They are not authentication and
// they do not identify the caller; see ADR 0004.
func guard(w http.ResponseWriter, r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusBadRequest)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			http.Error(w, "cross-origin command rejected", http.StatusBadRequest)
			return false
		}
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "cross-site command rejected", http.StatusBadRequest)
		return false
	}
	return true
}

// decodeBody reads one bounded, strict JSON object and rejects anything after it.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		http.Error(w, "invalid command body", http.StatusBadRequest)
		return false
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid trailing JSON", http.StatusBadRequest)
		return false
	}
	return true
}

func ArmHandler(registry *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var body struct {
			SystemID uint32 `json:"system_id"`
			Arm      bool   `json:"arm"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		if body.SystemID == 0 || body.SystemID > 255 {
			http.Error(w, "system_id must be between 1 and 255", http.StatusBadRequest)
			return
		}

		tx, err := registry.Arm(r.Context(), uint8(body.SystemID), body.Arm)

		var quarantined *QuarantineError
		switch {
		case errors.As(err, &quarantined):
			writeJSON(w, http.StatusConflict, map[string]any{
				"code": CodeQuarantined,
				// Relative on purpose: no caller decides expiry from its own clock.
				"retry_after_ms": quarantined.RetryAfter.Milliseconds(),
			})
			return
		case errors.Is(err, ErrPoisoned):
			// The retained transaction rides along so a reloaded page learns
			// which ambiguity it has to resolve.
			data, marshalErr := protojson.Marshal(tx)
			if marshalErr != nil {
				http.Error(w, "encoding command response", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"code":        CodeUnresolved,
				"transaction": json.RawMessage(data),
			})
			return
		case errors.Is(err, ErrInFlight):
			http.Error(w, "a command is already in flight for this vehicle", http.StatusConflict)
			return
		case errors.Is(err, ErrNoRoute):
			http.Error(w, "no route to vehicle", http.StatusNotFound)
			return
		}

		status := http.StatusOK
		if errors.Is(err, ErrWriteFailed) {
			status = http.StatusBadGateway
		}
		if tx == nil {
			http.Error(w, "command failed", http.StatusBadRequest)
			return
		}
		writeTransaction(w, status, tx)
	}
}

// ResolveHandler records an operator's attestation about an ambiguous
// arm/disarm transaction.
func ResolveHandler(registry *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		var body struct {
			SystemID      uint32 `json:"system_id"`
			RegistryEpoch string `json:"registry_epoch"`
			TransactionID uint32 `json:"transaction_id"`
			ObservedState string `json:"observed_state"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		if body.SystemID == 0 || body.SystemID > 255 {
			http.Error(w, "system_id must be between 1 and 255", http.StatusBadRequest)
			return
		}
		if body.RegistryEpoch == "" {
			http.Error(w, "registry_epoch is required", http.StatusBadRequest)
			return
		}
		if body.TransactionID == 0 {
			http.Error(w, "transaction_id is required", http.StatusBadRequest)
			return
		}
		observed, ok := parseObservedState(body.ObservedState)
		if !ok {
			http.Error(w,
				"observed_state must be "+observedArmed+" or "+observedDisarmed,
				http.StatusBadRequest)
			return
		}

		tx, err := registry.ResolveArm(
			r.Context(), uint8(body.SystemID), body.RegistryEpoch, body.TransactionID, observed)
		switch {
		case errors.Is(err, ErrNotPoisoned):
			http.Error(w, "no unresolved command for this vehicle", http.StatusConflict)
			return
		case errors.Is(err, ErrStaleRef):
			http.Error(w, "transaction reference does not match the unresolved command", http.StatusConflict)
			return
		case errors.Is(err, ErrInFlight):
			http.Error(w, "a command is already in flight for this vehicle", http.StatusConflict)
			return
		case err != nil:
			http.Error(w, "resolution rejected", http.StatusBadRequest)
			return
		}
		writeTransaction(w, http.StatusOK, tx)
	}
}

// parseObservedState accepts only an explicit attestation.
func parseObservedState(raw string) (gcsv1.ArmState, bool) {
	switch raw {
	case observedArmed:
		return gcsv1.ArmState_ARM_STATE_ARMED, true
	case observedDisarmed:
		return gcsv1.ArmState_ARM_STATE_DISARMED, true
	default:
		return gcsv1.ArmState_ARM_STATE_UNSPECIFIED, false
	}
}

func writeTransaction(w http.ResponseWriter, status int, tx *gcsv1.CommandTransaction) {
	data, err := protojson.Marshal(tx)
	if err != nil {
		http.Error(w, "encoding command response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "encoding command response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
