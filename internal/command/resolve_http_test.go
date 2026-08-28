package command

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

func post(t *testing.T, handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestResolveHandlerRecordsAttestation(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)

	body := fmt.Sprintf(`{"system_id":1,"registry_epoch":%q,"transaction_id":%d,"observed_state":"DISARMED"}`,
		testEpoch, first.GetId())
	rec := post(t, ResolveHandler(f.registry), "/api/commands/arm/resolve", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	tx := &gcsv1.CommandTransaction{}
	if err := protojson.Unmarshal(rec.Body.Bytes(), tx); err != nil {
		t.Fatal(err)
	}
	if tx.GetResolution().GetObservedState() != gcsv1.ArmState_ARM_STATE_DISARMED {
		t.Fatalf("observed_state = %v", tx.GetResolution().GetObservedState())
	}
	if tx.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Fatalf("terminal state was rewritten to %v", tx.GetState())
	}
}

func TestResolveHandlerRejectsIncompleteAttestations(t *testing.T) {
	tests := []struct{ name, body string }{
		// The one that matters most: a bool would have decoded this as
		// "observed disarmed" from a field the operator never sent.
		{"omitted observed_state", `{"system_id":1,"registry_epoch":"e","transaction_id":7}`},
		{"empty observed_state", `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":""}`},
		{"unknown observed_state", `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":"MAYBE"}`},
		{"lowercase observed_state", `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":"disarmed"}`},
		{"omitted transaction_id", `{"system_id":1,"registry_epoch":"e","observed_state":"ARMED"}`},
		{"omitted registry_epoch", `{"system_id":1,"transaction_id":7,"observed_state":"ARMED"}`},
		{"omitted system_id", `{"registry_epoch":"e","transaction_id":7,"observed_state":"ARMED"}`},
		{"unknown field", `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":"ARMED","force":true}`},
		{"trailing JSON", `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":"ARMED"}{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := resolveFixtureFor(t, &capture{}, silentWrite)
			timedOutArm(t, f)
			rec := post(t, ResolveHandler(f.registry), "/api/commands/arm/resolve", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			// The ambiguity must survive a rejected attestation.
			if _, err := f.registry.Arm(context.Background(), 1, true); err == nil {
				t.Fatal("key was cleared by a rejected attestation")
			}
		})
	}
}

func TestResolveHandlerConflicts(t *testing.T) {
	t.Run("no ambiguity", func(t *testing.T) {
		f := resolveFixtureFor(t, &capture{}, silentWrite)
		body := fmt.Sprintf(`{"system_id":1,"registry_epoch":%q,"transaction_id":1,"observed_state":"ARMED"}`, testEpoch)
		rec := post(t, ResolveHandler(f.registry), "/api/commands/arm/resolve", body)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("stale epoch", func(t *testing.T) {
		f := resolveFixtureFor(t, &capture{}, silentWrite)
		first := timedOutArm(t, f)
		body := fmt.Sprintf(`{"system_id":1,"registry_epoch":"other","transaction_id":%d,"observed_state":"ARMED"}`,
			first.GetId())
		rec := post(t, ResolveHandler(f.registry), "/api/commands/arm/resolve", body)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestArmHandlerDistinguishesUnresolvedFromQuarantined(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	first := timedOutArm(t, f)

	// Poisoned: the response must name the transaction to resolve.
	rec := post(t, ArmHandler(f.registry), "/api/commands/arm", `{"system_id":1,"arm":true}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("poisoned status = %d", rec.Code)
	}
	var poisoned struct {
		Code        string          `json:"code"`
		Transaction json.RawMessage `json:"transaction"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &poisoned); err != nil {
		t.Fatal(err)
	}
	if poisoned.Code != CodeUnresolved {
		t.Fatalf("code = %q, want %q", poisoned.Code, CodeUnresolved)
	}
	retained := &gcsv1.CommandTransaction{}
	if err := protojson.Unmarshal(poisoned.Transaction, retained); err != nil {
		t.Fatal(err)
	}
	if retained.GetId() != first.GetId() || retained.GetRegistryEpoch() != testEpoch {
		t.Fatalf("retained ref = (%d,%q)", retained.GetId(), retained.GetRegistryEpoch())
	}

	if _, err := f.registry.ResolveArm(
		context.Background(), 1, testEpoch, first.GetId(), gcsv1.ArmState_ARM_STATE_DISARMED); err != nil {
		t.Fatal(err)
	}

	// Quarantined: a different code, and a wait rather than an attestation.
	rec = post(t, ArmHandler(f.registry), "/api/commands/arm", `{"system_id":1,"arm":true}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("quarantined status = %d", rec.Code)
	}
	var quarantined struct {
		Code         string `json:"code"`
		RetryAfterMs int64  `json:"retry_after_ms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &quarantined); err != nil {
		t.Fatal(err)
	}
	if quarantined.Code != CodeQuarantined {
		t.Fatalf("code = %q, want %q", quarantined.Code, CodeQuarantined)
	}
	if quarantined.Code == poisoned.Code {
		t.Fatal("the two conflicts are indistinguishable")
	}
	if quarantined.RetryAfterMs != (5 * time.Second).Milliseconds() {
		t.Fatalf("retry_after_ms = %d, want 5000", quarantined.RetryAfterMs)
	}
}

func TestResolveHandlerHardening(t *testing.T) {
	f := resolveFixtureFor(t, &capture{}, silentWrite)
	body := `{"system_id":1,"registry_epoch":"e","transaction_id":7,"observed_state":"ARMED"}`
	tests := []struct{ name, contentType, origin, fetchSite string }{
		{"wrong content type", "text/plain", "", ""},
		{"cross origin", "application/json", "http://evil.test", ""},
		{"cross site", "application/json", "", "cross-site"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost,
				"http://localhost:8080/api/commands/arm/resolve", strings.NewReader(body))
			req.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			rec := httptest.NewRecorder()
			ResolveHandler(f.registry)(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", rec.Code)
			}
		})
	}
}
