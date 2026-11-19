package config

import (
	"strings"
	"testing"
)

func TestSNMPDeepDiagnosticsDefaults(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Diagnostics.SNMPDeep {
		t.Fatal("el sondeo SNMP profundo debe estar activo por defecto")
	}
	if cfg.Diagnostics.SNMPHistoryDays != 30 {
		t.Fatalf("retención inesperada: %d", cfg.Diagnostics.SNMPHistoryDays)
	}
	cfg.Diagnostics.SNMPHistoryDays = 0
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.Diagnostics.SNMPHistoryDays != 30 {
		t.Fatalf("normalize no restauró la retención: %d", cfg.Diagnostics.SNMPHistoryDays)
	}
}

func TestSNMPDeepDiagnosticsValidation(t *testing.T) {
	cfg := Defaults()
	cfg.Diagnostics.SNMPHistoryDays = 0
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "snmp_history_days") {
		t.Fatalf("esperaba error de retención: %v", err)
	}
}
