package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testKey(t *testing.T) (seed string, public string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()),
		base64.StdEncoding.EncodeToString(pub)
}

func sampleLicense() *License {
	return &License{
		Schema:          Schema,
		LicenseID:       "ND-TEST-001",
		Customer:        "Cliente Prueba",
		Edition:         "pro",
		MaxManagedHosts: 25,
		MaxSites:        3,
		RetentionDays:   30,
		ExpiresAt:       time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339),
		Capabilities:    []string{"reports", "alerts"},
		SupportLevel:    "business",
	}
}

func writeLicenseFile(t *testing.T, path string, l *License) {
	t.Helper()
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	seed, public := testKey(t)
	l := sampleLicense()
	if err := Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	if l.Signature == "" {
		t.Fatal("la firma quedó vacía")
	}
	if err := Verify(l, public); err != nil {
		t.Fatalf("la licencia firmada no verificó: %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	seed, public := testKey(t)
	l := sampleLicense()
	if err := Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	original := l.Signature
	l.MaxManagedHosts = 5000
	if err := Verify(l, public); err == nil {
		t.Fatal("se aceptó una licencia manipulada")
	}
	l.MaxManagedHosts = 25
	l.Signature = original
	if err := Verify(l, public); err != nil {
		t.Fatalf("la licencia original dejó de verificar: %v", err)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	seed, _ := testKey(t)
	_, otherPublic := testKey(t)
	l := sampleLicense()
	if err := Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	if err := Verify(l, otherPublic); err == nil {
		t.Fatal("se aceptó una licencia firmada por otra clave")
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	base := sampleLicense()
	cases := []struct {
		name   string
		mutate func(*License)
	}{
		{"esquema", func(l *License) { l.Schema = "otro" }},
		{"id", func(l *License) { l.LicenseID = " " }},
		{"edición", func(l *License) { l.Edition = "gold" }},
		{"límite hosts", func(l *License) { l.MaxManagedHosts = 0 }},
		{"retención", func(l *License) { l.RetentionDays = -1 }},
		{"fecha", func(l *License) { l.ExpiresAt = "ayer" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := *base
			tc.mutate(&l)
			if err := Validate(&l); err == nil {
				t.Fatal("se aceptó una licencia inválida")
			}
		})
	}
}

func TestSignValidatesBeforeSigning(t *testing.T) {
	seed, _ := testKey(t)
	l := sampleLicense()
	l.Edition = "desconocida"
	if err := Sign(l, seed); err == nil {
		t.Fatal("se firmó una licencia estructuralmente inválida")
	}
}

func TestManagerLifecycle(t *testing.T) {
	seed, public := testKey(t)
	dir := t.TempDir()
	m, err := NewWithKey(dir, public)
	if err != nil {
		t.Fatal(err)
	}
	if m.Current() != nil {
		t.Fatal("sin archivo debería haber licencia Community")
	}
	if st := m.Status(time.Now(), 3, 1); st.Status != "community" || st.Edition != "community" {
		t.Fatalf("estado Community inesperado: %+v", st)
	}
	if st := m.Status(time.Now(), 3, 1); st.Limits.RetentionDays != 7 || st.Limits.MaxManagedHosts != 0 {
		t.Fatalf("límites Community inesperados: %+v", st.Limits)
	}

	src := filepath.Join(dir, "emitida.json")
	l := sampleLicense()
	if err := Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	writeLicenseFile(t, src, l)

	activated, err := m.Activate(src)
	if err != nil {
		t.Fatal(err)
	}
	if activated.LicenseID != l.LicenseID {
		t.Fatalf("ID activado inesperado: %q", activated.LicenseID)
	}
	if st := m.Status(time.Now(), 3, 1); st.Status != "active" || st.Edition != "pro" {
		t.Fatalf("estado activo inesperado: %+v", st)
	}
	if info, err := os.Stat(m.Path()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("license.json debería ser 0600: %v %v", info, err)
	}

	// Una firma manipulada degrada a Community con aviso, sin borrar el archivo.
	raw, err := os.ReadFile(m.Path())
	if err != nil {
		t.Fatal(err)
	}
	var tampered License
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.Customer = "Atacante"
	writeLicenseFile(t, m.Path(), &tampered)
	if err := m.Load(); err == nil {
		t.Fatal("se aceptó license.json manipulado")
	}
	st := m.Status(time.Now(), 3, 1)
	if st.Status != "invalid" || len(st.Warnings) == 0 {
		t.Fatalf("se esperaba estado invalid con aviso: %+v", st)
	}
	if st.Limits.RetentionDays != 7 {
		t.Fatal("un licencia inválida no debería conservar límites Pro")
	}
	if _, err := os.Stat(m.Path()); err != nil {
		t.Fatal("license.json no debería borrarse automáticamente")
	}

	if err := m.Clear(); err != nil {
		t.Fatal(err)
	}
	if m.Current() != nil || m.Problem() != "" {
		t.Fatal("Clear no devolvió el estado Community limpio")
	}
	if _, err := os.Stat(m.Path()); !os.IsNotExist(err) {
		t.Fatalf("Clear debería eliminar license.json: %v", err)
	}
}

func TestActivateRejectsForgedFile(t *testing.T) {
	_, public := testKey(t)
	dir := t.TempDir()
	m, err := NewWithKey(dir, public)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "falsa.json")
	writeLicenseFile(t, src, sampleLicense()) // sin firma
	if _, err := m.Activate(src); err == nil {
		t.Fatal("se activó una licencia sin firma")
	}
	if _, err := os.Stat(m.Path()); !os.IsNotExist(err) {
		t.Fatal("una licencia rechazada no debería instalarse")
	}
}

func TestStatusWarningsDoNotBlock(t *testing.T) {
	seed, public := testKey(t)
	dir := t.TempDir()
	m, err := NewWithKey(dir, public)
	if err != nil {
		t.Fatal(err)
	}
	l := sampleLicense()
	l.ExpiresAt = time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339)
	if err := Sign(l, seed); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "caducada.json")
	writeLicenseFile(t, src, l)
	if _, err := m.Activate(src); err != nil {
		t.Fatal(err)
	}
	st := m.Status(time.Now(), 40, 6)
	if st.Status != "expired" {
		t.Fatalf("se esperaba expired, se obtuvo %q", st.Status)
	}
	joined := strings.Join(st.Warnings, " ")
	if !strings.Contains(joined, "caduc") || !strings.Contains(joined, "40") || !strings.Contains(joined, "6") {
		t.Fatalf("faltan avisos de caducidad o banda: %v", st.Warnings)
	}
	if len(st.Capabilities) == 0 {
		t.Fatal("una licencia caducada debería seguir exponiendo capacidades informativas")
	}
}
