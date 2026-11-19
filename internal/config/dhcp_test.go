package config

import (
	"strings"
	"testing"
	"time"
)

func TestDHCPDiagnosticsDefaults(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Diagnostics.DHCPProbe {
		t.Fatal("el sondeo DHCP debe estar activo por defecto en segmentos locales")
	}
	if cfg.Diagnostics.DHCPProbeTimeout.Duration() != 700*time.Millisecond {
		t.Fatalf("timeout inesperado: %s", cfg.Diagnostics.DHCPProbeTimeout.Duration())
	}
	if cfg.Diagnostics.DHCPHistoryDays != 30 {
		t.Fatalf("retención inesperada: %d", cfg.Diagnostics.DHCPHistoryDays)
	}
}

func TestDHCPDiagnosticsValidation(t *testing.T) {
	cfg := Defaults()
	cfg.Diagnostics.DHCPProbeTimeout = Duration(10 * time.Millisecond)
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "dhcp_probe_timeout") {
		t.Fatalf("esperaba error de timeout: %v", err)
	}
	cfg = Defaults()
	cfg.Diagnostics.AuthorizedDHCPServers = []string{"no-es-ip"}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "authorized_dhcp_servers") {
		t.Fatalf("esperaba error de servidor autorizado: %v", err)
	}
}
