// Package httpapi serves yalb.aero/api over HTTP. It owns everything HTTP has
// and the application boundary does not: routing, methods, content types, body
// limits, status codes and JSON serialization.
//
// It contains no equation, no requirement rule and no unit conversion. Every
// handler decodes a request, hands it to the api service and maps the result or
// the failure onto a status code; a physical answer never depends on which
// transport asked for it.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"yalb.aero/api"
)

// MaxRequestBytes bounds a request body. It is generous for the largest design
// the contract allows — 32 cases and 64 requirements — and small enough that an
// unbounded upload cannot be turned into memory pressure.
const MaxRequestBytes = 1 << 20

// Prefix is the versioned path every route lives under.
const Prefix = "/api/" + api.ContractVersion

// Handler returns the HTTP routes for the boundary service. The returned
// handler is safe for concurrent use: the service holds no state, and each
// request carries its whole design.
func Handler(service *api.Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+Prefix+"/discovery", discovery(service))
	mux.Handle("GET "+Prefix+"/equations", equations(service))
	mux.Handle("GET "+Prefix+"/equations/{id...}", equation(service))
	mux.Handle("GET "+Prefix+"/patterns", patterns(service))
	mux.Handle("GET "+Prefix+"/units", units(service))
	mux.Handle("POST "+Prefix+"/evaluate", evaluate(service))
	mux.Handle("POST "+Prefix+"/evaluate-batch", evaluateBatch(service))
	mux.Handle("POST "+Prefix+"/apply", apply(service))
	mux.Handle("POST "+Prefix+"/preview", preview(service))
	return withLimit(mux)
}

// withLimit caps every request body before a handler reads it, and answers an
// unrouted path with the boundary's own error shape rather than net/http's
// plain-text default, so a client never has to parse two failure formats.
func withLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.wrote {
			return
		}
		// ServeMux answered nothing of its own only when no route matched at
		// all; its 404 and 405 both write, so this is the not-found case.
		writeFailure(w, &api.Failure{
			Kind:    api.FailureNotFound,
			Message: "no route serves " + r.Method + " " + r.URL.Path,
		})
	})
}

// statusRecorder notices whether a handler wrote anything, so the not-found
// fallback does not overwrite a real response.
type statusRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (s *statusRecorder) WriteHeader(status int) {
	s.wrote = true
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wrote = true
	n, err := s.ResponseWriter.Write(b)
	if err != nil {
		return n, err //nolint:wrapcheck // passthrough of the underlying writer's error
	}
	return n, nil
}

// errorBody is the failure shape every refused call returns, whatever went
// wrong and wherever it was found.
type errorBody struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Issues  []api.Issue `json:"issues,omitempty"`
}

// statusFor maps a boundary failure kind onto an HTTP status. This is the only
// place in the system that knows what a status code is.
func statusFor(kind api.FailureKind) int {
	switch kind {
	case api.FailureMalformed, api.FailureInvalid:
		return http.StatusBadRequest
	case api.FailureNotFound:
		return http.StatusNotFound
	case api.FailureUnsupported:
		return http.StatusUnprocessableEntity
	case api.FailureTooLarge:
		return http.StatusRequestEntityTooLarge
	case api.FailureCancelled:
		// The caller is gone, so nothing reads this; 499 records why in the
		// server's own logs rather than claiming the work succeeded.
		return 499
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	// The response is fully built before any byte is written, so an encoding
	// failure here is a broken connection rather than a half-formed answer.
	_ = json.NewEncoder(w).Encode(body)
}

func writeFailure(w http.ResponseWriter, err error) {
	failure, ok := api.AsFailure(err)
	if !ok {
		failure = &api.Failure{Kind: api.FailureInvalid, Message: err.Error()}
	}
	writeJSON(w, statusFor(failure.Kind), errorBody{
		Error:   failure.Kind.String(),
		Message: failure.Message,
		Issues:  failure.Issues,
	})
}

// decodeBody reads a JSON request body strictly: the content type must be JSON,
// unknown fields are refused rather than ignored, the body must hold exactly one
// value, and an oversized body is reported as too large rather than as
// malformed. Rejecting an unknown field is what stops a client's typo from
// looking like a successful call that quietly did something else.
func decodeBody[T any](r *http.Request) (T, error) {
	var value T
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		if media, _, _ := strings.Cut(contentType, ";"); strings.TrimSpace(media) != "application/json" {
			return value, &api.Failure{
				Kind:    api.FailureInvalid,
				Message: "the request body must be application/json, received " + contentType,
			}
		}
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, decodeFailure(err)
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return value, &api.Failure{
			Kind:    api.FailureMalformed,
			Message: "the request body must hold exactly one JSON value",
		}
	}
	return value, nil
}

// decodeFailure classifies a decoding error. An oversized body and a syntax
// error are different answers, and so is a field the contract does not define.
func decodeFailure(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &api.Failure{
			Kind:    api.FailureTooLarge,
			Message: "the request body exceeds the limit of " + strconv.Itoa(MaxRequestBytes) + " bytes",
		}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "(root)"
		}
		return &api.Failure{
			Kind:    api.FailureInvalid,
			Message: "the request body could not be read",
			Issues: []api.Issue{{
				Field:  field,
				Kind:   "invalid",
				Detail: "expected " + typeErr.Type.String() + " but received a JSON " + typeErr.Value,
			}},
		}
	}
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		return &api.Failure{
			Kind:    api.FailureInvalid,
			Message: "the request body could not be read",
			Issues: []api.Issue{{
				Field:  strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`),
				Kind:   "invalid",
				Detail: "the contract defines no such field; the discovery document lists what it accepts",
			}},
		}
	}
	return &api.Failure{Kind: api.FailureMalformed, Message: "the request body is not valid JSON: " + err.Error()}
}

// formatUint renders a request sequence for the identity header.
func formatUint(n uint64) string { return strconv.FormatUint(n, 10) }
