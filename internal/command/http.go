package command

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"google.golang.org/protobuf/encoding/protojson"
)

const ArmPattern = "POST /api/commands/arm"

func ArmHandler(registry *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusBadRequest)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				http.Error(w, "cross-origin command rejected", http.StatusBadRequest)
				return
			}
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "cross-site command rejected", http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body struct {
			SystemID uint32 `json:"system_id"`
			Arm      bool   `json:"arm"`
		}
		if err := dec.Decode(&body); err != nil {
			http.Error(w, "invalid command body", http.StatusBadRequest)
			return
		}
		var trailing any
		if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
			http.Error(w, "invalid trailing JSON", http.StatusBadRequest)
			return
		}
		if body.SystemID == 0 || body.SystemID > 255 {
			http.Error(w, "system_id must be between 1 and 255", http.StatusBadRequest)
			return
		}
		tx, err := registry.Arm(r.Context(), uint8(body.SystemID), body.Arm)
		status := http.StatusOK
		switch {
		case errors.Is(err, ErrNoRoute):
			status = http.StatusNotFound
		case errors.Is(err, ErrInFlight), errors.Is(err, ErrPoisoned):
			status = http.StatusConflict
		case errors.Is(err, ErrWriteFailed):
			status = http.StatusBadGateway
		case err != nil:
			status = http.StatusBadRequest
		}
		if tx == nil {
			message := "command failed"
			if errors.Is(err, ErrPoisoned) {
				message = "previous command unresolved — restart the backend"
			}
			http.Error(w, message, status)
			return
		}
		data, marshalErr := protojson.Marshal(tx)
		if marshalErr != nil {
			http.Error(w, "encoding command response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}
