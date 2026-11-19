// Package diagnostics turns the inventory and telemetry already collected by
// NetDiag into enterprise-oriented findings. It deliberately avoids raw packet
// capture: every anomaly is backed by observable evidence and a confidence
// score, and the report states which signals are unavailable.
package diagnostics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// Input is everything the analyzers need for one site and time window.
type Input struct {
	SiteID     int64
	From       time.Time
	To         time.Time
	Hosts      []model.Host
	Neighbors  map[int64][]model.Neighbor
	Networks   []model.Network
	Metrics    []model.Metric
	Interfaces []model.SystemInterface
	ARP        []model.ARPObservation
	DHCP       []model.DHCPObservation
	SNMP       *model.SNMPDeepSnapshot
	Passive    []model.PassiveObservation
	Config     config.Diagnostics
}

// Analyzer runs all detectors over one site snapshot.
type Analyzer struct{}

// Analyze produces a report. Anomalies are sorted by severity and confidence
// and capped by cfg.max_anomalies so a noisy network cannot flood the UI.
func (Analyzer) Analyze(in Input) *model.DiagnosticsReport {
	to := in.To
	if to.IsZero() {
		to = time.Now()
	}
	from := in.From
	if from.IsZero() || !from.Before(to) {
		from = to.Add(-24 * time.Hour)
	}
	cfg := in.Config
	if cfg.MinConfidence < 1 {
		cfg.MinConfidence = 60
	}
	if cfg.MaxAnomalies < 1 {
		cfg.MaxAnomalies = 200
	}
	if cfg.StormHosts < 2 {
		cfg.StormHosts = 3
	}
	in.Config, in.From, in.To = cfg, from, to

	report := &model.DiagnosticsReport{
		SiteID:          in.SiteID,
		GeneratedAt:     to,
		From:            from,
		To:              to,
		EnterpriseMode:  cfg.EnterpriseMode,
		HostsAnalyzed:   len(in.Hosts),
		SamplesAnalyzed: len(in.Metrics),
		Anomalies:       []model.DiagnosticAnomaly{},
		Counts:          map[string]int{},
		STP:             []model.STPLoop{},
		VLAN:            []model.VLANLeak{},
		Duplex:          []model.DuplexMismatch{},
		MTU:             []model.MTUProblem{},
		Limitations: []string{
			"Sin captura de BPDUs 802.1D/802.1w: los ciclos se infieren de vecindades LLDP/CDP/FDB de alta confianza.",
			"Los OID de VLAN (Q-BRIDGE), ARP y FDB se leen por SNMP sin captura: las fugas se infieren de esas tablas y de las adyacencias L2; no hay contadores broadcast.",
			"Sin OID de dúplex ni contadores de error: la discrepancia se compara entre velocidades IF-MIB de ambos extremos.",
			"Sin sondeo de fragmentación activo: los problemas de MTU se correlacionan con interfaces locales de MTU bajo y pérdida observada.",
			"Sin captura ARP en vivo: los cambios IP→MAC se infieren del histórico de escaneos y son pistas, no confirmación de spoofing activo.",
			"El sondeo DHCP usa UDP/68 sin raw sockets: solo ve respuestas a DHCPINFORM/DHCPDISCOVER, no el tráfico de otros clientes.",
			"La comunidad SNMP se guarda enmascarada; los walks profundos tienen límite de filas y pueden desactivarse con diagnostics.snmp_deep: false.",
			"La captura pasiva es opcional (diagnostics.passive_capture): solo se activa con -tags pcap y CAP_NET_RAW, guarda metadatos sin payload y necesita validación humana.",
		},
	}

	if !cfg.EnterpriseMode {
		report.Limitations = append(report.Limitations,
			"Modo empresarial desactivado: solo se evalúan los umbrales SLA.")
	}

	report.STP = detectSTPLoops(in, report)
	report.VLAN = detectVLANLeaks(in, report)
	report.Duplex = detectDuplexMismatch(in, report)
	report.MTU = detectMTUProblems(in, report)
	if report.STP == nil {
		report.STP = []model.STPLoop{}
	}
	if report.VLAN == nil {
		report.VLAN = []model.VLANLeak{}
	}
	if report.Duplex == nil {
		report.Duplex = []model.DuplexMismatch{}
	}
	if report.MTU == nil {
		report.MTU = []model.MTUProblem{}
	}
	detectBroadcastStorms(in, report)
	detectRogueAPs(in, report)
	detectSLABreaches(in, report)
	if cfg.EnterpriseMode {
		for _, a := range DetectARP(in.ARP, cfg) {
			add(report, a)
		}
		for _, a := range DetectDHCP(in.DHCP, in.ARP, cfg) {
			add(report, a)
		}
		for _, a := range DetectSNMP(in.SNMP, in.ARP, in.Hosts, in.Neighbors, cfg) {
			add(report, a)
		}
		for _, a := range DetectPassive(in.Passive, in.ARP, cfg) {
			add(report, a)
		}
	}

	severityRank := map[string]int{model.SeverityCritical: 0, model.SeverityWarning: 1, model.SeverityInfo: 2}
	sort.SliceStable(report.Anomalies, func(i, j int) bool {
		ri, rj := severityRank[report.Anomalies[i].Severity], severityRank[report.Anomalies[j].Severity]
		if ri != rj {
			return ri < rj
		}
		if report.Anomalies[i].Confidence != report.Anomalies[j].Confidence {
			return report.Anomalies[i].Confidence > report.Anomalies[j].Confidence
		}
		return report.Anomalies[i].ID < report.Anomalies[j].ID
	})
	if len(report.Anomalies) > cfg.MaxAnomalies {
		report.Anomalies = report.Anomalies[:cfg.MaxAnomalies]
	}
	for _, a := range report.Anomalies {
		report.Counts[a.Severity]++
		report.Counts[a.Type]++
	}
	return report
}

