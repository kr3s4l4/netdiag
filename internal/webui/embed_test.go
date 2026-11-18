package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRoutesAPIAndMetrics(t *testing.T) {
	var apiPath string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiPath = r.URL.Path
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("api"))
	})
	h := Handler(api)

	cases := []struct {
		path string
		want string
	}{
		{"/metrics", "/metrics"},
		{"/api/v1/hosts", "/api/v1/hosts"},
	}
	for _, c := range cases {
		apiPath = ""
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if apiPath != c.want {
			t.Errorf("%s no se delegó a la API (ruta %q)", c.path, apiPath)
		}
		if rec.Body.String() != "api" {
			t.Errorf("%s devolvió %q", c.path, rec.Body.String())
		}
	}
}

func TestHandlerServesSPAAndHealthz(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("la ruta %s no debería llegar a la API", r.URL.Path)
	})
	h := Handler(api)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type inesperado: %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "NetDiag") {
		t.Fatal("el índice no contiene la marca")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("falta la CSP en el índice")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("healthz inesperado: %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandlerDoesNotCacheUnhashedAssets(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("la ruta %s no debería llegar a la API", r.URL.Path)
	})
	h := Handler(api)

	for _, file := range []string{"/app.js", "/styles.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, file, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s devolvió %d", file, rec.Code)
			continue
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s Cache-Control = %q, quería %q", file, cc, "no-cache")
		}
	}
}
