package recording

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// StartHandler starts the single supported active recording.
func StartHandler(store *Store) http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid JSON request", http.StatusBadRequest)
			return
		}
		recording, err := store.StartRecording(r.Context(), body.Name)
		if err != nil {
			if errors.Is(err, ErrRecordingActive) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "could not start recording", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, recording)
	}
}

// StopHandler flushes and stops the active recording.
func StopHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recording, err := store.StopRecording(r.Context())
		if err != nil {
			if errors.Is(err, ErrNoRecording) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "could not stop recording", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, recording)
	}
}

// ListHandler lists recording lifecycle metadata.
func ListHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recordings, err := store.ListRecordings(r.Context())
		if err != nil {
			http.Error(w, "could not list recordings", http.StatusInternalServerError)
			return
		}
		if recordings == nil {
			recordings = []Recording{}
		}
		writeJSON(w, http.StatusOK, recordings)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