// add appends an anomaly unless another finding already used the same ID.
func add(report *model.DiagnosticsReport, a model.DiagnosticAnomaly) {
	for _, existing := range report.Anomalies {
		if existing.ID == a.ID {
			return
		}
	}
	report.Anomalies = append(report.Anomalies, a)
}

func anomalyID(kind string, ids ...int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return kind + ":" + strings.Join(parts, "-")
}

func hostLabel(h *model.Host) string {
	if h == nil {
		return "desconocido"
	}
	if h.Hostname != "" {
		return h.Hostname + " (" + h.IP + ")"
	}
	if h.IP != "" {
		return h.IP
	}
	if h.MAC != "" {
		return h.MAC
	}
	return fmt.Sprintf("host %d", h.ID)
}

func hostByID(index map[int64]*model.Host, id int64) *model.Host {
	return index[id]
}

func indexHosts(hosts []model.Host) map[int64]*model.Host {
	out := make(map[int64]*model.Host, len(hosts))
	for i := range hosts {
		out[hosts[i].ID] = &hosts[i]
	}
	return out
}

// resolveNeighbor maps a discovered neighbor to an inventoried host. It uses
// MAC first (stable), then IP and finally name, all case-insensitively.
func resolveNeighbor(n model.Neighbor, byMAC, byIP, byName map[string]int64) int64 {
	for _, key := range []string{n.RemoteMAC, n.RemoteChassis} {
		if key = strings.ToLower(strings.TrimSpace(key)); key != "" {
			if id := byMAC[key]; id > 0 {
				return id
			}
		}
	}
	if key := strings.ToLower(strings.TrimSpace(n.RemoteIP)); key != "" {
		if id := byIP[key]; id > 0 {
			return id
		}
	}
	if key := strings.ToLower(strings.TrimSpace(n.RemoteName)); key != "" {
		if id := byName[key]; id > 0 {
			return id
		}
	}
	return 0
}

func neighborIndexes(hosts []model.Host) (map[string]int64, map[string]int64, map[string]int64) {
	byMAC, byIP, byName := map[string]int64{}, map[string]int64{}, map[string]int64{}
	for _, h := range hosts {
		if mac := strings.ToLower(strings.TrimSpace(h.MAC)); mac != "" {
			byMAC[mac] = h.ID
		}
		if ip := strings.TrimSpace(h.IP); ip != "" {
			byIP[ip] = h.ID
		}
		if name := strings.ToLower(strings.TrimSpace(h.Hostname)); name != "" {
			byName[name] = h.ID
		}
	}
	return byMAC, byIP, byName
}

func isL2Device(h *model.Host) bool {
	if h == nil {
		return false
	}
	switch h.DeviceType {
	case model.DevSwitch, model.DevAccessPoint:
		return true
	}
	return h.SNMP != nil && (len(h.SNMP.Interfaces) > 1 || h.SNMP.SysDescr != "")
}

func isTrustedProtocol(n model.Neighbor) bool {
	switch strings.ToLower(strings.TrimSpace(n.Protocol)) {
	case "lldp", "cdp":
		return n.Confidence >= 80
	case "bridge":
		return n.Confidence >= 60
	default:
		return false
	}
}

func networkLabel(id int64, networks map[int64]model.Network) string {
	if id == 0 {
		return "sin red"
	}
	if n, ok := networks[id]; ok {
		if n.Name != "" {
			return n.Name + " (" + n.CIDR + ")"
		}
		return n.CIDR
	}
	return fmt.Sprintf("red %d", id)
}

func indexNetworks(networks []model.Network) map[int64]model.Network {
	out := make(map[int64]model.Network, len(networks))
	for _, n := range networks {
		out[n.ID] = n
	}
	return out
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// matchesPort reports whether an interface name, alias or description refers
// to the given LLDP/CDP port identifier.
func matchesPort(iface model.SNMPInterface, port string) bool {
	port = strings.ToLower(strings.TrimSpace(port))
	if port == "" {
		return false
	}
	for _, candidate := range []string{iface.Name, iface.Alias, iface.Descr} {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == port || strings.HasSuffix(candidate, port) || strings.Contains(candidate, " "+port) {
			return true
		}
	}
	return false
}
