package diagnostics

import (
	"fmt"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// detectMTUProblems correlates local low-MTU interfaces with loss symptoms on
// hosts attached to the matching monitored network. Fragmentation counters are
// not collected, so the confidence is deliberately capped at 75.
func detectMTUProblems(in Input, report *model.DiagnosticsReport) []model.MTUProblem {
	if !in.Config.EnterpriseMode {
		return nil
	}
	stats := statsByHost(in)
	var out []model.MTUProblem
	for _, iface := range in.Interfaces {
		if iface.Loopback || !iface.Up || iface.MTU <= 0 || iface.MTU >= 1500 {
			continue
		}
		ifaceName := strings.ToLower(strings.TrimSpace(iface.Name))
		var matchedNetworks []model.Network
		for _, n := range in.Networks {
			if strings.EqualFold(strings.TrimSpace(n.Interface), iface.Name) {
				matchedNetworks = append(matchedNetworks, n)
			}
		}
		if len(matchedNetworks) == 0 {
			continue
		}
		for _, network := range matchedNetworks {
			fragmented := 0
			affected := []int64{}
			for _, h := range in.Hosts {
				if h.NetworkID != network.ID {
					continue
				}
				s := stats[h.ID]
				if s.Samples >= 3 && s.AvgLoss >= 10 {
					fragmented++
					affected = append(affected, h.ID)
				}
			}
			if fragmented == 0 {
				continue
			}
			confidence := 60
			if fragmented >= 3 {
				confidence = 75
			}
			if confidence < in.Config.MinConfidence {
				continue
			}
			item := model.MTUProblem{
				Src: iface.Name, Dst: network.CIDR, MTUPath: iface.MTU,
				FragmentedCount: fragmented, Confidence: confidence,
				Evidence: []string{
					fmt.Sprintf("Interfaz local %s con MTU %d", ifaceName, iface.MTU),
					fmt.Sprintf("%d hosts de %s con pérdida media >= 10%%", fragmented, network.CIDR),
					"Sin contadores de fragmentación: correlación heurística",
				},
			}
			out = append(out, item)
			add(report, model.DiagnosticAnomaly{
				ID: anomalyID("mtu_problem", network.ID), Type: "mtu_problem", Category: "performance",
				Severity: model.SeverityWarning, Title: "Posible problema de MTU",
				Detail:  fmt.Sprintf("La ruta por %s usa MTU %d y hay pérdida en %s.", iface.Name, iface.MTU, network.CIDR),
				HostIDs: affected, Confidence: confidence, Evidence: item.Evidence,
				Recommendation: "Comprueba tunnel/TCP MSS clamping y usa un sondeo DF para confirmar el MTU de ruta.",
				DetectedAt:     report.GeneratedAt,
			})
		}
	}
	return out
}
