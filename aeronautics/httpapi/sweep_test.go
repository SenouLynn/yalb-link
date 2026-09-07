package httpapi_test

import (
	"net/http"
	"testing"

	"yalb.aero/api"
	"yalb.aero/httpapi"
)

func sweepRequest() api.SweepRequest {
	return api.SweepRequest{
		Request: identity(),
		Design:  wireDesign(),
		Settings: api.SweepSettings{
			Driver:  "wing.aspect_ratio.planform",
			From:    api.Quantity{Value: 4, Unit: "1"},
			To:      api.Quantity{Value: 10, Unit: "1"},
			Samples: 5,
			Output:  api.SweepOutput{Subject: "stall-speed", Case: fixtureCaseName},
		},
	}
}

// The sweep route answers with the identity in a header as well as in the body,
// so a client can discard a superseded answer without decoding it first.
func TestTheSweepRouteEchoesTheIdentityInAHeader(t *testing.T) {
	server := newServer(t)
	response := post(t, server, "/sweep", sweepRequest())
	wantStatus(t, response, http.StatusOK)

	if got := response.Header.Get(httpapi.RequestHeader); got != identity().Session {
		t.Errorf("%s = %q, want %q", httpapi.RequestHeader, got, identity().Session)
	}
	if got := response.Header.Get(httpapi.SequenceHeader); got != "7" {
		t.Errorf("%s = %q, want \"7\"", httpapi.SequenceHeader, got)
	}
	body := decode[api.SweepResponse](t, response)
	if body.Request != identity() {
		t.Errorf("identity in the body = %+v, want %+v", body.Request, identity())
	}
	if len(body.Samples) != 5 {
		t.Fatalf("%d samples, want 5", len(body.Samples))
	}
	if body.Snapshot == "" || body.SettingsFingerprint == "" {
		t.Error("the answer carries neither its input snapshot nor its settings fingerprint")
	}
}

// A refused sweep comes back in the same failure shape every other refusal
// uses, with the field issue that explains it.
func TestARefusedSweepUsesTheBoundaryFailureShape(t *testing.T) {
	server := newServer(t)
	request := sweepRequest()
	request.Settings.Driver = "wing.chord.root"
	response := post(t, server, "/sweep", request)
	wantStatus(t, response, http.StatusBadRequest)

	failure := decode[failureBody](t, response)
	if failure.Error != "invalid" || failure.Message == "" {
		t.Fatalf("failure = %+v", failure)
	}
	found := false
	for _, issue := range failure.Issues {
		if issue.Field == "sweep.driver" {
			found = true
		}
	}
	if !found {
		t.Errorf("no issue on the swept driver: %+v", failure.Issues)
	}
}

// A field the contract does not define is refused rather than ignored, on this
// route like every other.
func TestASweepWithAnUnknownFieldIsRefused(t *testing.T) {
	response := postRaw(t, newServer(t), "/sweep",
		`{"request":{"session":"s","sequence":1},"design":{},"settings":{},"nonsense":1}`,
		"application/json")
	wantStatus(t, response, http.StatusBadRequest)
}

// A sweep over a design the model cannot cover is unprocessable rather than a
// bad request: the client asked a well-formed question this model does not
// answer.
func TestAnUnsupportedSweepDriverIsUnprocessable(t *testing.T) {
	server := newServer(t)
	request := sweepRequest()
	request.Design.Wing.Span = nil
	request.Design.Wing.AspectRatio = 0
	request.Design.Wing.Area = &api.Quantity{Value: 0.24, Unit: "m^2"}
	request.Design.Wing.RootChord = &api.Quantity{Value: 0.2, Unit: "m"}
	request.Settings.Driver = "wing.aspect_ratio.planform"
	response := post(t, server, "/sweep", request)
	wantStatus(t, response, http.StatusBadRequest)
}

// GET is not how a sweep is asked for, and the answer says so with the methods
// the path does accept.
func TestTheSweepRouteRefusesTheWrongMethod(t *testing.T) {
	server := newServer(t)
	response := get(t, server, httpapi.Prefix+"/sweep")
	wantStatus(t, response, http.StatusMethodNotAllowed)
	if allow := response.Header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want %q", allow, http.MethodPost)
	}
}
