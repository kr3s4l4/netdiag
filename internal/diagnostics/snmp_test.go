package diagnostics

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func snmpTestCfg() config.Diagnostics {
	return config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, SNMPHistoryDays: 30}
}

func hasSNMPType(findings []model.DiagnosticAnomaly, kind string) bool {
	for _, f := range findings {
		if f.Type == kind {
			return true
		}
	}
	return false
}

func TestDetectSNMPGatewayARPConflict(t *testing.T) {
	now := time.Now()
	snap := &model.SNMPDeepSnapshot{ARP: []model.SNMPARPRow{
		{ScanID: 1, HostIP: "10.0.0.2", IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", Type: "dynamic", Table: "net_to_media", ObservedAt: now.Add(-time.Hour)},
		{ScanID: 2, HostIP: "10.0.0.2", IP: "192.168.0.1", MAC: "de:ad:be:ef:00:01", Type: "dynamic", Table: "net_to_media", ObservedAt: now},
	}}
	arp := []model.ARPObservation{{IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IsGateway: true, ObservedAt: now.Add(-time.Hour)}}
	findings := DetectSNMP(snap, arp, nil, nil, snmpTestCfg())
	if !hasSNMPType(findings, "snmp_gateway_arp_conflict") {
		t.Fatalf("esperaba gateway ARP conflict: %+v", findings)
	}
	for _, f := range findings {
		if f.Type == "snmp_gateway_arp_conflict" {
			if f.Category != "snmp" || f.Severity != model.SeverityCritical || f.Confidence < 90 {
				t.Fatalf("hallazgo inesperado: %+v", f)
			}
			return
		}
	}
}

func TestDetectSNMPFDBPortAndVLANConflict(t *testing.T) {
	now := time.Now()
	snap := &model.SNMPDeepSnapshot{
		VLANs: []model.SNMPVLAN{{ScanID: 1, HostIP: "10.0.0.2", VLAN: 10, ObservedAt: now}},
		FDB: []model.SNMPFDBRow{
			{ScanID: 1, HostIP: "10.0.0.2", MAC: "aa:bb:cc:dd:ee:01", Port: 5, IfIndex: 7, VLAN: 10, Status: "learned", Table: "q_bridge", ObservedAt: now},
			{ScanID: 1, HostIP: "10.0.0.2", MAC: "aa:bb:cc:dd:ee:01", Port: 6, IfIndex: 8, VLAN: 20, Status: "learned", Table: "q_bridge", ObservedAt: now},
		},
	}
	findings := DetectSNMP(snap, nil, nil, nil, snmpTestCfg())
	if !hasSNMPType(findings, "snmp_fdb_port_conflict") || !hasSNMPType(findings, "snmp_fdb_vlan_conflict") {
		t.Fatalf("esperaba conflictos de puerto y VLAN: %+v", findings)
	}
}

func TestDetectSNMPLLDPDisagree(t *testing.T) {
	now := time.Now()
	hosts := []model.Host{{
		ID: 1, IP: "10.0.0.2", SiteID: 1,
		SNMP: &model.SNMPSummary{Interfaces: []model.SNMPInterface{{Index: 7, Name: "Gi0/1"}}},
	}}
	neighbors := map[int64][]model.Neighbor{
		1: {{Protocol: "lldp", LocalPort: "Gi0/2", RemoteMAC: "aa:bb:cc:dd:ee:01", Confidence: 95}},
	}
	snap := &model.SNMPDeepSnapshot{FDB: []model.SNMPFDBRow{
		{ScanID: 1, HostIP: "10.0.0.2", MAC: "aa:bb:cc:dd:ee:01", Port: 5, IfIndex: 7, Status: "learned", Table: "q_bridge", ObservedAt: now},
	}}
	findings := DetectSNMP(snap, nil, hosts, neighbors, snmpTestCfg())
	if !hasSNMPType(findings, "snmp_lldp_fdb_disagree") {
		t.Fatalf("esperaba desacuerdo LLDP/FDB: %+v", findings)
	}
}

func TestDetectSNMPNilSnapshot(t *testing.T) {
	if findings := DetectSNMP(nil, nil, nil, nil, snmpTestCfg()); len(findings) != 0 {
		t.Fatalf("sin snapshot no debe haber hallazgos: %+v", findings)
	}
}
