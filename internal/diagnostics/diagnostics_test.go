package diagnostics

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func testInput() Input {
	now := time.Now()
	return Input{
		SiteID: 1, From: now.Add(-time.Hour), To: now,
		Config: config.Diagnostics{
			EnterpriseMode: true, MinConfidence: 60, MinAvailability: 99,
			MaxRTTMS: 200, MaxAnomalies: 200, StormHosts: 3,
		},
		Neighbors: map[int64][]model.Neighbor{},
	}
}

func findAnomaly(report *model.DiagnosticsReport, kind string) *model.DiagnosticAnomaly {
	for i := range report.Anomalies {
		if report.Anomalies[i].Type == kind {
			return &report.Anomalies[i]
		}
	}
	return nil
}

func TestSTPDetectsCycleOfThreeDevices(t *testing.T) {
	in := testInput()
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", DeviceType: model.DevSwitch},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", DeviceType: model.DevSwitch},
		{ID: 3, IP: "10.0.0.3", MAC: "cc:cc:cc:cc:cc:cc", DeviceType: model.DevSwitch},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {{Protocol: "lldp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", RemotePort: "Gi0/2", Confidence: 95}},
		2: {{Protocol: "lldp", RemoteMAC: "cc:cc:cc:cc:cc:cc", LocalPort: "Gi0/3", RemotePort: "Gi0/4", Confidence: 95}},
		3: {{Protocol: "cdp", RemoteMAC: "aa:aa:aa:aa:aa:aa", LocalPort: "Gi0/5", RemotePort: "Gi0/6", Confidence: 90}},
	}
	report := (Analyzer{}).Analyze(in)
	loop := findAnomaly(report, "stp_loop")
	if loop == nil {
		t.Fatalf("esperaba anomaly stp_loop, obtenidas: %+v", report.Anomalies)
	}
	if len(loop.HostIDs) != 3 {
		t.Fatalf("el ciclo debe incluir 3 hosts, tiene %d", len(loop.HostIDs))
	}
	if len(report.STP) != 1 {
		t.Fatalf("esperaba 1 ciclo estructurado, obtenidos %d", len(report.STP))
	}
	if loop.Severity != model.SeverityCritical {
		t.Fatalf("un ciclo con confianza 90+ debe ser crítico, es %q", loop.Severity)
	}
	if len(loop.Evidence) == 0 {
		t.Fatal("la anomalía debe incluir evidencia")
	}
}

func TestSTPNoFalsePositiveOnSingleLink(t *testing.T) {
	in := testInput()
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", DeviceType: model.DevSwitch},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", DeviceType: model.DevSwitch},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {{Protocol: "lldp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", RemotePort: "Gi0/2", Confidence: 95}},
	}
	report := (Analyzer{}).Analyze(in)
	if findAnomaly(report, "stp_loop") != nil {
		t.Fatal("un único enlace no debe generar un lazo")
	}
}

func TestSTPLowConfidenceNeighborIgnored(t *testing.T) {
	in := testInput()
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", DeviceType: model.DevSwitch},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", DeviceType: model.DevSwitch},
		{ID: 3, IP: "10.0.0.3", MAC: "cc:cc:cc:cc:cc:cc", DeviceType: model.DevSwitch},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {{Protocol: "arp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", Confidence: 50}},
		2: {{Protocol: "arp", RemoteMAC: "cc:cc:cc:cc:cc:cc", LocalPort: "Gi0/3", Confidence: 50}},
		3: {{Protocol: "arp", RemoteMAC: "aa:aa:aa:aa:aa:aa", LocalPort: "Gi0/5", Confidence: 50}},
	}
	if got := (Analyzer{}).Analyze(in); findAnomaly(got, "stp_loop") != nil {
		t.Fatal("vecindades ARP de baja confianza no deben generar lazos")
	}
}

