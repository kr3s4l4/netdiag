package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// allFindingTypes mirrors every Type currently emitted by the detectors plus
// the TLS type introduced by F60.
var allFindingTypes = []string{
	"arp_gateway_mac_changed", "arp_ip_mac_changed", "arp_mac_multi_ip", "arp_ip_multi_mac",
	"dhcp_multiple_servers", "dhcp_server_mac_changed", "dhcp_gateway_spoof", "dhcp_unauthorized_server",
	"snmp_fdb_mac_moved", "snmp_fdb_port_conflict", "snmp_fdb_vlan_conflict",
	"snmp_lldp_fdb_disagree", "snmp_vlan_unknown", "stp_loop", "broadcast_storm",
	"duplex_mismatch", "mtu_problem", "vlan_leak", "sla_breach", "rogue_ap",
	"passive_gratuitous_arp", "passive_icmp_redirect", "passive_dhcp_server",
	"passive_inbound_scan", "passive_dns_tunnel", "tls_expiring",
	"unknown_future_type",
}

func TestExplainEveryType(t *testing.T) {
	for _, kind := range allFindingTypes {
		t.Run(kind, func(t *testing.T) {
			in := model.DiagnosticAnomaly{
				ID:         kind + ":1",
				Type:       kind,
				Title:      "Hallazgo de prueba",
				Detail:     "Detalle",
				Confidence: 85,
				Evidence:   []string{"evidencia uno", "evidencia dos"},
				DetectedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
			}
			out := Explain(in)
			if out.Cause == "" || out.Impact == "" || out.Limitations == "" {
				t.Fatalf("faltan campos explicativos: %+v", out)
			}
			if len(out.NextSteps) < 3 || len(out.NextSteps) > 5 {
				t.Fatalf("NextSteps debe tener 3-5 pasos, tiene %d", len(out.NextSteps))
			}
			if len(out.Hypotheses) < 2 {
				t.Fatalf("faltan hipótesis: %d", len(out.Hypotheses))
			}
			for i, h := range out.Hypotheses {
				if h.Rank != i+1 || h.Label == "" || h.Why == "" || h.Check == "" {
					t.Fatalf("hipótesis mal formada en %d: %+v", i, h)
				}
				if h.Confidence > out.Confidence {
					t.Fatalf("hipótesis %d supera la confianza de la evidencia", i)
				}
			}
			if out.Recommendation == "" {
				t.Fatal("Recommendation debe mantenerse por compatibilidad")
			}
		})
	}
}

func TestExplainDeterministic(t *testing.T) {
	in := model.DiagnosticAnomaly{Type: "arp_gateway_mac_changed", Confidence: 95,
		Evidence: []string{"MAC anterior aa", "MAC actual bb"}}
	a, _ := json.Marshal(Explain(in))
	b, _ := json.Marshal(Explain(in))
	if string(a) != string(b) {
		t.Fatal("la explicación no es determinista")
	}
}

func TestExplainDoesNotMutateInput(t *testing.T) {
	in := model.DiagnosticAnomaly{Type: "stp_loop", Confidence: 80}
	_ = Explain(in)
	if in.Cause != "" || len(in.NextSteps) != 0 || len(in.Hypotheses) != 0 {
		t.Fatal("Explain mutó la entrada")
	}
}

func TestLegacyJSONHasNoNewKeys(t *testing.T) {
	legacy := model.DiagnosticAnomaly{ID: "x", Type: "stp_loop", Severity: "warning",
		Title: "t", Detail: "d", Confidence: 80, DetectedAt: time.Now()}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"cause"`, `"impact"`, `"next_steps"`, `"limitations"`, `"hypotheses"`} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("el JSON antiguo contiene %s: %s", key, raw)
		}
	}
}

func TestLowConfidenceIsHedged(t *testing.T) {
	out := Explain(model.DiagnosticAnomaly{Type: "arp_ip_mac_changed", Confidence: 60})
	if !strings.HasPrefix(strings.ToLower(out.Cause), "posible") {
		t.Fatalf("cause sin atenuar: %s", out.Cause)
	}
	high := Explain(model.DiagnosticAnomaly{Type: "arp_ip_mac_changed", Confidence: 95})
	if strings.HasPrefix(strings.ToLower(high.Cause), "posible") {
		t.Fatalf("cause atenuada sin necesidad: %s", high.Cause)
	}
}

func TestRankCorrelations(t *testing.T) {
	report := &model.DiagnosticsReport{Anomalies: []model.DiagnosticAnomaly{
		{ID: "1", Type: "arp_gateway_mac_changed", Title: "Cambió la MAC del gateway", Confidence: 95,
			Evidence: []string{"MAC aa → bb"}},
		{ID: "2", Type: "dhcp_unauthorized_server", Title: "DHCP no autorizado", Confidence: 90,
			Evidence: []string{"server 10.0.0.50"}},
	}}
	for i := range report.Anomalies {
		enrich(&report.Anomalies[i])
	}
	rankCorrelations(report)
	for _, a := range report.Anomalies {
		if len(a.Hypotheses) == 0 || a.Hypotheses[0].Label != "Suplantación de gateway" {
			t.Fatalf("correlación no aplicada en %s: %+v", a.ID, a.Hypotheses)
		}
		if a.Hypotheses[0].Rank != 1 || len(a.Hypotheses[0].Evidence) < 2 {
			t.Fatalf("hipótesis compuesta incompleta: %+v", a.Hypotheses[0])
		}
	}
}

func TestAnalyzerEnrichesFindings(t *testing.T) {
	from := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	a := (Analyzer{}).Analyze(Input{
		SiteID: 1, From: from, To: to,
		Config: config.Diagnostics{EnterpriseMode: true},
		ARP: []model.ARPObservation{
			{ScanID: 1, SiteID: 1, IP: "192.168.1.1", MAC: "aa:aa:aa:aa:aa:aa", IsGateway: true, ObservedAt: from},
			{ScanID: 2, SiteID: 1, IP: "192.168.1.1", MAC: "cc:cc:cc:cc:cc:cc", IsGateway: true, ObservedAt: to},
		},
	})
	if len(a.Anomalies) == 0 {
		t.Fatal("el analizador no devolvió hallazgos")
	}
	for _, finding := range a.Anomalies {
		if finding.Cause == "" || finding.Impact == "" || len(finding.NextSteps) == 0 {
			t.Fatalf("hallazgo sin explicar: %+v", finding)
		}
	}
}
