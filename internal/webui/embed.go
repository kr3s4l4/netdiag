// Package webui serves the embedded single-page application.
package webui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
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
	index = versionAssets(index, assetVersion(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/metrics" {
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
					// Los assets no llevan hash en el nombre: una caché larga
					// deja la UI antigua después de actualizar el binario.
					h.Set("Cache-Control", "no-cache")
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

// assetVersion devuelve una huella corta de los assets embebidos. Se usa como
// query string en el índice para que un cambio en el JS o CSS cambie la URL y
// ningún navegador pueda reutilizar una copia antigua en memoria.
func assetVersion(dist fs.FS) string {
	entries, err := fs.ReadDir(dist, ".")
	if err != nil {
		return "1"
	}
	h := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "index.html" {
			continue
		}
		data, err := fs.ReadFile(dist, entry.Name())
		if err != nil {
			continue
		}
		_, _ = h.Write([]byte(entry.Name()))
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func versionAssets(index []byte, version string) []byte {
	for _, asset := range []string{"styles.css", "i18n.js", "views.js", "app.js", "views-saved.js"} {
		for _, attr := range []string{"href", "src"} {
			old := []byte(attr + `="/` + asset + `"`)
			newValue := []byte(attr + `="/` + asset + `?v=` + version + `"`)
			index = bytes.ReplaceAll(index, old, newValue)
		}
	}
	return index
}
