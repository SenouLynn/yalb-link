package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"yalb.aero/api"
	"yalb.aero/httpapi"
)

// The HTTP tests exercise the boundary through a real server, because that is
// where the things this layer owns actually happen: status codes, method
// matching, body limits, strict decoding and cancellation.

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(httpapi.Handler(api.NewService()))
	t.Cleanup(server.Close)
	return server
}

// answer is a completed exchange. The helpers read and close the body before
// returning, so a test never holds an open response and the assertions can look
// at the status, the headers and the body together.
type answer struct {
	Header http.Header
	Body   []byte
	Status int
}

// post sends a JSON body to a route under the versioned prefix.
func post(t *testing.T, server *httptest.Server, path string, body any) answer {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}
	return postRaw(t, server, path, string(encoded), "application/json")
}

func postRaw(t *testing.T, server *httptest.Server, path, body, contentType string) answer {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		server.URL+httpapi.Prefix+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return send(t, server, request)
}

func get(t *testing.T, server *httptest.Server, path string) answer {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		server.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	return send(t, server, request)
}

func send(t *testing.T, server *httptest.Server, request *http.Request) answer {
	t.Helper()
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("sending the request: %v", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("closing the body: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	return answer{Status: response.StatusCode, Header: response.Header, Body: body}
}

func decode[T any](t *testing.T, response answer) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response.Body, &value); err != nil {
		t.Fatalf("decoding the body %q: %v", string(response.Body), err)
	}
	return value
}

// failureBody mirrors the shape every refused call returns.
type failureBody struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Issues  []api.Issue `json:"issues"`
}

func wantStatus(t *testing.T, response answer, want int) {
	t.Helper()
	if response.Status != want {
		t.Fatalf("status = %d, want %d (body %q)", response.Status, want, string(response.Body))
	}
	if media := response.Header.Get("Content-Type"); !strings.HasPrefix(media, "application/json") {
		t.Errorf("content type = %q, want JSON", media)
	}
}

// TestEvaluateOverHTTPMatchesTheCoreResult is the success path: a design goes
// out as JSON and comes back with the same numbers a direct core call produces,
// its provenance intact and its identity echoed in the headers.
func TestEvaluateOverHTTPMatchesTheCoreResult(t *testing.T) {
	server := newServer(t)
	request := evaluateRequest()
	response := post(t, server, "/evaluate", request)
	wantStatus(t, response, http.StatusOK)

	if got := response.Header.Get(httpapi.RequestHeader); got != request.Request.Session {
		t.Errorf("%s = %q, want %q", httpapi.RequestHeader, got, request.Request.Session)
	}
	want := strconv.FormatUint(request.Request.Sequence, 10)
	if got := response.Header.Get(httpapi.SequenceHeader); got != want {
		t.Errorf("%s = %q, want %q", httpapi.SequenceHeader, got, want)
	}

	wantSameAsDirect(t, decode[api.Evaluation](t, response), request)
}

// wantSameAsDirect compares an evaluation that crossed HTTP against the one a
// direct call to the same service produces.
func wantSameAsDirect(t *testing.T, evaluation api.Evaluation, request api.EvaluateRequest) {
	t.Helper()
	direct, err := api.NewService().Evaluate(t.Context(), request)
	if err != nil {
		t.Fatalf("direct call: %v", err)
	}
	if evaluation.Snapshot != direct.Snapshot {
		t.Error("HTTP and a direct call disagree about the input snapshot")
	}
	if evaluation.Geometry != "computed" {
		t.Fatalf("geometry = %q: %+v", evaluation.Geometry, evaluation.GeometryIssues)
	}
	if len(evaluation.Checks) != len(direct.Checks) {
		t.Fatalf("check counts differ: %d and %d", len(evaluation.Checks), len(direct.Checks))
	}
	if evaluation.Checks[0].Actual == nil ||
		evaluation.Checks[0].Actual.Value != direct.Checks[0].Actual.Value {
		t.Errorf("stall speed over HTTP = %+v, want %+v",
			evaluation.Checks[0].Actual, direct.Checks[0].Actual)
	}
	if evaluation.Checks[0].Trace == nil || evaluation.Checks[0].Trace.EquationID == "" {
		t.Error("the trace did not survive the transport")
	}
	if evaluation.AreaLower.Value == nil || evaluation.AreaLower.Value.Value != direct.AreaLower.Value.Value {
		t.Error("the area bound did not survive the transport")
	}
}

