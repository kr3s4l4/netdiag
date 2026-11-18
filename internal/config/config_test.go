package config

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultsAreValid(t *testing.T) {
	cfg := Defaults()
	cfg.Storage.DataDir = t.TempDir()
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("las opciones por defecto no son válidas: %v", err)
	}
	if cfg.Storage.DB != filepath.Join(cfg.Storage.DataDir, "netdiag.db") {
		t.Fatalf("base de datos mal resuelta: %q", cfg.Storage.DB)
	}
	if cfg.Server.Listen != "127.0.0.1:8080" {
		t.Fatalf("listen inesperado: %q", cfg.Server.Listen)
	}
	if cfg.Scan.Timeout.Duration() != 900*time.Millisecond {
		t.Fatalf("timeout inesperado: %v", cfg.Scan.Timeout.Duration())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Defaults()
	cfg.Storage.DataDir = dir
	cfg.Scan.Profile = "full"
	cfg.Monitor.Interval = Duration(90 * time.Second)
	cfg.Scan.SNMPCommunities = []string{"public", "privada"}
	cfg.Security.Auth = false
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos esperados 0600, obtenidos %o", info.Mode().Perm())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Scan.Profile != "full" {
		t.Fatalf("perfil no conservado: %q", loaded.Scan.Profile)
	}
	if loaded.Monitor.Interval.Duration() != 90*time.Second {
		t.Fatalf("intervalo no conservado: %v", loaded.Monitor.Interval.Duration())
	}
	if loaded.Security.Auth {
		t.Fatal("security.auth no conservado")
	}
	if len(loaded.Scan.SNMPCommunities) != 2 {
		t.Fatalf("comunidades no conservadas: %v", loaded.Scan.SNMPCommunities)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "no-existe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scan.Profile != "common" {
		t.Fatalf("se esperaban valores por defecto, perfil %q", cfg.Scan.Profile)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	cfg := Defaults()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Scan.Profile = "turbo"
	cfg.Scan.Concurrency = 0
	cfg.Monitor.Interval = Duration(time.Second)
	cfg.Server.TLSCert = "/tmp/cert.pem"
	err := cfg.Normalize()
	if err == nil {
		t.Fatal("se esperaba un error de validación")
	}
	for _, want := range []string{"scan.profile", "scan.concurrency", "monitor.interval", "tls_key"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("el error no menciona %q: %v", want, err)
		}
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("NETDIAG_LISTEN", "0.0.0.0:9999")
	t.Setenv("NETDIAG_DATA_DIR", t.TempDir())
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != "0.0.0.0:9999" {
		t.Fatalf("listen no aplicado desde el entorno: %q", cfg.Server.Listen)
	}
}

func TestNormalizeAllowedHosts(t *testing.T) {
	cfg := Defaults()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Security.AllowedHosts = []string{" NetDiag.Example.com. ", "netdiag.example.com", "", "[::1]"}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	want := []string{"netdiag.example.com", "::1"}
	if len(cfg.Security.AllowedHosts) != len(want) {
		t.Fatalf("hosts inesperados: %v", cfg.Security.AllowedHosts)
	}
	for i := range want {
		if cfg.Security.AllowedHosts[i] != want[i] {
			t.Fatalf("hosts inesperados: %v", cfg.Security.AllowedHosts)
		}
	}
}

func TestUpdateConfigValidation(t *testing.T) {
	cfg := Defaults()
	cfg.Storage.DataDir = t.TempDir()
	if cfg.Update.Enabled || cfg.Update.CheckIntervalHours != 24 || cfg.Update.MaxDownloadMB != 512 {
		t.Fatalf("defaults de actualización inesperados: %+v", cfg.Update)
	}
	publicKey := base64.StdEncoding.EncodeToString(append([]byte("Ednetdiag1"), bytes.Repeat([]byte{0x11}, 32)...))

	cfg.Update.Enabled = true
	cfg.Update.ManifestURL = "https://updates.example.com/manifest.json"
	cfg.Update.PublicKey = publicKey
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("configuración válida rechazada: %v", err)
	}

	cfg.Update.ManifestURL = "http://updates.example.com/manifest.json"
	if err := cfg.Validate(); err == nil {
		t.Fatal("se aceptó HTTP fuera de localhost")
	}
	cfg.Update.ManifestURL = "https://user:pass@updates.example.com/manifest.json"
	if err := cfg.Validate(); err == nil {
		t.Fatal("se aceptaron credenciales en la URL")
	}
	cfg.Update.ManifestURL = "http://127.0.0.1:8080/manifest.json"
	cfg.Update.PublicKey = "clave-invalida"
	if err := cfg.Validate(); err == nil {
		t.Fatal("se aceptó una clave pública inválida")
	}
}

func TestDurationJSON(t *testing.T) {
	in := struct {
		D Duration `json:"d"`
	}{D: Duration(2 * time.Minute)}
	data, err := in.D.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"2m0s"` {
		t.Fatalf("serialización inesperada: %s", data)
	}
	var out Duration
	if err := out.UnmarshalJSON([]byte(`"45s"`)); err != nil {
		t.Fatal(err)
	}
	if out.Duration() != 45*time.Second {
		t.Fatalf("deserialización inesperada: %v", out.Duration())
	}
}
