package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIHandler(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>shell</title>"), 0o600))
	must(os.MkdirAll(filepath.Join(dir, "assets"), 0o750))
	must(os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o600))

	cases := []struct {
		path string
		code int
		body string
	}{
		{"/", 200, "shell"},
		{"/assets/app.js", 200, "console.log"},
		{"/some/client/route", 200, "shell"},
		{"/assets", 200, "shell"},
		{"/api/unknown", 404, "not found"},
		{"/../../etc/passwd", 400, "invalid url path"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		uiHandler(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.code || !strings.Contains(strings.ToLower(rec.Body.String()), c.body) {
			t.Errorf("%s: got %d %q", c.path, rec.Code, rec.Body.String())
		}
	}
}

func TestValidateUIDir(t *testing.T) {
	if err := validateUIDir(t.TempDir()); err == nil || !strings.Contains(err.Error(), "pnpm build") {
		t.Fatalf("empty dir must fail with the fix named, got %v", err)
	}
}
