package diagnostics

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func dhcpTestCfg() config.Diagnostics {
	return config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, DHCPHistoryDays: 30}
}

func hasDHCPType(findings []model.DiagnosticAnomaly, kind string) bool {
	for _, f := range findings {
		if f.Type == kind {
			return true
		}
	}
	return false
}

func TestDHCPUnauthorizedServer(t *testing.T) {
	now := time.Now()
	obs := []model.DHCPObservation{{
		IP: "192.168.0.50", ServerID: "192.168.0.50", MAC: "aa:bb:cc:dd:ee:50",
		MessageType: "offer", ScanID: 1, ObservedAt: now,
	}}
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "11:22:33:44:55:01", IsGateway: true, ScanID: 1, ObservedAt: now}}
	findings := DetectDHCP(obs, arp, dhcpTestCfg())
	if !hasDHCPType(findings, "dhcp_unauthorized_server") {
		t.Fatalf("esperaba servidor no autorizado: %+v", findings)
	}
}

func TestDHCPGatewaySpoof(t *testing.T) {
	now := time.Now()
	obs := []model.DHCPObservation{{
		IP: "192.168.0.1", ServerID: "192.168.0.1", MAC: "de:ad:be:ef:00:01",
		MessageType: "ack", ScanID: 1, ObservedAt: now,
	}}
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "11:22:33:44:55:01", IsGateway: true, ScanID: 1, ObservedAt: now}}
	findings := DetectDHCP(obs, arp, dhcpTestCfg())
	if !hasDHCPType(findings, "dhcp_gateway_spoof") {
		t.Fatalf("esperaba suplantación del gateway: %+v", findings)
	}
	if hasDHCPType(findings, "dhcp_unauthorized_server") {
		t.Fatalf("el gateway suplantado no debe duplicarse como no autorizado: %+v", findings)
	}
}

func TestDHCPMultipleServers(t *testing.T) {
	now := time.Now()
	obs := []model.DHCPObservation{
		{IP: "192.168.0.1", ServerID: "192.168.0.1", MAC: "11:22:33:44:55:01", MessageType: "ack", ScanID: 1, ObservedAt: now},
		{IP: "192.168.0.50", ServerID: "192.168.0.50", MAC: "aa:bb:cc:dd:ee:50", MessageType: "offer", ScanID: 1, ObservedAt: now},
	}
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "11:22:33:44:55:01", IsGateway: true, ScanID: 1, ObservedAt: now}}
	findings := DetectDHCP(obs, arp, dhcpTestCfg())
	if !hasDHCPType(findings, "dhcp_multiple_servers") {
		t.Fatalf("esperaba múltiples servidores: %+v", findings)
	}
}

func TestDHCPAuthorizedServerIgnored(t *testing.T) {
	now := time.Now()
	obs := []model.DHCPObservation{{
		IP: "192.168.0.50", ServerID: "192.168.0.50", MAC: "aa:bb:cc:dd:ee:50",
		MessageType: "offer", ScanID: 1, ObservedAt: now,
	}}
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "11:22:33:44:55:01", IsGateway: true, ScanID: 1, ObservedAt: now}}
	cfg := dhcpTestCfg()
	cfg.AuthorizedDHCPServers = []string{"192.168.0.50"}
	if findings := DetectDHCP(obs, arp, cfg); len(findings) != 0 {
		t.Fatalf("el servidor autorizado no debe alertar: %+v", findings)
	}
}

func TestDHCPServerMACChangedAcrossScans(t *testing.T) {
	now := time.Now()
	obs := []model.DHCPObservation{
		{IP: "192.168.0.50", ServerID: "192.168.0.50", MAC: "aa:bb:cc:dd:ee:50", MessageType: "offer", ScanID: 1, ObservedAt: now.Add(-time.Hour)},
		{IP: "192.168.0.50", ServerID: "192.168.0.50", MAC: "aa:bb:cc:dd:ee:99", MessageType: "offer", ScanID: 2, ObservedAt: now},
	}
	if findings := DetectDHCP(obs, nil, dhcpTestCfg()); !hasDHCPType(findings, "dhcp_server_mac_changed") {
		t.Fatalf("esperaba cambio de MAC del servidor: %+v", findings)
	}
}