func TestVLANLeakSharedPort(t *testing.T) {
	in := testInput()
	in.Networks = []model.Network{{ID: 10, CIDR: "10.10.0.0/24"}, {ID: 20, CIDR: "10.20.0.0/24"}}
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.10.0.1", MAC: "aa:aa:aa:aa:aa:aa", DeviceType: model.DevSwitch, NetworkID: 10},
		{ID: 2, IP: "10.10.0.2", MAC: "bb:bb:bb:bb:bb:bb", NetworkID: 10},
		{ID: 3, IP: "10.20.0.3", MAC: "cc:cc:cc:cc:cc:cc", NetworkID: 20},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {
			{Protocol: "lldp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", Confidence: 95},
			{Protocol: "lldp", RemoteMAC: "cc:cc:cc:cc:cc:cc", LocalPort: "Gi0/1", Confidence: 95},
		},
	}
	report := (Analyzer{}).Analyze(in)
	leak := findAnomaly(report, "vlan_leak")
	if leak == nil {
		t.Fatalf("esperaba anomaly vlan_leak, obtenidas: %+v", report.Anomalies)
	}
	if len(report.VLAN) != 1 {
		t.Fatalf("esperaba 1 fuga estructurada, obtenidas %d", len(report.VLAN))
	}
	if report.VLAN[0].Port != "Gi0/1" {
		t.Fatalf("puerto incorrecto: %q", report.VLAN[0].Port)
	}
}

func TestVLANLeakIgnoresDifferentPorts(t *testing.T) {
	in := testInput()
	in.Networks = []model.Network{{ID: 10, CIDR: "10.10.0.0/24"}, {ID: 20, CIDR: "10.20.0.0/24"}}
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.10.0.1", MAC: "aa:aa:aa:aa:aa:aa", DeviceType: model.DevSwitch, NetworkID: 10},
		{ID: 2, IP: "10.10.0.2", MAC: "bb:bb:bb:bb:bb:bb", NetworkID: 10},
		{ID: 3, IP: "10.20.0.3", MAC: "cc:cc:cc:cc:cc:cc", NetworkID: 20},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {
			{Protocol: "lldp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", Confidence: 95},
			{Protocol: "lldp", RemoteMAC: "cc:cc:cc:cc:cc:cc", LocalPort: "Gi0/2", Confidence: 95},
		},
	}
	if got := (Analyzer{}).Analyze(in); findAnomaly(got, "vlan_leak") != nil {
		t.Fatal("puertos distintos representan un trunk legítimo, no una fuga")
	}
}

func TestDuplexMismatchRequiresSpeedDifference(t *testing.T) {
	in := testInput()
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", SNMP: &model.SNMPSummary{Interfaces: []model.SNMPInterface{
			{Index: 1, Name: "Gi0/1", SpeedMbps: 100},
		}}},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", SNMP: &model.SNMPSummary{Interfaces: []model.SNMPInterface{
			{Index: 1, Name: "Gi0/2", SpeedMbps: 1000},
		}}},
	}
	in.Neighbors = map[int64][]model.Neighbor{
		1: {{Protocol: "lldp", RemoteMAC: "bb:bb:bb:bb:bb:bb", LocalPort: "Gi0/1", RemotePort: "Gi0/2", Confidence: 95}},
	}
	report := (Analyzer{}).Analyze(in)
	if findAnomaly(report, "duplex_mismatch") == nil {
		t.Fatalf("esperaba anomaly duplex_mismatch, obtenidas: %+v", report.Anomalies)
	}
	if len(report.Duplex) != 1 || report.Duplex[0].SpeedA != 100 || report.Duplex[0].SpeedB != 1000 {
		t.Fatalf("datos de dúplex incorrectos: %+v", report.Duplex)
	}
}

