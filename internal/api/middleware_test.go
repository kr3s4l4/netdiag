package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestSecurityHeadersHostAllowlist(t *testing.T) {
	s := &Server{cfg: &config.Config{
		Server:   config.Server{EnableHSTS: true, PublicURL: "https://netdiag.example.com"},
		Security: config.Security{AllowedHosts: []string{"netdiag.example.com"}},
	}}
	h := s.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		host string
		want int
	}{
		{"netdiag.example.com", http.StatusNoContent},
		{"NetDiag.Example.com:443", http.StatusNoContent},
		{"netdiag.example.com.", http.StatusNoContent},
		{"evil.example.com", http.StatusMisdirectedRequest},
		{"netdiag.example.com.evil.test", http.StatusMisdirectedRequest},
		{"", http.StatusMisdirectedRequest},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("host %q: código %d, se esperaba %d", c.host, rec.Code, c.want)
		}
	}

	// Sin lista, cualquier host pasa y se añaden las cabeceras de seguridad.
	s.cfg.Security.AllowedHosts = nil
	req := httptest.NewRequest(http.MethodGet, "http://otro.host/", nil)
	req.Host = "otro.host"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("sin lista de hosts se esperaba 204, hay %d", rec.Code)
	}
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("falta HSTS")
	}
}

func TestCanWriteRejectsViewer(t *testing.T) {
	s := &Server{}
	cases := []struct {
		role string
		want int
	}{
		{"admin", http.StatusNoContent},
		{"owner", http.StatusNoContent},
		{"viewer", http.StatusForbidden},
		{"", http.StatusForbidden},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", nil)
		req = req.WithContext(context.WithValue(req.Context(), ctxUser, &model.User{Username: "u", Role: c.role}))
		rec := httptest.NewRecorder()
		if !s.canWrite(rec, req) {
			if rec.Code != c.want {
				t.Errorf("rol %q: código %d, se esperaba %d", c.role, rec.Code, c.want)
			}
			continue
		}
		if c.want != http.StatusNoContent {
			t.Errorf("rol %q: se permitió escribir y no debería", c.role)
		}
	}

	// Sin usuario en contexto la respuesta es 401.
	rec := httptest.NewRecorder()
	if s.canWrite(rec, httptest.NewRequest(http.MethodPost, "/api/v1/scans", nil)) {
		t.Fatal("sin usuario se permitió escribir")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin usuario: código %d, se esperaba 401", rec.Code)
	}
}