// TestBadJSONIsRejected covers a body that is not JSON at all, a body with more
// than one value in it, and a body of the wrong media type.
func TestBadJSONIsRejected(t *testing.T) {
	server := newServer(t)
	for name, tc := range map[string]struct {
		body        string
		contentType string
		errorToken  string
		status      int
	}{
		"not json":         {body: "{not json", contentType: "application/json", status: http.StatusBadRequest, errorToken: "malformed"},
		"trailing value":   {body: `{"request":{"session":"a","sequence":1}} {}`, contentType: "application/json", status: http.StatusBadRequest, errorToken: "malformed"},
		"wrong media":      {body: "session=a", contentType: "application/x-www-form-urlencoded", status: http.StatusBadRequest, errorToken: "invalid"},
		"empty body":       {body: "", contentType: "application/json", status: http.StatusBadRequest, errorToken: "malformed"},
		"array not object": {body: "[]", contentType: "application/json", status: http.StatusBadRequest, errorToken: "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			response := postRaw(t, server, "/evaluate", tc.body, tc.contentType)
			wantStatus(t, response, tc.status)
			failure := decode[failureBody](t, response)
			if failure.Error != tc.errorToken {
				t.Errorf("error = %q, want %q (message %q)", failure.Error, tc.errorToken, failure.Message)
			}
			if failure.Message == "" {
				t.Error("a refusal should explain itself")
			}
		})
	}
}

// TestUnknownFieldsAreRejected holds that a field the contract does not define
// is refused rather than ignored. Ignoring it would let a client's typo look
// like a call that succeeded and quietly did something else.
func TestUnknownFieldsAreRejected(t *testing.T) {
	server := newServer(t)
	body := `{"request":{"session":"a","sequence":1},"design":{"name":"x"},"extra":true}`
	response := postRaw(t, server, "/evaluate", body, "application/json")
	wantStatus(t, response, http.StatusBadRequest)
	failure := decode[failureBody](t, response)
	if len(failure.Issues) != 1 || failure.Issues[0].Field != "extra" {
		t.Errorf("issues = %+v, want one naming the unknown field", failure.Issues)
	}
}

// TestUnknownModeAndEquationAreNotFound separates an unregistered equation and
// an unrouted path from a bad request.
func TestUnknownModeAndEquationAreNotFound(t *testing.T) {
	server := newServer(t)
	for name, path := range map[string]string{
		"unknown equation": httpapi.Prefix + "/equations/lift.does-not-exist",
		"unknown route":    httpapi.Prefix + "/solve",
		"unversioned path": "/api/evaluate",
	} {
		t.Run(name, func(t *testing.T) {
			response := get(t, server, path)
			wantStatus(t, response, http.StatusNotFound)
			if decode[failureBody](t, response).Error != "not-found" {
				t.Error("a missing resource should report itself as not found")
			}
		})
	}
}

// TestWrongMethodIsRefused holds that a read route does not accept a write and
// the other way round.
func TestWrongMethodIsRefused(t *testing.T) {
	server := newServer(t)
	response := get(t, server, httpapi.Prefix+"/evaluate")
	wantStatus(t, response, http.StatusMethodNotAllowed)
	if allow := response.Header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want %q", allow, http.MethodPost)
	}
	if decode[failureBody](t, response).Message == "" {
		t.Error("a method refusal should say what the path does accept")
	}
}

// TestInvalidNumericDataIsRefusedWithItsField covers the numeric shapes a client
// can get wrong, each naming the field it is about.
func TestInvalidNumericDataIsRefusedWithItsField(t *testing.T) {
	server := newServer(t)
	for name, tc := range map[string]struct {
		mutate func(*api.EvaluateRequest)
		field  string
	}{
		"unknown unit": {
			mutate: func(r *api.EvaluateRequest) { r.Design.Mass = &api.Quantity{Value: 2, Unit: "stone"} },
			field:  "design.mass",
		},
		"missing identity": {
			mutate: func(r *api.EvaluateRequest) { r.Request.Session = "" },
			field:  "request.session",
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := evaluateRequest()
			tc.mutate(&request)
			response := post(t, server, "/evaluate", request)
			if response.Status == http.StatusOK {
				t.Fatal("the request should have been refused")
			}
			failure := decode[failureBody](t, response)
			found := false
			for _, issue := range failure.Issues {
				if issue.Field == tc.field {
					found = true
				}
			}
			if !found {
				t.Errorf("issues = %+v, want one on %q", failure.Issues, tc.field)
			}
		})
	}
}

