package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/capture"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// diagnosticsInput gathers one site snapshot from the store and local interfaces.
// Every source is optional: a missing source degrades the report instead of
// failing the request, so diagnostics remain useful on a fresh installation.
func (s *Server) diagnosticsInput(r *http.Request) (diagnostics.Input, time.Time, time.Time) {
	siteID := queryInt64(r, "site_id")
	from, to := parseWindow(r, 24*time.Hour)
	return s.diagnosticsFor(siteID, from, to), from, to
}

// diagnosticsFor is the request-free form used by exports such as the
// one-page executive report. It reads the same local sources as the API.
func (s *Server) diagnosticsFor(siteID int64, from, to time.Time) diagnostics.Input {
	cfg := config.Diagnostics{EnterpriseMode: true, MinConfidence: 60, MinAvailability: 99,
		MaxRTTMS: 200, MaxAnomalies: 200, StormHosts: 3}
	if s.cfg != nil {
		cfg = s.cfg.Diagnostics
	}

	in := diagnostics.Input{
		SiteID: siteID, From: from, To: to, Config: cfg,
		Neighbors: map[int64][]model.Neighbor{},
	}

	hosts, err := s.st.ListAllHostsForSite(siteID, 20000)
	if err == nil {
		in.Hosts = hosts
	}
	neighbors, err := s.st.NeighborsForSite(siteID)
	if err == nil {
		in.Neighbors = neighbors
	}
	if siteID > 0 {
		in.Networks, err = s.st.ListNetworksBySite(siteID)
	} else {
		in.Networks, err = s.st.ListNetworks()
	}
	if err != nil {
		in.Networks = nil
	}
	in.Metrics, err = s.st.MetricsForSite(siteID, from, 50000)
	if err != nil {
		in.Metrics = nil
	}
	in.Interfaces, err = s.systemInterfaces()
	if err != nil {
		in.Interfaces = nil
	}
	arpDays := cfg.ARPHistoryDays
	if arpDays < 1 {
		arpDays = 30
	}
	in.ARP, err = s.st.ListARPObservations(siteID, to.AddDate(0, 0, -arpDays), 50000)
	if err != nil {
		in.ARP = nil
	}
	dhcpDays := cfg.DHCPHistoryDays
	if dhcpDays < 1 {
		dhcpDays = 30
	}
	in.DHCP, err = s.st.ListDHCPObservations(siteID, to.AddDate(0, 0, -dhcpDays), 50000)
	if err != nil {
		in.DHCP = nil
	}
	snmpDays := cfg.SNMPHistoryDays
	if snmpDays < 1 {
		snmpDays = 30
	}
	in.SNMP, err = s.st.ListSNMPDeep(siteID, to.AddDate(0, 0, -snmpDays), 50000)
	if err != nil {
		in.SNMP = nil
	}
	passiveDays := cfg.PassiveHistoryDays
	if passiveDays < 1 {
		passiveDays = 7
	}
	in.Passive, err = s.st.ListPassiveObservations(siteID, to.AddDate(0, 0, -passiveDays), 50000)
	if err != nil {
		in.Passive = nil
	}
	return in
}

// handleEnterpriseDiagnostics returns every detector's findings plus the
// explicit limitations of the heuristic analysis.
func (s *Server) handleEnterpriseDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, _, _ := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	report.Anomalies = s.attachFindingReviews(in.SiteID, report.Anomalies)
	writeJSON(w, http.StatusOK, report)
}

// handleSTPDiagnostics returns the L2 cycle findings only.
func (s *Server) handleSTPDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, _, _ := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": report.From, "to": report.To,
		"loops": report.STP, "anomalies": filterAnomalies(report.Anomalies, "stp"),
		"limitations": report.Limitations,
	})
}

// handleVLANDiagnostics returns the possible VLAN leak findings only.
func (s *Server) handleVLANDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, _, _ := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": report.From, "to": report.To,
		"leaks": report.VLAN, "anomalies": filterAnomalies(report.Anomalies, "vlan_leak"),
		"limitations": report.Limitations,
	})
}

