// Package history builds temporal diffs between two moments of a network.
// Comparison is deliberately evidence-driven: metrics are the primary source
// for device changes, while lightweight topology snapshots are used when they
// exist. Missing snapshots are reported as limitations, never as "no change".
package history

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// MaxChanges bounds the response so a busy network cannot flood the UI.
const MaxChanges = 500

const (
	availabilityThreshold = 1.0 // percentage points
	rttRelativeThreshold  = 0.10
	rttAbsoluteThreshold  = 2.0 // milliseconds
)

var changeOrder = map[string]int{
	"host.added":                 0,
	"host.removed":               1,
	"host.status_changed":        2,
	"host.availability_changed":  3,
	"host.rtt_changed":           4,
	"topology.node_added":        5,
	"topology.node_removed":      6,
	"topology.edge_added":        7,
	"topology.edge_removed":      8,
	"fleet.availability_changed": 9,
	"fleet.rtt_changed":          10,
}

// Compare produces a directional diff from period b (older) to period a.
func Compare(a, b model.HistoryPeriod) model.DiffReport {
	report := model.DiffReport{
		PeriodA: a,
		PeriodB: b,
		Changes: []model.HistoryChange{},
		Summary: map[string]int{},
	}
	if a.Topology.SnapshotAt == nil && b.Topology.SnapshotAt == nil {
		report.Limitations = append(report.Limitations,
			"No hay snapshots de topología en ninguno de los periodos: el diff de topología no está disponible. "+
				"Se capturan automáticamente tras cada escaneo y cada hora.")
	} else if a.Topology.SnapshotAt == nil {
		report.Limitations = append(report.Limitations,
			"No hay snapshot de topología en el periodo A; solo se comparan métricas y dispositivos para ese lado.")
	} else if b.Topology.SnapshotAt == nil {
		report.Limitations = append(report.Limitations,
			"No hay snapshot de topología en el periodo B; solo se comparan métricas y dispositivos para ese lado.")
	}
	if a.Metrics.Samples == 0 || b.Metrics.Samples == 0 {
		report.Limitations = append(report.Limitations,
			"Uno de los periodos no tiene muestras de disponibilidad; los cambios de métricas pueden ser incompletos.")
	}

	mapA, mapB := hostsByKey(a.Hosts), hostsByKey(b.Hosts)
	for key, current := range mapA {
		previous, ok := mapB[key]
		if !ok {
			addChange(&report, model.HistoryChange{
				Type: "host.added", Entity: "host", ID: key, Label: hostLabel(current),
				To: current.Status, Evidence: []string{fmt.Sprintf(
					"%d muestras de disponibilidad en %s", current.Samples, a.Label)},
			})
			continue
		}
		diffHost(&report, key, current, previous, a, b)
	}
	for key, previous := range mapB {
		if _, ok := mapA[key]; !ok {
			addChange(&report, model.HistoryChange{
				Type: "host.removed", Entity: "host", ID: key, Label: hostLabel(previous),
				From: previous.Status, Evidence: []string{fmt.Sprintf(
					"%d muestras de disponibilidad en %s", previous.Samples, b.Label)},
			})
		}
	}

	diffTopology(&report)
	diffFleet(&report)

	sort.SliceStable(report.Changes, func(i, j int) bool {
		ri, okI := changeOrder[report.Changes[i].Type]
		rj, okJ := changeOrder[report.Changes[j].Type]
		if !okI {
			ri = 99
		}
		if !okJ {
			rj = 99
		}
		if ri != rj {
			return ri < rj
		}
		if report.Changes[i].ID != report.Changes[j].ID {
			return report.Changes[i].ID < report.Changes[j].ID
		}
		return report.Changes[i].Type < report.Changes[j].Type
	})
	if len(report.Changes) > MaxChanges {
		report.Changes = report.Changes[:MaxChanges]
		report.Truncated = true
	}
	report.Summary["total"] = len(report.Changes)
	for _, c := range report.Changes {
		report.Summary[c.Type]++
	}
	return report
}

