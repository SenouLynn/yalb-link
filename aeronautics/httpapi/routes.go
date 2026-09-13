package httpapi

import (
	"net/http"

	"yalb.aero/api"
)

// The read routes serve discovery. They take no body and no identity: nothing
// they return depends on a candidate.

func discovery(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, service.Discover())
	})
}

func equations(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]api.Equation{"equations": service.Equations()})
	})
}

// equation serves one equation by ID. The IDs contain dots and hyphens and are
// matched with a wildcard rather than re-encoded, so the path carries the
// registry's own identifier.
func equation(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		found, err := service.Equation(r.PathValue("id"))
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeJSON(w, http.StatusOK, found)
	})
}

func patterns(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]api.Pattern{"patterns": service.Patterns()})
	})
}

func units(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]api.UnitInfo{"units": service.Units()})
	})
}

// The evaluation routes carry the whole design and the client's request
// identity, and echo the identity back on the response as well as in the body,
// so a client can match a late answer without parsing it first.

func evaluate(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.EvaluateRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.Evaluate(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeIdentity(w, result.Request)
		writeJSON(w, http.StatusOK, result)
	})
}

func evaluateBatch(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.BatchEvaluateRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.EvaluateBatch(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func apply(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.ApplyRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.Apply(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeIdentity(w, result.Evaluation.Request)
		writeJSON(w, http.StatusOK, result)
	})
}

// sweep answers a sensitivity request. It echoes the identity like the other
// evaluation routes, and the body also carries the input snapshot and the
// settings fingerprint, so a client can tell whether an answer still belongs to
// the question it is asking.
func sweep(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.SweepRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.Sweep(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeIdentity(w, result.Request)
		writeJSON(w, http.StatusOK, result)
	})
}

// powerSearch answers a bounded power search. It echoes the identity like the
// other evaluation routes, and the body carries the input snapshot and the
// settings fingerprint for the same reason a sweep does: an answer must be
// matchable against the question it was asked.
func powerSearch(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.PowerSearchRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.PowerSearch(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeIdentity(w, result.Request)
		writeJSON(w, http.StatusOK, result)
	})
}

func preview(service *api.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeBody[api.PreviewRequest](r)
		if err != nil {
			writeFailure(w, err)
			return
		}
		result, err := service.Preview(r.Context(), request)
		if err != nil {
			writeFailure(w, err)
			return
		}
		writeIdentity(w, result.After.Request)
		writeJSON(w, http.StatusOK, result)
	})
}

// RequestHeader carries the evaluation identity a response answers, so a client
// can discard a stale answer without decoding the body.
const (
	RequestHeader  = "X-Aero-Request"
	SequenceHeader = "X-Aero-Sequence"
)

func writeIdentity(w http.ResponseWriter, request api.Request) {
	w.Header().Set(RequestHeader, request.Session)
	w.Header().Set(SequenceHeader, formatUint(request.Sequence))
}
