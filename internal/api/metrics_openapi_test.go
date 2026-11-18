package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func newMetricsServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := config.Defaults()
	cfg.Storage.DataDir = dir
	return &Server{
		st:        st,
		cfg:       cfg,
		version:   "1.4.0",
		commit:    "abc123",
		buildDate: "2026-11-17",
		startedAt: time.Now().Add(-time.Minute),
	}
}

func TestMetricsExposition(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type inesperado: %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# TYPE netdiag_up gauge",
		"netdiag_up 1",
		"# TYPE netdiag_hosts_total gauge",
		"netdiag_hosts_total 0",
		"# TYPE netdiag_uptime_seconds gauge",
		`netdiag_info{commit="abc123",version="1.4.0"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %q en la salida Prometheus", want)
		}
	}
}

func TestMetricsToken(t *testing.T) {
	s := newMetricsServer(t)
	s.cfg.Security.MetricsToken = "s3cret-token"

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"sin token", "", http.StatusUnauthorized},
		{"token incorrecto", "Bearer nope", http.StatusUnauthorized},
		{"esquema incorrecto", "Basic s3cret-token", http.StatusUnauthorized},
		{"token correcto", "Bearer s3cret-token", http.StatusOK},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		rec := httptest.NewRecorder()
		s.handleMetrics(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: código %d, se esperaba %d", c.name, rec.Code, c.want)
		}
	}
}

func TestOpenAPISpecIsValidJSON(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleOpenAPI(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	var spec map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if spec["openapi"] != "3.1.0" {
		t.Fatalf("versión OpenAPI inesperada: %v", spec["openapi"])
	}
	info, _ := spec["info"].(map[string]any)
	if info == nil || info["title"] != "NetDiag API" {
		t.Fatalf("info inválida: %v", spec["info"])
	}
	if info["version"] != "1.4.0" {
		t.Fatalf("versión de la API inesperada: %v", info["version"])
	}
	paths, _ := spec["paths"].(map[string]any)
	if len(paths) < 20 {
		t.Fatalf("solo %d rutas documentadas", len(paths))
	}
	seen := map[string]bool{}
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		if len(ops) == 0 {
			t.Fatalf("la ruta %s no tiene operaciones", path)
		}
		for method, raw := range ops {
			op, _ := raw.(map[string]any)
			id, _ := op["operationId"].(string)
			if id == "" {
				t.Errorf("%s %s sin operationId", method, path)
				continue
			}
			if seen[id] {
				t.Errorf("operationId duplicado: %s", id)
			}
			seen[id] = true
			if op["responses"] == nil {
				t.Errorf("%s %s sin responses", method, path)
			}
		}
	}
	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for _, name := range []string{"Error", "LoginRequest", "CreateUserRequest", "ScanRequest"} {
		if schemas[name] == nil {
			t.Errorf("falta el esquema %s", name)
		}
	}
	if schemas["Error"] == nil {
		t.Fatal("sin esquema Error")
	}
}
