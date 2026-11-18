package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// l2Link is a resolved adjacency between two inventoried hosts.
type l2Link struct {
	A          int64
	B          int64
	Neighbor   model.Neighbor
	Confidence int
	LocalPort  string
	RemotePort string
}

// l2Links resolves the configured neighbors to inventory hosts. Unresolved
// neighbors are ignored: without both endpoints there is no graph to analyze.
func l2Links(in Input, minConfidence int) []l2Link {
	byMAC, byIP, byName := neighborIndexes(in.Hosts)
	var out []l2Link
	seen := map[string]bool{}
	for localID, neighbors := range in.Neighbors {
		for _, n := range neighbors {
			if !isTrustedProtocol(n) || n.Confidence < minConfidence {
				continue
			}
			remoteID := resolveNeighbor(n, byMAC, byIP, byName)
			if remoteID == 0 || remoteID == localID {
				continue
			}
			key := fmt.Sprintf("%d-%d-%s-%s", localID, remoteID,
				strings.ToLower(n.LocalPort), strings.ToLower(n.RemotePort))
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, l2Link{
				A: localID, B: remoteID, Neighbor: n, Confidence: n.Confidence,
				LocalPort: n.LocalPort, RemotePort: n.RemotePort,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].A != out[j].A {
			return out[i].A < out[j].A
		}
		return out[i].B < out[j].B
	})
	return out
}

// detectSTPLoops looks for cycles in the high-confidence L2 graph. A cycle of
// three or more distinct devices is a strong sign of a spanning-tree problem;
// requiring several devices and trusted protocols avoids single-link flapping
// false positives.
func detectSTPLoops(in Input, report *model.DiagnosticsReport) []model.STPLoop {
	if !in.Config.EnterpriseMode {
		return nil
	}
	links := l2Links(in, in.Config.MinConfidence)
	hosts := indexHosts(in.Hosts)
	forest := map[int64][]int64{}
	edgeConf := map[string]int{}
	var cycles []model.STPLoop

	pathBetween := func(from, to int64) []int64 {
		if from == to {
			return []int64{from}
		}
		prev := map[int64]int64{from: 0}
		queue := []int64{from}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, next := range forest[cur] {
				if _, ok := prev[next]; ok {
					continue
				}
				prev[next] = cur
				if next == to {
					path := []int64{next}
					for p := prev[next]; p != 0; p = prev[p] {
						path = append(path, p)
					}
					return path
				}
				queue = append(queue, next)
			}
		}
		return nil
	}

	for _, link := range links {
		path := pathBetween(link.A, link.B)
		if len(path) < 3 {
			// No cycle yet; keep this edge in the spanning forest.
			forest[link.A] = append(forest[link.A], link.B)
			forest[link.B] = append(forest[link.B], link.A)
			edgeConf[edgeKey(link.A, link.B)] = link.Confidence
			continue
		}
		// pathBetween already starts at link.A and ends at link.B, so the
		// closing edge is the current link back to link.A.
		ids := append([]int64{}, path...)
		confidence := link.Confidence
		for i := 0; i < len(path)-1; i++ {
			if c := edgeConf[edgeKey(path[i], path[i+1])]; c < confidence {
				confidence = c
			}
		}
		if confidence < in.Config.MinConfidence {
			continue
		}
		bridges, ports, evidence := []string{}, []string{}, []string{}
		for _, id := range ids {
			h := hosts[id]
			label := hostLabel(h)
			bridges = append(bridges, label)
			if h != nil && h.MAC != "" {
				bridges[len(bridges)-1] = strings.ToLower(h.MAC) + " (" + label + ")"
			}
		}
		ports = append(ports, link.LocalPort, link.RemotePort)
		for _, p := range ports {
			if p != "" {
				evidence = append(evidence, "Puerto implicado: "+p)
			}
		}
		evidence = append(evidence,
			fmt.Sprintf("Ciclo L2 con %d dispositivos detectado mediante %s", len(ids), link.Neighbor.Protocol),
			fmt.Sprintf("Confianza mínima de la vecindad: %d%%", confidence))
		loop := model.STPLoop{
			BridgeIDs: bridges, AffectedPorts: dedupe(ports),
			Rate:       round1(float64(len(ids)) / float64(len(in.Hosts)+1)),
			Confidence: confidence, Evidence: dedupe(evidence),
		}
		cycles = append(cycles, loop)
		severity := model.SeverityWarning
		if confidence >= 90 {
			severity = model.SeverityCritical
		}
		add(report, model.DiagnosticAnomaly{
			ID: anomalyID("stp_loop", ids...), Type: "stp_loop", Category: "stp",
			Severity: severity, Title: "Posible lazo de spanning-tree",
			Detail:  fmt.Sprintf("Se ha cerrado un ciclo L2 entre %d dispositivos; el tráfico puede estar formando un bucle.", len(ids)),
			HostIDs: ids, Confidence: confidence, Evidence: loop.Evidence,
			Recommendation: "Revisa los puertos implicados, prioridades de root bridge y el estado STP en los switches.",
			DetectedAt:     report.GeneratedAt,
		})
		if len(cycles) >= 20 {
			break
		}
	}
	return cycles
}

func edgeKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d-%d", a, b)
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
