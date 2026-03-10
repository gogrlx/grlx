package serve

//go:generate bash -c "cd ../../../../grlx-web-ui && bun install --frozen-lockfile && bun run build:static && rm -rf ../grlx/internal/serve/dist && cp -r dist ../grlx/internal/serve/dist"

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var uiAssets embed.FS

// UIHandler returns an http.Handler that serves the embedded SPA.
// Known files are served directly; all other paths fall back to
// index.html for client-side routing.
func UIHandler() http.Handler {
	distFS, err := fs.Sub(uiAssets, "dist")
	if err != nil {
		// Should never happen — dist/ is embedded at compile time.
		panic("serve: failed to open embedded dist/ filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash for fs.Open
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the exact file
		if f, err := distFS.Open(path); err == nil {
			f.Close()

			// Set long cache for hashed assets (assets/ directory)
			if strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}

			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
