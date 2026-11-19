package diagnostics

import (
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func tlsHost(notAfter time.Time) model.Host {
	return model.Host{ID: 7, IP: "192.168.1.10", Ports: []model.Port{{
		Number: 443, Protocol: "tcp", Service: "https",
		TLS: &model.TLSCert{Subject: "CN=router.local", Issuer: "CN=CA local", NotAfter: notAfter},
	}}}
}

func TestDetectTLSExpiryThresholds(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		days     int
		want     bool
		severity string
	}{
		{"31 días", 31, false, ""},
		{"30 días", 30, true, model.SeverityWarning},
		{"14 días", 14, true, model.SeverityWarning},
		{"7 días", 7, true, model.SeverityWarning},
		{"1 día", 1, true, model.SeverityCritical},
		{"caducado", -3, true, model.SeverityCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectTLSExpiry([]model.Host{tlsHost(now.AddDate(0, 0, tc.days))}, now, config.Diagnostics{})
			if !tc.want {
				if len(got) != 0 {
					t.Fatalf("no debería avisar: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Severity != tc.severity || got[0].Type != "tls_expiring" {
				t.Fatalf("resultado inesperado: %+v", got)
			}
			if got[0].Cause == "" || got[0].Impact == "" || len(got[0].NextSteps) < 3 {
				t.Fatalf("hallazgo sin explicar: %+v", got[0])
			}
			if !strings.Contains(got[0].Detail, "certificado") {
				t.Fatalf("detalle sin fecha legible: %s", got[0].Detail)
			}
		})
	}
}

func TestDetectTLSExpiryIgnoresMissingDates(t *testing.T) {
	now := time.Now()
	hosts := []model.Host{
		{ID: 1, Ports: []model.Port{{Number: 80}}},
		{ID: 2, Ports: []model.Port{{Number: 443, TLS: &model.TLSCert{Subject: "sin fecha"}}}},
	}
	if got := DetectTLSExpiry(hosts, now, config.Diagnostics{}); len(got) != 0 {
		t.Fatalf("no debería avisar sin not_after: %+v", got)
	}
}

func TestDetectTLSExpiryDeterministic(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	h := tlsHost(now.AddDate(0, 0, 9))
	a := DetectTLSExpiry([]model.Host{h}, now, config.Diagnostics{})
	b := DetectTLSExpiry([]model.Host{h}, now, config.Diagnostics{})
	if a[0].ID != b[0].ID || a[0].Detail != b[0].Detail {
		t.Fatal("el aviso TLS no es determinista")
	}
}