func TestSLAStillRunsWithEnterpriseModeOff(t *testing.T) {
	in := testInput()
	in.Config.EnterpriseMode = false
	in.Hosts = []model.Host{{ID: 1, IP: "10.0.0.9"}}
	for i := 0; i < 10; i++ {
		in.Metrics = append(in.Metrics, model.Metric{HostID: 1, Up: i < 8, RTTMS: 300, Loss: 20})
	}
	report := (Analyzer{}).Analyze(in)
	if report.EnterpriseMode {
		t.Fatal("el modo empresarial debe reflejarse como desactivado")
	}
	if findAnomaly(report, "sla_breach") == nil {
		t.Fatalf("SLA debe evaluarse siempre, obtenidas: %+v", report.Anomalies)
	}
	if findAnomaly(report, "stp_loop") != nil || findAnomaly(report, "broadcast_storm") != nil {
		t.Fatal("los detectores empresariales no deben ejecutarse con el modo desactivado")
	}
}

func TestBroadcastStormNeedsMultipleHosts(t *testing.T) {
	in := testInput()
	in.Networks = []model.Network{{ID: 10, CIDR: "10.10.0.0/24"}}
	in.Hosts = []model.Host{{ID: 1, NetworkID: 10}, {ID: 2, NetworkID: 10}, {ID: 3, NetworkID: 10}}
	for _, id := range []int64{1, 2} {
		for i := 0; i < 5; i++ {
			in.Metrics = append(in.Metrics, model.Metric{HostID: id, Up: i < 3, RTTMS: 180, Loss: 30})
		}
	}
	if got := (Analyzer{}).Analyze(in); findAnomaly(got, "broadcast_storm") != nil {
		t.Fatal("solo 2 hosts afectados no debe disparar tormenta con storm_hosts=3")
	}
	for i := 0; i < 5; i++ {
		in.Metrics = append(in.Metrics, model.Metric{HostID: 3, Up: i < 3, RTTMS: 180, Loss: 30})
	}
	if got := (Analyzer{}).Analyze(in); findAnomaly(got, "broadcast_storm") == nil {
		t.Fatal("3 hosts afectados deben disparar tormenta")
	}
}

func TestRogueAPUsesAuthorizedList(t *testing.T) {
	in := testInput()
	in.Hosts = []model.Host{{ID: 1, IP: "10.0.0.5", MAC: "aa:bb:cc:dd:ee:ff", DeviceType: model.DevAccessPoint}}
	in.Config.AuthorizedAPs = []string{"AA:BB:CC:DD:EE:FF"}
	if got := (Analyzer{}).Analyze(in); findAnomaly(got, "rogue_ap") != nil {
		t.Fatal("un AP autorizado no debe generar una anomalía")
	}
	in.Config.AuthorizedAPs = []string{"11:22:33:44:55:66"}
	report := (Analyzer{}).Analyze(in)
	anomaly := findAnomaly(report, "rogue_ap")
	if anomaly == nil || anomaly.Confidence != 95 {
		t.Fatalf("un AP no autorizado debe generar una anomalía confiable: %+v", anomaly)
	}
}

func TestLimitationsAndAnomalyCap(t *testing.T) {
	in := testInput()
	in.Config.MaxAnomalies = 1
	in.Hosts = []model.Host{
		{ID: 1, IP: "10.0.0.5", MAC: "aa:bb:cc:dd:ee:ff", DeviceType: model.DevAccessPoint},
		{ID: 2, IP: "10.0.0.6", MAC: "11:22:33:44:55:66", DeviceType: model.DevAccessPoint},
	}
	report := (Analyzer{}).Analyze(in)
	if len(report.Anomalies) != 1 {
		t.Fatalf("el límite debe aplicarse, hay %d anomalías", len(report.Anomalies))
	}
	if report.Anomalies[0].Type != "rogue_ap" {
		t.Fatalf("se esperaba rogue_ap como primera anomalía, tiene %q", report.Anomalies[0].Type)
	}
	if len(report.Limitations) < 4 {
		t.Fatalf("el informe debe documentar sus limitaciones: %+v", report.Limitations)
	}
}
