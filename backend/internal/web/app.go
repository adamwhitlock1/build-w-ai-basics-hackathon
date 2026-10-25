package web

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// App serves the built web app in dir (frontend/dist). A file that exists
// is served as it is; any other path gets index.html, so the app's own
// routes (/invite/…, /chat/3) load when opened directly. Files under
// /assets/ have content hashes in their names, so browsers keep them for a
// year; everything else, the page above all, is checked every time.
//
// Register it as the lowest-priority pattern (GET /), after the API, and
// keep /api/ paths from reaching it.
func App(dir string) http.Handler {
	fsys := os.DirFS(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if info, err := fs.Stat(fsys, name); name != "" && err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFileFS(w, r, fsys, name)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		page, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.Error(w, "The web app isn't built.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
}