// TestNegativeSpanIsADomainFailureNotARequestFailure records where the line
// between the two sits. A negative span is a well-formed request — a length may
// be negative in general, which is what makes anhedral and forward sweep
// expressible — so the boundary passes it through and the core refuses it as an
// invalid driver. It comes back as an evaluation carrying that issue, because
// the candidate is still editable and a worksheet has to show it.
func TestNegativeSpanIsADomainFailureNotARequestFailure(t *testing.T) {
	server := newServer(t)
	request := evaluateRequest()
	request.Design.Wing.Span = &api.Quantity{Value: -1.2, Unit: "m"}
	response := post(t, server, "/evaluate", request)
	wantStatus(t, response, http.StatusOK)
	evaluation := decode[api.Evaluation](t, response)
	if evaluation.Geometry != "invalid" {
		t.Fatalf("geometry = %q, want invalid", evaluation.Geometry)
	}
	found := false
	for _, issue := range evaluation.GeometryIssues {
		if issue.Field == "span" && issue.Kind == "invalid" {
			found = true
		}
	}
	if !found {
		t.Errorf("geometry issues = %+v, want an invalid span", evaluation.GeometryIssues)
	}
}

// TestOversizedBodyIsRefused holds the transport's own payload limit, which is
// separate from the api package's count limits.
func TestOversizedBodyIsRefused(t *testing.T) {
	server := newServer(t)
	body := `{"request":{"session":"` + strings.Repeat("a", httpapi.MaxRequestBytes) + `","sequence":1}}`
	response := postRaw(t, server, "/evaluate", body, "application/json")
	wantStatus(t, response, http.StatusRequestEntityTooLarge)
	if decode[failureBody](t, response).Error != "too-large" {
		t.Error("an oversized body should report itself as too large")
	}
}

// TestStructuredDomainFailureIsAnOkResult holds that a design that does not
// solve comes back as a 200 with its issues, rather than as an HTTP error. A
// worksheet has to render an unfinished candidate.
func TestStructuredDomainFailureIsAnOkResult(t *testing.T) {
	server := newServer(t)
	request := evaluateRequest()
	request.Design.Wing.Shape = "trapezoid"
	response := post(t, server, "/evaluate", request)
	wantStatus(t, response, http.StatusOK)
	evaluation := decode[api.Evaluation](t, response)
	if evaluation.Geometry == "computed" {
		t.Fatal("a trapezoid with no taper ratio must not solve")
	}
	if len(evaluation.GeometryIssues) == 0 {
		t.Fatal("the evaluation should carry the core's typed issues")
	}
	if evaluation.GeometryIssues[0].Kind != "missing" {
		t.Errorf("issue kind = %q, want missing", evaluation.GeometryIssues[0].Kind)
	}
}

// TestUnsupportedInputIsUnprocessable holds that "this model does not cover
// your design" is a different status from "your request is malformed".
func TestUnsupportedInputIsUnprocessable(t *testing.T) {
	server := newServer(t)
	request := evaluateRequest()
	for len(request.Design.Cases) <= api.MaxCases {
		extra := request.Design.Cases[0]
		extra.Name += "+"
		request.Design.Cases = append(request.Design.Cases, extra)
	}
	response := post(t, server, "/evaluate", request)
	wantStatus(t, response, http.StatusUnprocessableEntity)
	if decode[failureBody](t, response).Error != "unsupported" {
		t.Error("a limit the model states should report itself as unsupported")
	}
}

// TestBatchEvaluationIsBounded holds the published batch limit at the transport.
func TestBatchEvaluationIsBounded(t *testing.T) {
	server := newServer(t)
	batch := api.BatchEvaluateRequest{}
	for len(batch.Evaluations) <= api.MaxBatch {
		batch.Evaluations = append(batch.Evaluations, evaluateRequest())
	}
	response := post(t, server, "/evaluate-batch", batch)
	wantStatus(t, response, http.StatusRequestEntityTooLarge)

	ok := api.BatchEvaluateRequest{Evaluations: []api.EvaluateRequest{evaluateRequest(), evaluateRequest()}}
	ok.Evaluations[1].Request.Sequence = 8
	response = post(t, server, "/evaluate-batch", ok)
	wantStatus(t, response, http.StatusOK)
	result := decode[api.BatchEvaluateResponse](t, response)
	if len(result.Evaluations) != 2 {
		t.Fatalf("evaluations = %d, want 2", len(result.Evaluations))
	}
	if result.Evaluations[0].Request.Sequence == result.Evaluations[1].Request.Sequence {
		t.Error("each batch entry keeps its own identity")
	}
}

