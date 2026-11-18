package diagnostics

import (
	"fmt"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// detectDuplexMismatch compares IF-MIB advertised speeds on both ends of an
// LLDP/CDP link. NetDiag cannot read the duplex OID itself, so the finding
// includes that limitation and requires a large speed difference.
func detectDuplexMismatch(in Input, report *model.DiagnosticsReport) []model.DuplexMismatch {
	if !in.Config.EnterpriseMode {
		return nil
	}
	hosts := indexHosts(in.Hosts)
	minConfidence := in.Config.MinConfidence
	var out []model.DuplexMismatch
	for _, link := range l2Links(in, minConfidence) {
		protocol := strings.ToLower(link.Neighbor.Protocol)
		if protocol != "lldp" && protocol != "cdp" {
			continue
		}
		a, b := hosts[link.A], hosts[link.B]
		if a == nil || b == nil || a.SNMP == nil || b.SNMP == nil {
			continue
		}
		ifaceA := findInterface(a.SNMP.Interfaces, link.LocalPort)
		ifaceB := findInterface(b.SNMP.Interfaces, link.RemotePort)
		if ifaceA == nil || ifaceB == nil || ifaceA.SpeedMbps <= 0 || ifaceB.SpeedMbps <= 0 || ifaceA.SpeedMbps == ifaceB.SpeedMbps {
			continue
		}
		lo, hi := ifaceA.SpeedMbps, ifaceB.SpeedMbps
		if lo > hi {
			lo, hi = hi, lo
		}
		// Ignore ordinary auto-negotiation steps (1000 vs 10000 is common on
		// uplinks); flag only clearly incompatible speeds.
		if lo >= 1000 {
			continue
		}
		confidence := link.Confidence
		if lo <= 100 {
			confidence += 5
		}
		if confidence > 95 {
			confidence = 95
		}
		if confidence < minConfidence {
			continue
		}
		port := link.LocalPort
		if port == "" {
			port = link.RemotePort
		}
		item := model.DuplexMismatch{
			Port: port, SpeedA: ifaceA.SpeedMbps, SpeedB: ifaceB.SpeedMbps,
			DuplexA: "desconocido", DuplexB: "desconocido",
			HostA: a.ID, HostB: b.ID, Confidence: confidence,
			Evidence: []string{
				fmt.Sprintf("%s anuncia %d Mbps en %s", hostLabel(a), ifaceA.SpeedMbps, ifaceA.Name),
				fmt.Sprintf("%s anuncia %d Mbps en %s", hostLabel(b), ifaceB.SpeedMbps, ifaceB.Name),
				"Dúplex no disponible: se compara únicamente la velocidad IF-MIB de ambos extremos",
			},
		}
		out = append(out, item)
		add(report, model.DiagnosticAnomaly{
			ID: anomalyID("duplex_mismatch", a.ID, b.ID), Type: "duplex_mismatch", Category: "duplex",
			Severity: model.SeverityWarning,
			Title:    "Posible desajuste de velocidad/dúplex",
			Detail:   fmt.Sprintf("Los extremos de %s anuncian %d y %d Mbps.", port, ifaceA.SpeedMbps, ifaceB.SpeedMbps),
			HostIDs:  []int64{a.ID, b.ID}, HostIPs: []string{a.IP, b.IP},
			Confidence: confidence, Evidence: item.Evidence,
			Recommendation: "Fuerza la misma velocidad y dúplex en ambos extremos o revisa el cableado y el puerto del switch.",
			DetectedAt:     report.GeneratedAt,
		})
	}
	return out
}

func findInterface(interfaces []model.SNMPInterface, port string) *model.SNMPInterface {
	if port == "" {
		return nil
	}
	for i := range interfaces {
		if matchesPort(interfaces[i], port) {
			return &interfaces[i]
		}
	}
	return nil
}