// handleARPDiagnostics returns the IP→MAC findings plus their raw history so
// the operator can contrast each alert with the evidence behind it.
func (s *Server) handleARPDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, from, to := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	findings := make([]model.DiagnosticAnomaly, 0)
	for _, a := range report.Anomalies {
		if strings.HasPrefix(a.Type, "arp_") {
			findings = append(findings, a)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": from, "to": to,
		"anomalies": findings, "observations": in.ARP,
		"limitations": report.Limitations,
	})
}

// handleDHCPDiagnostics returns rogue-server findings with the raw replies
// collected by the active UDP probe.
func (s *Server) handleDHCPDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, from, to := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	findings := make([]model.DiagnosticAnomaly, 0)
	for _, a := range report.Anomalies {
		if strings.HasPrefix(a.Type, "dhcp_") {
			findings = append(findings, a)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": from, "to": to,
		"anomalies": findings, "observations": in.DHCP,
		"limitations": report.Limitations,
	})
}

// handleSNMPDiagnostics returns deep SNMP findings with the raw IP-MIB, FDB
// and VLAN rows behind them. Communities are masked by the collector.
func (s *Server) handleSNMPDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, from, to := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	findings := make([]model.DiagnosticAnomaly, 0)
	for _, a := range report.Anomalies {
		if strings.HasPrefix(a.Type, "snmp_") {
			findings = append(findings, a)
		}
	}
	snap := in.SNMP
	if snap == nil {
		snap = &model.SNMPDeepSnapshot{
			IPAddresses: []model.SNMPIPAddress{}, ARP: []model.SNMPARPRow{},
			VLANs: []model.SNMPVLAN{}, FDB: []model.SNMPFDBRow{},
		}
	}
	communities := map[string]string{}
	for _, h := range in.Hosts {
		if h.SNMP != nil && h.SNMP.Community != "" {
			communities[h.IP] = h.SNMP.Community
		}
	}
	if snap.IPAddresses == nil {
		snap.IPAddresses = []model.SNMPIPAddress{}
	}
	if snap.ARP == nil {
		snap.ARP = []model.SNMPARPRow{}
	}
	if snap.VLANs == nil {
		snap.VLANs = []model.SNMPVLAN{}
	}
	if snap.FDB == nil {
		snap.FDB = []model.SNMPFDBRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": from, "to": to,
		"anomalies":    findings,
		"communities":  communities,
		"ip_addresses": snap.IPAddresses, "arp": snap.ARP,
		"vlans": snap.VLANs, "fdb": snap.FDB,
		"limitations": report.Limitations,
	})
}

// handlePassiveDiagnostics returns passive-capture findings and the metadata
// behind them. The available flag tells the UI whether the binary was built
// with pcap support; an empty window is not an error.
func (s *Server) handlePassiveDiagnostics(w http.ResponseWriter, r *http.Request) {
	in, from, to := s.diagnosticsInput(r)
	report := diagnostics.Analyzer{}.Analyze(in)
	findings := make([]model.DiagnosticAnomaly, 0)
	for _, a := range report.Anomalies {
		if strings.HasPrefix(a.Type, "passive_") {
			findings = append(findings, a)
		}
	}
	observations := in.Passive
	if observations == nil {
		observations = []model.PassiveObservation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id": in.SiteID, "generated_at": report.GeneratedAt,
		"from": from, "to": to,
		"available": capture.Available(), "enabled": in.Config.PassiveCapture,
		"anomalies": findings, "observations": observations,
		"limitations": report.Limitations,
	})
}

func filterAnomalies(all []model.DiagnosticAnomaly, kind string) []model.DiagnosticAnomaly {
	out := make([]model.DiagnosticAnomaly, 0)
	for _, a := range all {
		if a.Type == kind {
			out = append(out, a)
		}
	}
	return out
}
