// Package webui serves the embedded single-page application.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed dist
var embedded embed.FS

// Handler serves /api/* through the API handler and everything else from the
// embedded SPA, falling back to index.html so client-side routes work.
func Handler(apiHandler http.Handler) http.Handler {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic("webui: no se pudo montar dist: " + err.Error())
	}
	files := http.FileServer(http.FS(dist))
	index, _ := fs.ReadFile(dist, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; "+
				"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")

		clean := path.Clean(r.URL.Path)
		if clean == "/" || clean == "" {
			serveIndex(w, r, index)
			return
		}
		ext := path.Ext(clean)
		if ext == ".js" || ext == ".css" || ext == ".svg" || ext == ".png" ||
			ext == ".ico" || ext == ".woff2" || ext == ".txt" || ext == ".webmanifest" {
			if f, err := dist.Open(strings.TrimPrefix(clean, "/")); err == nil {
				_ = f.Close()
				if ext == ".js" || ext == ".css" {
					h.Set("Cache-Control", "public, max-age=3600")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		// Unknown path: let the SPA router handle it.
		serveIndex(w, r, index)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	_, _ = w.Write(index)
}
