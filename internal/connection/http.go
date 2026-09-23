package connection

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
)

// Mount registers acquisition routes independently of operator-command policy.
func Mount(mux *http.ServeMux, m *Manager) {
	mux.HandleFunc("GET /api/connections", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, m.Connections()) })
	mux.HandleFunc("GET /api/connections/devices", func(w http.ResponseWriter, r *http.Request) {
		devices, err := m.Devices(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, devices)
	})
	mux.HandleFunc("POST /api/connections/connect", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceID  string   `json:"device_id"`
			ProfileID string   `json:"profile_id"`
			Settings  Settings `json:"settings"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		var status Status
		var err error
		switch {
		case body.ProfileID != "":
			status, err = m.ConnectProfile(r.Context(), body.ProfileID, body.DeviceID, body.Settings)
		case body.DeviceID != "":
			status, err = m.Connect(r.Context(), body.DeviceID, body.Settings)
		default:
			http.Error(w, "device_id or profile_id is required", 400)
			return
		}
		code := http.StatusOK
		switch {
		case errors.Is(err, ErrMissing), errors.Is(err, ErrProfileNotFound):
			code = 404
		case errors.Is(err, ErrConflict), errors.Is(err, ErrAmbiguous):
			code = 409
		case errors.Is(err, ErrSettings), errors.Is(err, ErrProfilesDisabled):
			code = 400
		case errors.Is(err, ErrClosed):
			code = 503
		case err != nil:
			switch status.ErrorCode {
			case "busy":
				code = 409
			case "permission_denied":
				code = 403
			case "missing":
				code = 404
			case "invalid_settings":
				code = 400
			default:
				code = 500
			}
		}
		if err != nil {
			writeJSON(w, code, map[string]any{"error": err.Error(), "status": status})
			return
		}
		writeJSON(w, code, status)
	})
	mux.HandleFunc("POST /api/connections/disconnect", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		if body.ID == "" {
			http.Error(w, "id is required", 400)
			return
		}
		if err := m.Disconnect(r.Context(), body.ID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]bool{"disconnected": true})
	})
	mux.HandleFunc("GET /api/connections/profiles", func(w http.ResponseWriter, r *http.Request) {
		profiles, err := m.Profiles()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if profiles == nil {
			profiles = []Profile{}
		}
		writeJSON(w, 200, profiles)
	})
	mux.HandleFunc("POST /api/connections/profiles", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			DeviceID string   `json:"device_id"`
			Settings Settings `json:"settings"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		if body.DeviceID == "" {
			http.Error(w, "device_id is required", 400)
			return
		}
		profile, err := m.SaveProfile(r.Context(), Profile{ID: body.ID, Name: body.Name, Device: Device{ID: body.DeviceID}, Settings: body.Settings})
		if err != nil {
			code := 500
			switch {
			case errors.Is(err, ErrMissing), errors.Is(err, ErrProfileNotFound):
				code = 404
			case errors.Is(err, ErrSettings), errors.Is(err, ErrProfileInvalid), errors.Is(err, ErrProfilesDisabled):
				code = 400
			}
			writeJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, profile)
	})
	mux.HandleFunc("DELETE /api/connections/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !guardOrigin(w, r) {
			return
		}
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", 400)
			return
		}
		if err := m.DeleteProfile(id); err != nil {
			code := 500
			switch {
			case errors.Is(err, ErrProfileNotFound):
				code = 404
			case errors.Is(err, ErrProfilesDisabled):
				code = 400
			}
			writeJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"deleted": true})
	})
}

// guardOrigin rejects a cross-origin/cross-site request to a mutating route.
// Factored out of decodeBody so a body-less mutation (profile delete) can
// apply the same same-origin guard.
func guardOrigin(w http.ResponseWriter, r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			http.Error(w, "cross-origin connection action rejected", 400)
			return false
		}
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "cross-site connection action rejected", 400)
		return false
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request, body any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "content type must be application/json", 400)
		return false
	}
	if !guardOrigin(w, r) {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		http.Error(w, "invalid connection body", 400)
		return false
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid trailing JSON", 400)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