// TestCancelledRequestStopsTheWork holds that a caller who goes away is not
// answered with a success. The client sees a transport error; what matters here
// is that the handler noticed rather than finishing and pretending.
func TestCancelledRequestStopsTheWork(t *testing.T) {
	server := newServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	encoded, err := json.Marshal(evaluateRequest())
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		server.URL+httpapi.Prefix+"/evaluate", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	cancel()
	response, err := server.Client().Do(request) //nolint:bodyclose // the call fails, so there is no body
	if err == nil {
		_ = response.Body.Close()
		t.Error("a cancelled request should not return a successful response")
	}
}

// TestDiscoveryServesTheContract holds that a client can learn the contract from
// the service rather than from documentation that can drift from it.
func TestDiscoveryServesTheContract(t *testing.T) {
	server := newServer(t)
	response := get(t, server, httpapi.Prefix+"/discovery")
	wantStatus(t, response, http.StatusOK)
	discovery := decode[api.Discovery](t, response)
	if discovery.ContractVersion != api.ContractVersion {
		t.Errorf("contract version = %q, want %q", discovery.ContractVersion, api.ContractVersion)
	}
	if len(discovery.Equations) == 0 || len(discovery.Patterns) == 0 ||
		len(discovery.Units) == 0 || len(discovery.Vocabularies) == 0 {
		t.Fatalf("discovery is incomplete: %d equations, %d patterns, %d units, %d vocabularies",
			len(discovery.Equations), len(discovery.Patterns),
			len(discovery.Units), len(discovery.Vocabularies))
	}
	if discovery.Limits.MaxBatch != api.MaxBatch {
		t.Errorf("published batch limit = %d, want the enforced %d",
			discovery.Limits.MaxBatch, api.MaxBatch)
	}

	// One equation by ID, with its source record intact.
	response = get(t, server, httpapi.Prefix+"/equations/lift.stall-speed")
	wantStatus(t, response, http.StatusOK)
	equation := decode[api.Equation](t, response)
	if equation.ID != "lift.stall-speed" || equation.Source.Kind == "" || equation.Expression == "" {
		t.Errorf("equation = %+v, want the stall-speed relation with its provenance", equation)
	}
}

// TestApplyAndPreviewOverHTTP exercises the write paths end to end.
func TestApplyAndPreviewOverHTTP(t *testing.T) {
	server := newServer(t)
	size := api.Command{
		Kind:  api.CmdSizeAtStallLimit,
		Hold:  "wing.span.projected",
		Scope: &api.Scope{Kind: "single", Case: fixtureCaseName},
	}

	previewResponse := post(t, server, "/preview", api.PreviewRequest{
		Request: identity(), Design: wireDesign(), Command: size,
	})
	wantStatus(t, previewResponse, http.StatusOK)
	preview := decode[api.PreviewResponse](t, previewResponse)
	if len(preview.Changes) != 1 || preview.Changes[0].To != "met" {
		t.Errorf("preview changes = %+v, want the ceiling becoming met", preview.Changes)
	}

	applyResponse := post(t, server, "/apply", api.ApplyRequest{
		Request: identity(), Design: wireDesign(), Commands: []api.Command{size},
	})
	wantStatus(t, applyResponse, http.StatusOK)
	applied := decode[api.ApplyResponse](t, applyResponse)
	if len(applied.Applied) != 1 {
		t.Fatalf("applied = %v, want the one command", applied.Applied)
	}
	if applied.Design.Wing.Area == nil {
		t.Fatal("the edited design carries no area driver")
	}
	if applied.Evaluation.Snapshot == preview.Before.Snapshot {
		t.Error("applying the command should have changed the definition")
	}
	if applied.Evaluation.Snapshot != preview.After.Snapshot {
		t.Error("the preview and the apply describe different results")
	}

	// A refused command is a bad request, and nothing was applied.
	refused := post(t, server, "/apply", api.ApplyRequest{
		Request: identity(), Design: wireDesign(),
		Commands: []api.Command{{Kind: api.CmdRemoveCase, Name: fixtureCaseName}},
	})
	wantStatus(t, refused, http.StatusBadRequest)
	if len(decode[failureBody](t, refused).Issues) == 0 {
		t.Error("a refused command should carry the core's field issues")
	}
}
