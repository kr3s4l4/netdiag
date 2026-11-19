// Package history — deterministic narrative summary of a diff (F57).
//
// Every sentence is built from the structured changes already computed by
// Compare. There is no language model, no inference beyond the counts and no
// claim that a change was caused by anything.
package history

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// buildNarrative turns the structured diff into short, verifiable sentences.
// The order is fixed: identity changes, hosts, topology, fleet, caveats.
func buildNarrative(r *model.DiffReport, now, before map[string]model.HistoryHost) []string {
	out := []string{}
	s := r.Summary

	changedIPs := identityChanges(now, before)
	if len(changedIPs) > 0 {
		out = append(out, fmt.Sprintf("%s cambió de identidad (MAC) en la ventana comparada: %s.",
			countWord(len(changedIPs), "dispositivo", "dispositivos"), joinLimited(changedIPs, 3)))
	}

	if n := s["host.added"] - len(changedIPs); n > 0 {
		labels := labelsForType(r, "host.added", 3)
		out = append(out, fmt.Sprintf("Aparecieron %s nuevos: %s.",
			countWord(n, "dispositivo", "dispositivos"), joinLimited(labels, 3)))
	}
	if n := s["host.removed"] - len(changedIPs); n > 0 {
		labels := labelsForType(r, "host.removed", 3)
		out = append(out, fmt.Sprintf("Desaparecieron %s: %s.",
			countWord(n, "dispositivo", "dispositivos"), joinLimited(labels, 3)))
	}
	if n := s["host.status_changed"]; n > 0 {
		out = append(out, fmt.Sprintf("Cambió el estado de %s entre ambos periodos.",
			countWord(n, "dispositivo", "dispositivos")))
	}
	if n := s["host.availability_changed"]; n > 0 {
		out = append(out, fmt.Sprintf("La disponibilidad de %s superó el umbral de cambio.",
			countWord(n, "dispositivo", "dispositivos")))
	}
	if n := s["host.rtt_changed"]; n > 0 {
		out = append(out, fmt.Sprintf("La latencia de %s superó el umbral de cambio.",
			countWord(n, "dispositivo", "dispositivos")))
	}

	nodeAdded, nodeRemoved := s["topology.node_added"], s["topology.node_removed"]
	edgeAdded, edgeRemoved := s["topology.edge_added"], s["topology.edge_removed"]
	if nodeAdded+nodeRemoved+edgeAdded+edgeRemoved > 0 {
		out = append(out, fmt.Sprintf("La topología ganó %s y %s, y perdió %s y %s.",
			countWord(nodeAdded, "nodo", "nodos"), countWord(edgeAdded, "enlace", "enlaces"),
			countWord(nodeRemoved, "nodo", "nodos"), countWord(edgeRemoved, "enlace", "enlaces")))
	}

	for _, c := range r.Changes {
		switch c.Type {
		case "fleet.availability_changed":
			out = append(out, fmt.Sprintf("La disponibilidad global pasó de %s a %s.", value(c.From), value(c.To)))
		case "fleet.rtt_changed":
			out = append(out, fmt.Sprintf("El RTT medio global pasó de %s ms a %s ms.", value(c.From), value(c.To)))
		}
	}

	if len(out) == 0 {
		out = append(out, "No hay cambios registrados en la ventana comparada.")
	}
	if r.Truncated {
		out = append(out, fmt.Sprintf("El detalle se limitó a los primeros %d cambios; puede haber más.", MaxChanges))
	}
	if len(r.Limitations) > 0 {
		out = append(out, "Alcance del resumen: "+lowerFirst(r.Limitations[0]))
	}
	return out
}

// identityChanges finds IPs that appear with a different MAC in the two
// periods. The host key is MAC-based, so this is the most security-relevant
// difference that a plain added/removed count would hide.
func identityChanges(now, before map[string]model.HistoryHost) []string {
	byIP := func(hosts map[string]model.HistoryHost) map[string]map[string]bool {
		out := map[string]map[string]bool{}
		for _, h := range hosts {
			if h.IP == "" {
				continue
			}
			if out[h.IP] == nil {
				out[h.IP] = map[string]bool{}
			}
			out[h.IP][strings.ToLower(h.MAC)] = true
		}
		return out
	}
	a, b := byIP(now), byIP(before)
	ips := []string{}
	for ip, macsA := range a {
		macsB, ok := b[ip]
		if !ok {
			continue
		}
		shared := false
		for mac := range macsA {
			if macsB[mac] {
				shared = true
				break
			}
		}
		if !shared {
			ips = append(ips, ip)
		}
	}
	sort.Strings(ips)
	return ips
}

func labelsForType(r *model.DiffReport, kind string, limit int) []string {
	labels := []string{}
	for _, c := range r.Changes {
		if c.Type != kind {
			continue
		}
		if c.Label != "" {
			labels = append(labels, c.Label)
		}
		if len(labels) >= limit {
			break
		}
	}
	return labels
}

func countWord(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func joinLimited(items []string, limit int) string {
	if len(items) == 0 {
		return "—"
	}
	clean := append([]string(nil), items...)
	if len(clean) > limit {
		clean = append(clean[:limit], fmt.Sprintf("y %d más", len(clean)-limit))
	}
	return strings.Join(clean, ", ")
}

func value(v any) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case float64:
		return num(x)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
