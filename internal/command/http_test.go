package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluenviron/gomavlib/v3/pkg/message"
	"google.golang.org/protobuf/encoding/protojson"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
)

func TestArmHandlerReturnsProtoJSONTransaction(t *testing.T) {
	var registry *Registry
	registry, _ = registryFor(t, func(message.Message) error {
		return registry.Publish(context.Background(), ackEvent(255, 190, gcsv1.MavResult_MAV_RESULT_ACCEPTED))
	})
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/commands/arm", strings.NewReader(`{"system_id":1,"arm":true}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	ArmHandler(registry)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	tx := &gcsv1.CommandTransaction{}
	if err := protojson.Unmarshal(rec.Body.Bytes(), tx); err != nil {
		t.Fatal(err)
	}
	if tx.GetState() != gcsv1.CommandState_COMMAND_STATE_ACCEPTED {
		t.Fatalf("state = %v", tx.GetState())
	}
}

func TestArmHandlerHardening(t *testing.T) {
	registry, _ := registryFor(t, func(message.Message) error { return nil })
	tests := []struct{ name, body, contentType, origin, fetchSite string }{
		{"wrong content type", `{}`, "text/plain", "", ""},
		{"cross origin", `{"system_id":1,"arm":true}`, "application/json", "http://evil.test", ""},
		{"cross site", `{"system_id":1,"arm":true}`, "application/json", "", "cross-site"},
		{"unknown field", `{"system_id":1,"arm":true,"force":true}`, "application/json", "", ""},
		{"trailing JSON", `{"system_id":1,"arm":true}{}`, "application/json", "", ""},
		{"oversized", `{"system_id":1,"arm":true,"padding":"` + strings.Repeat("x", 4096) + `"}`, "application/json", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/commands/arm", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			rec := httptest.NewRecorder()
			ArmHandler(registry)(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}
