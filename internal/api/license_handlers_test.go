package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/license"
)

func licenseTestManager(t *testing.T, dir string) (*license.Manager, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public := base64.StdEncoding.EncodeToString(pub)
	m, err := license.NewWithKey(dir, public)
	if err != nil {
		t.Fatal(err)
	}
	return m, base64.StdEncoding.EncodeToString(priv.Seed())
}

func signedLicenseJSON(t *testing.T, dir, seed string) []byte {
	t.Helper()
	l := &license.License{
		Schema:          license.Schema,
		LicenseID:       "ND-API-001",
		Customer:        "Cliente Secreto",
		Edition:         "enterprise",
		MaxManagedHosts: 100,
		MaxSites:        10,
		RetentionDays:   365,
		ExpiresAt:       time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339),
		Capabilities:    []string{"sso", "audit"},
		SupportLevel:    "enterprise",
	}
	if err := license.Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "license.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeLicenseResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, se obtuvo %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("respuesta no JSON: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestHandleSystemLicenseCommunity(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleSystemLicense(rec, httptest.NewRequest(http.MethodGet, "/api/v1/system/license", nil))
	out := decodeLicenseResponse(t, rec)
	if out["status"] != "community" || out["edition"] != "community" {
		t.Fatalf("estado Community inesperado: %v", out)
	}
	if out["expires_at"] != "" || out["support_level"] != "" {
		t.Fatal("Community debería exponer expires_at y support_level vacíos, no omitirlos")
	}
	limits, ok := out["limits"].(map[string]any)
	if !ok || limits["retention_days"] != float64(7) || limits["max_managed_hosts"] != float64(0) {
		t.Fatalf("límites Community inesperados: %v", out["limits"])
	}
	if _, leaked := out["customer"]; leaked {
		t.Fatal("la respuesta filtró el cliente")
	}
	if _, leaked := out["signature"]; leaked {
		t.Fatal("la respuesta filtró la firma")
	}
}

func TestHandleSystemLicenseActiveHidesPrivateFields(t *testing.T) {
	s := newMetricsServer(t)
	m, seed := licenseTestManager(t, s.cfg.Storage.DataDir)
	signedLicenseJSON(t, s.cfg.Storage.DataDir, seed)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	s.licenses = m

	rec := httptest.NewRecorder()
	s.handleSystemLicense(rec, httptest.NewRequest(http.MethodGet, "/api/v1/system/license", nil))
	out := decodeLicenseResponse(t, rec)
	if out["status"] != "active" || out["edition"] != "enterprise" {
		t.Fatalf("estado activo inesperado: %v", out)
	}
	for _, secret := range []string{"customer", "signature"} {
		if _, leaked := out[secret]; leaked {
			t.Fatalf("la respuesta filtró %s", secret)
		}
	}
	body := rec.Body.String()
	if strings.Contains(body, "Cliente Secreto") {
		t.Fatal("el nombre del cliente aparece en la respuesta")
	}
}

func TestHandleSystemLicenseInvalidIsNotAnError(t *testing.T) {
	s := newMetricsServer(t)
	m, _ := licenseTestManager(t, s.cfg.Storage.DataDir)
	if err := os.WriteFile(filepath.Join(s.cfg.Storage.DataDir, "license.json"), []byte("{\"schema\":\"roto\"}"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = m.Load() // falla a propósito
	s.licenses = m

	rec := httptest.NewRecorder()
	s.handleSystemLicense(rec, httptest.NewRequest(http.MethodGet, "/api/v1/system/license", nil))
	out := decodeLicenseResponse(t, rec)
	if out["status"] != "invalid" {
		t.Fatalf("se esperaba invalid, se obtuvo %v", out["status"])
	}
	warnings, ok := out["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("se esperaba al menos un aviso: %v", out["warnings"])
	}
}