func addChange(r *model.DiffReport, c model.HistoryChange) {
	if c.Evidence == nil {
		c.Evidence = []string{}
	}
	r.Changes = append(r.Changes, c)
}

func diffHost(r *model.DiffReport, key string, a, b model.HistoryHost, pa, pb model.HistoryPeriod) {
	label := hostLabel(a)
	if label == "" {
		label = hostLabel(b)
	}
	if a.Status != b.Status {
		addChange(r, model.HistoryChange{
			Type: "host.status_changed", Entity: "host", ID: key, Label: label,
			From: string(b.Status), To: string(a.Status),
			Evidence: []string{
				fmt.Sprintf("estado en %s: %d muestras (arriba %d, abajo %d)", pb.Label, b.Samples, b.UpSamples, b.DownSamples),
				fmt.Sprintf("estado en %s: %d muestras (arriba %d, abajo %d)", pa.Label, a.Samples, a.UpSamples, a.DownSamples),
			},
		})
	}
	if delta := a.Availability - b.Availability; abs(delta) >= availabilityThreshold && a.Samples > 0 && b.Samples > 0 {
		addChange(r, model.HistoryChange{
			Type: "host.availability_changed", Entity: "host", ID: key, Label: label,
			From: b.Availability, To: a.Availability,
			Evidence: []string{fmt.Sprintf("disponibilidad %s → %s (Δ %s pp, umbral %.1f pp)",
				pct(b.Availability), pct(a.Availability), pct(delta), availabilityThreshold)},
		})
	}
	if a.AvgRTT > 0 && b.AvgRTT > 0 && a.Samples > 0 && b.Samples > 0 {
		delta := a.AvgRTT - b.AvgRTT
		limit := b.AvgRTT * rttRelativeThreshold
		if limit < rttAbsoluteThreshold {
			limit = rttAbsoluteThreshold
		}
		if abs(delta) >= limit {
			addChange(r, model.HistoryChange{
				Type: "host.rtt_changed", Entity: "host", ID: key, Label: label,
				From: b.AvgRTT, To: a.AvgRTT,
				Evidence: []string{fmt.Sprintf("RTT medio %s → %s ms (Δ %s ms, umbral %s ms)",
					num(b.AvgRTT), num(a.AvgRTT), num(delta), num(limit))},
			})
		}
	}
}

func diffTopology(r *model.DiffReport) {
	if r.PeriodA.Topology.SnapshotAt == nil || r.PeriodB.Topology.SnapshotAt == nil {
		return
	}
	nodesA, nodesB := toStringSet(r.PeriodA.Topology.NodeIDs), toStringSet(r.PeriodB.Topology.NodeIDs)
	for _, id := range sortedDifference(nodesA, nodesB) {
		addChange(r, model.HistoryChange{Type: "topology.node_added", Entity: "topology", ID: id,
			Evidence: []string{"Nodo presente en el snapshot del periodo A y ausente en B."}})
	}
	for _, id := range sortedDifference(nodesB, nodesA) {
		addChange(r, model.HistoryChange{Type: "topology.node_removed", Entity: "topology", ID: id,
			Evidence: []string{"Nodo presente en el snapshot del periodo B y ausente en A."}})
	}
	edgesA, edgesB := toStringSet(r.PeriodA.Topology.EdgeIDs), toStringSet(r.PeriodB.Topology.EdgeIDs)
	for _, id := range sortedDifference(edgesA, edgesB) {
		addChange(r, model.HistoryChange{Type: "topology.edge_added", Entity: "topology", ID: id,
			Evidence: []string{"Enlace presente en el snapshot del periodo A y ausente en B."}})
	}
	for _, id := range sortedDifference(edgesB, edgesA) {
		addChange(r, model.HistoryChange{Type: "topology.edge_removed", Entity: "topology", ID: id,
			Evidence: []string{"Enlace presente en el snapshot del periodo B y ausente en A."}})
	}
}

