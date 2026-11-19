package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestEnterpriseDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	_, _, _, _, err := s.st.UpsertHost(&model.Host{IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:ff", DeviceType: model.DevAccessPoint, SiteID: 1})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleEnterpriseDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/enterprise?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var report model.DiagnosticsReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SiteID != 1 || !report.EnterpriseMode {
		t.Fatalf("informe inesperado: %+v", report)
	}
	if len(report.Limitations) < 4 {
		t.Fatalf("faltan limitaciones documentadas: %+v", report.Limitations)
	}
	if report.HostsAnalyzed != 1 {
		t.Fatalf("esperaba 1 host analizado, tiene %d", report.HostsAnalyzed)
	}
	found := false
	for _, a := range report.Anomalies {
		if a.Type == "rogue_ap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperaba rogue_ap en el informe: %+v", report.Anomalies)
	}
}

func TestARPDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	now := time.Now()
	err := s.st.SaveARPObservations([]model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IsGateway: true, ObservedAt: now.Add(-48 * time.Hour)},
		{SiteID: 1, ScanID: 2, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:02", IsGateway: true, ObservedAt: now},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleARPDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/arp?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Observations []model.ARPObservation    `json:"observations"`
		Anomalies    []model.DiagnosticAnomaly `json:"anomalies"`
		Limitations  []string                  `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Observations) != 2 {
		t.Fatalf("esperaba 2 observaciones: %+v", body.Observations)
	}
	found := false
	for _, a := range body.Anomalies {
		if a.Type == "arp_gateway_mac_changed" && a.Severity == model.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperaba arp_gateway_mac_changed crítico: %+v", body.Anomalies)
	}
	if len(body.Limitations) == 0 {
		t.Fatal("faltan limitaciones del análisis ARP")
	}
}

func TestDHCPDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	now := time.Now()
	if err := s.st.SaveDHCPObservations([]model.DHCPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.1", ServerID: "192.168.0.1", MAC: "de:ad:be:ef:00:01",
			MessageType: "ack", ObservedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.st.SaveARPObservations([]model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.1", MAC: "11:22:33:44:55:01", IsGateway: true, ObservedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleDHCPDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/dhcp?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Observations []model.DHCPObservation   `json:"observations"`
		Anomalies    []model.DiagnosticAnomaly `json:"anomalies"`
		Limitations  []string                  `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Observations) != 1 || body.Observations[0].ServerID != "192.168.0.1" {
		t.Fatalf("observaciones inesperadas: %+v", body.Observations)
	}
	found := false
	for _, a := range body.Anomalies {
		if a.Type == "dhcp_gateway_spoof" && a.Severity == model.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperaba dhcp_gateway_spoof crítico: %+v", body.Anomalies)
	}
}

func TestSNMPDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	now := time.Now()
	if _, _, _, _, err := s.st.UpsertHost(&model.Host{
		IP: "10.0.0.2", MAC: "aa:bb:cc:dd:ee:02", SiteID: 1,
		SNMP: &model.SNMPSummary{Community: "pu***ic"},
	}); err != nil {
		t.Fatal(err)
	}
	deep := model.SNMPDeep{
		IPAddresses: []model.SNMPIPAddress{{IP: "10.0.0.2", IfIndex: 2, Mask: "255.255.255.0"}},
		ARP: []model.SNMPARPRow{
			{ScanID: 6, HostIP: "10.0.0.2", IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", Type: "dynamic", Table: "net_to_media", ObservedAt: now.Add(-time.Hour)},
			{ScanID: 7, HostIP: "10.0.0.2", IP: "192.168.0.1", MAC: "de:ad:be:ef:00:01", Type: "dynamic", Table: "net_to_media", ObservedAt: now},
		},
		VLANs: []model.SNMPVLAN{{VLAN: 10, Name: "Red"}},
		FDB: []model.SNMPFDBRow{{
			ScanID: 7, HostIP: "10.0.0.2", MAC: "de:ad:be:ef:00:01", Port: 5, IfIndex: 7,
			VLAN: 10, Status: "learned", Table: "q_bridge", ObservedAt: now,
		}},
	}
	if err := s.st.SaveSNMPDeep(1, 6, "10.0.0.2", "pu***ic", now.Add(-time.Hour), model.SNMPDeep{
		ARP: deep.ARP[:1],
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.st.SaveSNMPDeep(1, 7, "10.0.0.2", "pu***ic", now, model.SNMPDeep{
		IPAddresses: deep.IPAddresses, ARP: deep.ARP[1:], VLANs: deep.VLANs, FDB: deep.FDB,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.st.SaveARPObservations([]model.ARPObservation{
		{SiteID: 1, ScanID: 6, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IsGateway: true, ObservedAt: now.Add(-time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleSNMPDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/snmp?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Anomalies   []model.DiagnosticAnomaly `json:"anomalies"`
		IPAddresses []model.SNMPIPAddress     `json:"ip_addresses"`
		ARP         []model.SNMPARPRow        `json:"arp"`
		VLANs       []model.SNMPVLAN          `json:"vlans"`
		FDB         []model.SNMPFDBRow        `json:"fdb"`
		Communities map[string]string         `json:"communities"`
		Limitations []string                  `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.IPAddresses) != 1 || len(body.ARP) != 2 || len(body.VLANs) != 1 || len(body.FDB) != 1 {
		t.Fatalf("tablas inesperadas: %s", rec.Body.String())
	}
	if body.Communities["10.0.0.2"] != "pu***ic" {
		t.Fatalf("comunidad ausente o en claro: %s", rec.Body.String())
	}
	found := false
	for _, a := range body.Anomalies {
		if a.Type == "snmp_gateway_arp_conflict" && a.Category == "snmp" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperaba snmp_gateway_arp_conflict: %s", rec.Body.String())
	}
	if len(body.Limitations) == 0 {
		t.Fatal("faltan limitaciones del análisis SNMP")
	}
}

func TestPassiveDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	now := time.Now()
	obs := []model.PassiveObservation{
		{SiteID: 1, ScanID: 7, Kind: "icmp_redirect", SrcIP: "192.168.0.1", DstIP: "192.168.0.50", ObservedAt: now},
		{SiteID: 1, ScanID: 7, Kind: "dhcp_server", SrcIP: "192.168.0.51", SrcMAC: "de:ad:be:ef:00:51", Detail: "DHCP OFFER", ObservedAt: now},
	}
	if err := s.st.SavePassiveObservations(obs); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handlePassiveDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/passive?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available    bool                       `json:"available"`
		Enabled      bool                       `json:"enabled"`
		Anomalies    []model.DiagnosticAnomaly  `json:"anomalies"`
		Observations []model.PassiveObservation `json:"observations"`
		Limitations  []string                   `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Observations) != 2 {
		t.Fatalf("observaciones inesperadas: %s", rec.Body.String())
	}
	if len(body.Anomalies) == 0 {
		t.Fatalf("faltan hallazgos pasivos: %s", rec.Body.String())
	}
	for _, a := range body.Anomalies {
		if !strings.HasPrefix(a.Type, "passive_") || a.Category != "capture" {
			t.Fatalf("hallazgo inesperado: %+v", a)
		}
	}
	if len(body.Limitations) == 0 {
		t.Fatal("faltan limitaciones de la captura pasiva")
	}
}

func TestSTPDiagnosticsEndpointShape(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleSTPDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/stp", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		SiteID      int64           `json:"site_id"`
		Loops       []model.STPLoop `json:"loops"`
		Anomalies   []any           `json:"anomalies"`
		Limitations []string        `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Loops == nil || body.Anomalies == nil || body.Limitations == nil {
		t.Fatalf("respuesta incompleta: %s", rec.Body.String())
	}
}

func TestVLANDiagnosticsEndpointShape(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleVLANDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/vlan", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Leaks       []model.VLANLeak `json:"leaks"`
		Anomalies   []any            `json:"anomalies"`
		Limitations []string         `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Leaks == nil || body.Anomalies == nil || body.Limitations == nil {
		t.Fatalf("respuesta incompleta: %s", rec.Body.String())
	}
}
