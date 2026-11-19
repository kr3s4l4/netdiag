package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPassiveCaptureDefaultsAreOptIn(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.Diagnostics.PassiveCapture {
		t.Fatal("la captura pasiva debe ser opt-in")
	}
	if cfg.Diagnostics.PassiveSeconds != 10 || cfg.Diagnostics.PassiveMaxPackets != 2000 {
		t.Fatalf("límites inesperados: %+v", cfg.Diagnostics)
	}
	if cfg.Diagnostics.PassiveHistoryDays != 7 {
		t.Fatalf("retención inesperada: %d", cfg.Diagnostics.PassiveHistoryDays)
	}
}

func TestPassiveCaptureValidationAndEnv(t *testing.T) {
	cfg := Defaults()
	cfg.Diagnostics.PassiveSeconds = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "passive_seconds") {
		t.Fatalf("esperaba error de ventana: %v", err)
	}
	cfg = Defaults()
	cfg.Diagnostics.PassiveMaxPackets = 1
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "passive_max_packets") {
		t.Fatalf("esperaba error de límite: %v", err)
	}
	t.Setenv("NETDIAG_PASSIVE_CAPTURE", "true")
	t.Setenv("NETDIAG_PASSIVE_IFACE", "eth0")
	t.Setenv("NETDIAG_PASSIVE_SECONDS", "5")
	t.Setenv("NETDIAG_PASSIVE_HISTORY_DAYS", "3")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Diagnostics.PassiveCapture || loaded.Diagnostics.PassiveInterface != "eth0" ||
		loaded.Diagnostics.PassiveSeconds != 5 || loaded.Diagnostics.PassiveHistoryDays != 3 {
		t.Fatalf("variables no aplicadas: %+v", loaded.Diagnostics)
	}
}