func diffFleet(r *model.DiffReport) {
	a, b := r.PeriodA.Metrics, r.PeriodB.Metrics
	if a.Samples > 0 && b.Samples > 0 {
		if delta := a.Availability - b.Availability; abs(delta) >= availabilityThreshold {
			addChange(r, model.HistoryChange{Type: "fleet.availability_changed", Entity: "fleet", ID: "availability",
				Label: "Disponibilidad global", From: b.Availability, To: a.Availability,
				Evidence: []string{fmt.Sprintf("%s → %s (Δ %s pp)", pct(b.Availability), pct(a.Availability), pct(delta))}})
		}
		if a.AvgRTT > 0 && b.AvgRTT > 0 {
			delta := a.AvgRTT - b.AvgRTT
			limit := b.AvgRTT * rttRelativeThreshold
			if limit < rttAbsoluteThreshold {
				limit = rttAbsoluteThreshold
			}
			if abs(delta) >= limit {
				addChange(r, model.HistoryChange{Type: "fleet.rtt_changed", Entity: "fleet", ID: "avg_rtt",
					Label: "RTT medio global", From: b.AvgRTT, To: a.AvgRTT,
					Evidence: []string{fmt.Sprintf("%s → %s ms (Δ %s ms)", num(b.AvgRTT), num(a.AvgRTT), num(delta))}})
			}
		}
	}
}

func hostsByKey(hosts []model.HistoryHost) map[string]model.HistoryHost {
	out := make(map[string]model.HistoryHost, len(hosts))
	for _, h := range hosts {
		key := hostKey(h)
		if existing, ok := out[key]; ok && existing.Samples >= h.Samples {
			continue
		}
		out[key] = h
	}
	return out
}

func hostKey(h model.HistoryHost) string {
	if h.MAC != "" {
		return "mac:" + strings.ToLower(h.MAC)
	}
	if h.IP != "" {
		return "ip:" + h.IP
	}
	return "id:" + strconv.FormatInt(h.ID, 10)
}

func hostLabel(h model.HistoryHost) string {
	if h.Hostname != "" {
		return h.Hostname + " (" + h.IP + ")"
	}
	return h.IP
}

func toStringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func sortedDifference(a, b map[string]bool) []string {
	out := []string{}
	for key := range a {
		if !b[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// Fingerprint computes a stable identifier for a set of node/edge IDs.
func Fingerprint(nodes, edges []string) string {
	all := append(append([]string{}, nodes...), edges...)
	sort.Strings(all)
	h := uint64(1469598103934665603)
	for _, item := range all {
		for i := 0; i < len(item); i++ {
			h ^= uint64(item[i])
			h *= 1099511628211
		}
		h ^= 0xff
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// DefaultPeriods implements the F21 closure case: today vs seven days ago,
// using two windows of equal length (24 h by default).
func DefaultPeriods(now time.Time, days int, window time.Duration) (model.HistoryPeriod, model.HistoryPeriod) {
	if days <= 0 {
		days = 7
	}
	if window <= 0 || window > 30*24*time.Hour {
		window = 24 * time.Hour
	}
	a := model.HistoryPeriod{Label: "hoy (" + fmtDuration(window) + ")", From: now.Add(-window), To: now}
	bStart := now.Add(-time.Duration(days) * 24 * time.Hour)
	b := model.HistoryPeriod{Label: fmt.Sprintf("hace %d días", days), From: bStart.Add(-window), To: bStart}
	return a, b
}

func fmtDuration(d time.Duration) string {
	if d == 24*time.Hour {
		return "24h"
	}
	if d%(24*time.Hour) == 0 {
		return strconv.Itoa(int(d/(24*time.Hour))) + "d"
	}
	return d.String()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func num(v float64) string { return strconv.FormatFloat(round2(v), 'f', -1, 64) }

func pct(v float64) string { return num(v) + "%" }

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
