package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// detectVLANLeaks flags an L2 port that reaches devices in different monitored
// subnets. A single inter-VLAN trunk between two switches would be a false
// positive, so a finding needs either a shared local port with several subnets
// or a bridge/FDB adjacency between two L2 devices on different networks.
func detectVLANLeaks(in Input, report *model.DiagnosticsReport) []model.VLANLeak {
	if !in.Config.EnterpriseMode {
		return nil
	}
	hosts := indexHosts(in.Hosts)
	networks := indexNetworks(in.Networks)
	byMAC, byIP, byName := neighborIndexes(in.Hosts)
	minConfidence := in.Config.MinConfidence
	if minConfidence < 70 {
		minConfidence = 70
	}

	type portGroup struct {
		localID int64
		port    string
		hosts   map[int64]bool
		remote  map[int64]model.Neighbor
	}
	groups := map[string]*portGroup{}
	for localID, neighbors := range in.Neighbors {
		for _, n := range neighbors {
			if !isTrustedProtocol(n) || n.Confidence < minConfidence {
				continue
			}
			port := strings.TrimSpace(n.LocalPort)
			if port == "" {
				continue
			}
			remoteID := resolveNeighbor(n, byMAC, byIP, byName)
			if remoteID == 0 {
				continue
			}
			key := fmt.Sprintf("%d|%s", localID, strings.ToLower(port))
			g := groups[key]
			if g == nil {
				g = &portGroup{localID: localID, port: port, hosts: map[int64]bool{}, remote: map[int64]model.Neighbor{}}
				groups[key] = g
			}
			g.hosts[remoteID] = true
			g.remote[remoteID] = n
		}
	}

	var out []model.VLANLeak
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		g := groups[key]
		netIDs := map[int64]bool{}
		for id := range g.hosts {
			if h := hosts[id]; h != nil && h.NetworkID > 0 {
				netIDs[h.NetworkID] = true
			}
		}
		if len(netIDs) < 2 {
			continue
		}
		local := hosts[g.localID]
		if local != nil && local.NetworkID > 0 {
			netIDs[local.NetworkID] = true
		}
		ordered := make([]int64, 0, len(netIDs))
		for id := range netIDs {
			ordered = append(ordered, id)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		confidence := minConfidence
		for _, n := range g.remote {
			if n.Confidence < confidence {
				confidence = n.Confidence
			}
		}
		ids := make([]int64, 0, len(g.hosts))
		for id := range g.hosts {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		leak := model.VLANLeak{
			VlanSrc: networkLabel(ordered[0], networks), VlanDst: networkLabel(ordered[1], networks),
			Count: len(ids), SourceMAC: localMAC(local), HostIDs: ids, Port: g.port,
			Confidence: confidence,
			Evidence: []string{
				fmt.Sprintf("El puerto %s de %s alcanza %d redes distintas", g.port, hostLabel(local), len(netIDs)),
				fmt.Sprintf("Vecindad observada por %s con confianza %d%%", firstProtocol(g.remote), confidence),
			},
		}
		out = append(out, leak)
		emitVLANLeak(report, leak, hosts)
	}

	// A bridge/FDB adjacency between two L2 devices on different subnets is
	// also suspicious, even when each port only sees one remote device.
	for _, link := range l2Links(in, minConfidence) {
		a, b := hosts[link.A], hosts[link.B]
		if !isL2Device(a) || !isL2Device(b) || a.NetworkID == 0 || b.NetworkID == 0 || a.NetworkID == b.NetworkID {
			continue
		}
		if !strings.EqualFold(link.Neighbor.Protocol, "bridge") {
			// LLDP/CDP alone can describe a legitimate inter-VLAN uplink.
			continue
		}
		leak := model.VLANLeak{
			VlanSrc: networkLabel(a.NetworkID, networks), VlanDst: networkLabel(b.NetworkID, networks),
			Count: 1, SourceMAC: a.MAC, HostIDs: []int64{a.ID, b.ID}, Port: link.LocalPort,
			Confidence: link.Confidence,
			Evidence: []string{
				fmt.Sprintf("Adyacencia %s entre dos dispositivos L2 de redes distintas", link.Neighbor.Protocol),
				fmt.Sprintf("%s: %s → %s: %s", hostLabel(a), link.LocalPort, hostLabel(b), link.RemotePort),
			},
		}
		out = append(out, leak)
		emitVLANLeak(report, leak, hosts)
	}
	return out
}

func emitVLANLeak(report *model.DiagnosticsReport, leak model.VLANLeak, hosts map[int64]*model.Host) {
	ips := make([]string, 0, len(leak.HostIDs))
	for _, id := range leak.HostIDs {
		if h := hosts[id]; h != nil && h.IP != "" {
			ips = append(ips, h.IP)
		}
	}
	add(report, model.DiagnosticAnomaly{
		ID:   anomalyID("vlan_leak", append([]int64{}, leak.HostIDs...)...),
		Type: "vlan_leak", Category: "vlan", Severity: model.SeverityWarning,
		Title:   "Posible fuga de VLAN",
		Detail:  fmt.Sprintf("El puerto %s conecta %s con %s.", leak.Port, leak.VlanSrc, leak.VlanDst),
		HostIDs: leak.HostIDs, HostIPs: ips, Confidence: leak.Confidence,
		Evidence:       leak.Evidence,
		Recommendation: "Comprueba el etiquetado 802.1Q, el modo trunk/access y la pertenencia a VLAN de esos puertos.",
		DetectedAt:     report.GeneratedAt,
	})
}

func localMAC(h *model.Host) string {
	if h == nil {
		return ""
	}
	return strings.ToLower(h.MAC)
}

func firstProtocol(neighbors map[int64]model.Neighbor) string {
	for _, n := range neighbors {
		if n.Protocol != "" {
			return strings.ToUpper(n.Protocol)
		}
	}
	return "L2"
}
