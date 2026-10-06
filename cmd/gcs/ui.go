package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// envUIDir names a directory of built frontend assets (the Vite `dist`). When
// set, the backend serves the observer UI itself, so a local launch needs one
// process, one port and no dev server or proxy.
const envUIDir = "GCS_UI_DIR"

// uiHandler serves static assets from dir with single-page-app fallback:
// an unknown path returns index.html so a browser reload on a client route
// works. Anything under /api/ that reached here is a genuine 404, never HTML.
func uiHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		clean := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if info, err := os.Stat(clean); err != nil || info.IsDir() && r.URL.Path != "/" {
			// Unknown path or a bare directory: the app shell decides.
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, index)
			return
		}

		files.ServeHTTP(w, r)
	})
}

// validateUIDir fails startup early, with a message naming the fix, when the
// configured directory cannot serve the app.
func validateUIDir(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return &uiDirError{dir: dir, err: err}
	}

	return nil
}

type uiDirError struct {
	dir string
	err error
}

func (e *uiDirError) Error() string {
	return envUIDir + "=" + e.dir + " has no index.html (build the frontend: cd frontend && pnpm build): " + e.err.Error()
}
