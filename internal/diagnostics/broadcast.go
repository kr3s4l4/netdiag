package diagnostics

import (
	"fmt"
	"sort"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// detectBroadcastStorms infers a storm from simultaneous loss/latency symptoms
// across several hosts of the same subnet. Requiring at least storm_hosts
// affected devices prevents a single unhealthy endpoint from raising a false
// positive. Raw broadcast counters are not available (see report.Limitations).
func detectBroadcastStorms(in Input, report *model.DiagnosticsReport) {
	if !in.Config.EnterpriseMode {
		return
	}
	minHosts := in.Config.StormHosts
	if minHosts < 2 {
		minHosts = 3
	}
	maxRTT := in.Config.MaxRTTMS
	if maxRTT <= 0 {
		maxRTT = 200
	}
	minAvail := in.Config.MinAvailability
	if minAvail <= 0 {
		minAvail = 99
	}
	stats := statsByHost(in)
	networks := indexNetworks(in.Networks)
	byNet := map[int64][]int64{}
	for _, h := range in.Hosts {
		if h.NetworkID > 0 {
			byNet[h.NetworkID] = append(byNet[h.NetworkID], h.ID)
		}
	}
	hosts := indexHosts(in.Hosts)
	netIDs := make([]int64, 0, len(byNet))
	for id := range byNet {
		netIDs = append(netIDs, id)
	}
	sort.Slice(netIDs, func(i, j int) bool { return netIDs[i] < netIDs[j] })

	for _, netID := range netIDs {
		affected := []int64{}
		symptoms := []string{}
		for _, id := range byNet[netID] {
			s := stats[id]
			if s.Samples < 3 {
				continue
			}
			bad := s.AvgLoss >= 20 && (s.AvgRTT >= maxRTT*0.7 || s.Availability <= minAvail-5)
			if !bad {
				continue
			}
			affected = append(affected, id)
			symptoms = append(symptoms, fmt.Sprintf("%s: pérdida %.1f%%, RTT %.1f ms, disponibilidad %.1f%%",
				hostLabel(hosts[id]), s.AvgLoss, s.AvgRTT, s.Availability))
		}
		if len(affected) < minHosts {
			continue
		}
		confidence := 60 + len(affected)*5
		if confidence > 90 {
			confidence = 90
		}
		if confidence < in.Config.MinConfidence {
			continue
		}
		evidence := append([]string{
			fmt.Sprintf("%d hosts de %s con pérdida y latencia simultáneas", len(affected), networkLabel(netID, networks)),
		}, symptoms...)
		if len(evidence) > 8 {
			evidence = evidence[:8]
		}
		add(report, model.DiagnosticAnomaly{
			ID: anomalyID("broadcast_storm", netID), Type: "broadcast_storm", Category: "performance",
			Severity: model.SeverityWarning, Title: "Posible tormenta de broadcast",
			Detail:  fmt.Sprintf("Varios hosts de %s presentan pérdida y latencia al mismo tiempo.", networkLabel(netID, networks)),
			HostIDs: affected, Confidence: confidence, Evidence: evidence,
			Recommendation: "Captura tráfico en la VLAN, revisa bucles L2, storm control y tráfico multicast/broadcast excesivo.",
			DetectedAt:     report.GeneratedAt,
		})
	}
}
