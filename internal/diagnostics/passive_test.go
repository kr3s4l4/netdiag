package diagnostics

import (
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func passiveCfg() config.Diagnostics {
	return config.Diagnostics{
		MinConfidence: 60, MaxAnomalies: 200,
		AuthorizedDHCPServers: []string{"10.0.0.2"},
	}
}

func hasPassiveType(findings []model.DiagnosticAnomaly, kind string) bool {
	for _, f := range findings {
		if f.Type == kind {
			return true
		}
	}
	return false
}

func TestDetectPassiveSecurityFindings(t *testing.T) {
	now := time.Now()
	obs := []model.PassiveObservation{
		{Kind: "garp", SrcIP: "10.0.0.9", Detail: "GARP 10.0.0.9", ObservedAt: now},
		{Kind: "garp", SrcIP: "10.0.0.9", Detail: "GARP 10.0.0.9", ObservedAt: now},
		{Kind: "garp", SrcIP: "10.0.0.9", Detail: "GARP 10.0.0.9", ObservedAt: now},
		{Kind: "icmp_redirect", SrcIP: "192.168.0.1", DstIP: "192.168.0.50", Detail: "ICMP redirect", ObservedAt: now},
		{Kind: "dhcp_server", SrcIP: "192.168.0.51", SrcMAC: "de:ad:be:ef:00:51", Detail: "DHCP OFFER", ObservedAt: now},
		{Kind: "dhcp_server", SrcIP: "10.0.0.2", Detail: "DHCP ACK", ObservedAt: now},
	}
	for port := 20; port < 35; port++ {
		obs = append(obs, model.PassiveObservation{
			Kind: "inbound_scan", SrcIP: "10.0.0.77", DstIP: "192.168.0.50",
			Detail: "SYN:" + itoaForTest(port), ObservedAt: now,
		})
	}
	for i := 0; i < 13; i++ {
		obs = append(obs, model.PassiveObservation{
			Kind: "dns_tunnel", SrcIP: "10.0.0.9",
			Detail: "domain" + itoaForTest(i) + ".example.com", ObservedAt: now,
		})
	}
	obs = append(obs, model.PassiveObservation{
		Kind: "dns_tunnel", SrcIP: "10.0.0.9",
		Detail: "xq7Km2P9vT4nB8cL1sZ5.example.net", ObservedAt: now,
	})

	findings := DetectPassive(obs, nil, passiveCfg())
	for _, kind := range []string{
		"passive_gratuitous_arp", "passive_icmp_redirect", "passive_dhcp_server",
		"passive_inbound_scan", "passive_dns_tunnel",
	} {
		if !hasPassiveType(findings, kind) {
			t.Errorf("falta %s en %+v", kind, findings)
		}
	}
	for _, f := range findings {
		if f.Category != "capture" {
			t.Fatalf("categoría inesperada: %+v", f)
		}
		if strings.Contains(f.Detail, "10.0.0.2") && f.Type == "passive_dhcp_server" {
			t.Fatalf("se marcó un servidor autorizado: %+v", f)
		}
	}
	// Dos hallazgos son passive_dns_tunnel: entropía alta y volumen de dominios.
	if len(findings) != 6 {
		t.Fatalf("esperaba 6 hallazgos: %+v", findings)
	}
}

func TestDetectPassiveGatewayDHCPIsTrusted(t *testing.T) {
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "d4:6e:0e:57:68:8a", IsGateway: true}}
	obs := []model.PassiveObservation{{Kind: "dhcp_server", SrcIP: "192.168.0.1", Detail: "DHCP OFFER"}}
	findings := DetectPassive(obs, arp, passiveCfg())
	if hasPassiveType(findings, "passive_dhcp_server") {
		t.Fatalf("el gateway no debe generar alerta: %+v", findings)
	}
}

func TestDetectPassiveEmpty(t *testing.T) {
	if findings := DetectPassive(nil, nil, passiveCfg()); len(findings) != 0 {
		t.Fatalf("sin observaciones no debe haber hallazgos: %+v", findings)
	}
}

func itoaForTest(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
