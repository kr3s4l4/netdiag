package diagnostics

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func arpTestCfg() config.Diagnostics {
	return config.Diagnostics{EnterpriseMode: true, MinConfidence: 60, MaxAnomalies: 200, MACIPLimit: 5}
}

func arpTypes(findings []model.DiagnosticAnomaly) map[string]model.DiagnosticAnomaly {
	out := map[string]model.DiagnosticAnomaly{}
	for _, a := range findings {
		out[a.Type] = a
	}
	return out
}

func TestDetectARPGatewayMACChange(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	now := time.Now()
	obs := []model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IsGateway: true, ObservedAt: old},
		{SiteID: 1, ScanID: 2, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:02", IsGateway: true, ObservedAt: now},
	}
	got := arpTypes(DetectARP(obs, arpTestCfg()))
	a, ok := got["arp_gateway_mac_changed"]
	if !ok {
		t.Fatalf("no se detectó el cambio de MAC del gateway: %v", got)
	}
	if a.Severity != "critical" || a.Confidence < 90 {
		t.Fatalf("severidad o confianza incorrectas: %+v", a)
	}
	if len(a.Evidence) < 2 || len(a.HostIPs) != 1 || a.HostIPs[0] != "192.168.0.1" {
		t.Fatalf("evidencia u host incompletos: %+v", a)
	}
}

func TestDetectARPIPMACChangeWithoutGatewayFlag(t *testing.T) {
	obs := []model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.50", MAC: "aa:bb:cc:dd:11:01", ObservedAt: time.Now().Add(-24 * time.Hour)},
		{SiteID: 1, ScanID: 2, IP: "192.168.0.50", MAC: "aa:bb:cc:dd:22:02", ObservedAt: time.Now()},
	}
	got := arpTypes(DetectARP(obs, arpTestCfg()))
	a, ok := got["arp_ip_mac_changed"]
	if !ok {
		t.Fatalf("no se detectó el cambio IP→MAC: %v", got)
	}
	if a.Severity != "warning" {
		t.Fatalf("severidad incorrecta: %s", a.Severity)
	}
}

func TestDetectARPMACClaimingManyIPs(t *testing.T) {
	old := time.Now().Add(-24 * time.Hour)
	now := time.Now()
	obs := []model.ARPObservation{{SiteID: 1, ScanID: 1, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: old}}
	for i := 2; i <= 7; i++ {
		obs = append(obs, model.ARPObservation{
			SiteID: 1, ScanID: 2, IP: "10.0.0." + string(rune('0'+i)),
			MAC: "aa:bb:cc:dd:ee:01", ObservedAt: now,
		})
	}
	got := arpTypes(DetectARP(obs, arpTestCfg()))
	a, ok := got["arp_mac_multi_ip"]
	if !ok {
		t.Fatalf("no se detectó la MAC con muchas IPs: %v", got)
	}
	if len(a.HostIPs) != 6 {
		t.Fatalf("esperaba 6 IPs, obtuve %d", len(a.HostIPs))
	}
}

func TestDetectARPIPWithTwoMACs(t *testing.T) {
	obs := []model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: time.Now().Add(-24 * time.Hour)},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: time.Now()},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:02", ObservedAt: time.Now()},
	}
	got := arpTypes(DetectARP(obs, arpTestCfg()))
	a, ok := got["arp_ip_multi_mac"]
	if !ok {
		t.Fatalf("no se detectó la IP con dos MAC: %v", got)
	}
	if a.Confidence != 85 {
		t.Fatalf("confianza incorrecta: %d", a.Confidence)
	}
}

func TestDetectARPNeedsTwoBatchesAndValidMAC(t *testing.T) {
	obs := []model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: time.Now()},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.1", MAC: "ff:ff:ff:ff:ff:ff", ObservedAt: time.Now()},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.2", MAC: "01:00:5e:00:00:01", ObservedAt: time.Now()},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.3", MAC: "", ObservedAt: time.Now()},
	}
	if got := DetectARP(obs, arpTestCfg()); len(got) != 0 {
		t.Fatalf("no debería haber hallazgos: %v", got)
	}
}

func TestDetectARPRespectsMinConfidence(t *testing.T) {
	obs := []model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: time.Now().Add(-24 * time.Hour)},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:02", ObservedAt: time.Now()},
	}
	cfg := arpTestCfg()
	cfg.MinConfidence = 90
	if got := DetectARP(obs, cfg); len(got) != 0 {
		t.Fatalf("el filtro de confianza no se aplicó: %v", got)
	}
}
