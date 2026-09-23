package connection

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"yalb.gcs/internal/codec"
)

func TestHTTPGuardsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, origin, content, site string
		want                              int
	}{
		{"missing", `{"device_id":"gone","settings":{"baud_rate":57600}}`, "", "application/json", "", 404},
		{"busy", `{"device_id":"serial:test","settings":{"baud_rate":57600}}`, "", "application/json", "", 409},
		{"baud", `{"device_id":"serial:test","settings":{"baud_rate":0}}`, "", "application/json", "", 400},
		{"origin", `{}`, "https://evil.invalid", "application/json", "", 400},
		{"site", `{}`, "", "application/json", "cross-site", 400},
		{"type", `{}`, "", "text/plain", "", 400},
		{"unknown", `{"path":"/etc/passwd"}`, "", "application/json", "", 400},
		{"trailing", `{} {}`, "", "application/json", "", 400},
		{"large", `{"device_id":"` + strings.Repeat("x", 5000) + `"}`, "", "application/json", "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(Config{Inventory: inventoryFunc(testInventory), Open: func(Device, Settings, string) (codec.FrameSource, error) { return nil, syscall.EBUSY }})
			defer m.Close()
			mux := http.NewServeMux()
			Mount(mux, m)
			req := httptest.NewRequest("POST", "/api/connections/connect", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.content)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestEmptyAndReleasedHTTP(t *testing.T) {
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{}, nil })})
	defer m.Close()
	mux := http.NewServeMux()
	Mount(mux, m)
	for _, path := range []string{"/api/connections", "/api/connections/devices"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/connections/disconnect", strings.NewReader(`{"id":"already-gone"}`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func jsonRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestProfileCRUDOverHTTP(t *testing.T) {
	device := Device{ID: "serial:test", Kind: "serial", Path: "test", SerialNumber: "SN", VID: "1", PID: "1"}
	var opens int
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return []Device{device}, nil }),
		Profiles: &stubStore{}, Open: newFakeOpener(&opens)})
	defer m.Close()
	mux := http.NewServeMux()
	Mount(mux, m)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/connections/profiles", nil))
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, jsonRequest("POST", "/api/connections/profiles", `{"name":"Bench","device_id":"serial:test","settings":{"baud_rate":57600}}`))
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created Profile
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("create decode: %v %s", err, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/connections/profiles", nil))
	var listed []Profile
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed) != 1 {
		t.Fatalf("list: %v %s", err, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, jsonRequest("POST", "/api/connections/connect", `{"profile_id":"`+created.ID+`"}`))
	if w.Code != 200 {
		t.Fatalf("connect via profile: %d %s", w.Code, w.Body.String())
	}
	var status Status
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.State != Awaiting {
		t.Fatalf("connect via profile decode: %v %+v", err, status)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, jsonRequest("POST", "/api/connections/disconnect", `{"id":"`+created.ID+`"}`))
	if w.Code != 200 {
		t.Fatalf("disconnect: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/connections/profiles/"+created.ID, nil))
	if w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/connections/profiles/"+created.ID, nil))
	if w.Code != 404 {
		t.Fatalf("delete again should 404: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/connections/profiles/anything", nil)
	r.Header.Set("Origin", "https://evil.invalid")
	mux.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("cross-origin delete should be rejected: %d %s", w.Code, w.Body.String())
	}
}

func TestProfileLoadErrorSurfacesOverHTTP(t *testing.T) {
	m := New(Config{Inventory: inventoryFunc(testInventory), Profiles: &stubStore{loadErr: errors.New("corrupt")}})
	defer m.Close()
	mux := http.NewServeMux()
	Mount(mux, m)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/connections/profiles", nil))
	if w.Code != 500 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestConnectRequiresDeviceOrProfileID(t *testing.T) {
	m := New(Config{Inventory: inventoryFunc(testInventory)})
	defer m.Close()
	mux := http.NewServeMux()
	Mount(mux, m)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, jsonRequest("POST", "/api/connections/connect", `{"settings":{"baud_rate":57600}}`))
	if w.Code != 400 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
