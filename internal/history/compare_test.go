package history

import (
	"fmt"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func period(label string, hosts []model.HistoryHost, metrics model.HistoryAggregate) model.HistoryPeriod {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	return model.HistoryPeriod{Label: label, From: now.Add(-24 * time.Hour), To: now, Hosts: hosts, Metrics: metrics}
}

func changeTypes(report model.DiffReport) map[string]int {
	out := map[string]int{}
	for _, c := range report.Changes {
		out[c.Type]++
	}
	return out
}

func TestCompareDetectsDeviceAndFleetChanges(t *testing.T) {
	oldHosts := []model.HistoryHost{
		{ID: 1, IP: "10.0.0.1", Hostname: "router", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusUp,
			Samples: 20, UpSamples: 20, Availability: 100, AvgRTT: 4},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", Status: model.StatusUp,
			Samples: 20, UpSamples: 18, DownSamples: 2, Availability: 90, AvgRTT: 20},
		{ID: 3, IP: "10.0.0.3", Hostname: "old", MAC: "cc:cc:cc:cc:cc:cc", Status: model.StatusUp,
			Samples: 20, UpSamples: 20, Availability: 100, AvgRTT: 8},
	}
	newHosts := []model.HistoryHost{
		{ID: 1, IP: "10.0.0.1", Hostname: "router", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusUp,
			Samples: 20, UpSamples: 20, Availability: 100, AvgRTT: 4},
		{ID: 2, IP: "10.0.0.2", MAC: "bb:bb:bb:bb:bb:bb", Status: model.StatusDown,
			Samples: 20, UpSamples: 2, DownSamples: 18, Availability: 10, AvgRTT: 45},
		{ID: 4, IP: "10.0.0.4", Hostname: "new", MAC: "dd:dd:dd:dd:dd:dd", Status: model.StatusUp,
			Samples: 20, UpSamples: 20, Availability: 100, AvgRTT: 6},
	}
	a := period("hoy", newHosts, model.HistoryAggregate{Samples: 60, Availability: 70, AvgRTT: 18, HostsUp: 2, HostsDown: 1})
	b := period("hace 7 días", oldHosts, model.HistoryAggregate{Samples: 60, Availability: 97, AvgRTT: 11, HostsUp: 3})

	report := Compare(a, b)
	types := changeTypes(report)
	for _, kind := range []string{"host.added", "host.removed", "host.status_changed", "host.availability_changed", "host.rtt_changed", "fleet.availability_changed"} {
		if types[kind] == 0 {
			t.Errorf("falta el cambio %s en %+v", kind, types)
		}
	}
	if report.Summary["total"] != len(report.Changes) {
		t.Fatalf("resumen incoherente: %+v", report.Summary)
	}
	if len(report.Limitations) == 0 {
		t.Fatal("faltan limitaciones por ausencia de snapshots")
	}
	for _, c := range report.Changes {
		if len(c.Evidence) == 0 {
			t.Fatalf("cambio sin evidencia: %+v", c)
		}
	}
}

func TestCompareTopologyWhenSnapshotsExist(t *testing.T) {
	ts := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	a := period("hoy", nil, model.HistoryAggregate{Samples: 10, Availability: 100})
	b := period("hace 7 días", nil, model.HistoryAggregate{Samples: 10, Availability: 100})
	a.Topology = model.HistoryTopology{
		Nodes: 2, Edges: 1, NodeIDs: []string{"host:1", "host:2"}, EdgeIDs: []string{"e1"}, SnapshotAt: &ts,
	}
	b.Topology = model.HistoryTopology{
		Nodes: 2, Edges: 1, NodeIDs: []string{"host:1", "host:3"}, EdgeIDs: []string{"e2"}, SnapshotAt: &ts,
	}
	report := Compare(a, b)
	types := changeTypes(report)
	if types["topology.node_added"] != 1 || types["topology.node_removed"] != 1 ||
		types["topology.edge_added"] != 1 || types["topology.edge_removed"] != 1 {
		t.Fatalf("diff de topología incorrecto: %+v", types)
	}
	if len(report.Limitations) != 0 {
		t.Fatalf("no debería haber limitaciones: %+v", report.Limitations)
	}
}

func TestCompareIgnoresSmallNoise(t *testing.T) {
	h := model.HistoryHost{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusUp,
		Samples: 20, UpSamples: 20, Availability: 99.5, AvgRTT: 10}
	a := period("hoy", []model.HistoryHost{h}, model.HistoryAggregate{Samples: 20, Availability: 99.5, AvgRTT: 10})
	b := period("ayer", []model.HistoryHost{h}, model.HistoryAggregate{Samples: 20, Availability: 99.8, AvgRTT: 10.4})
	report := Compare(a, b)
	if len(report.Changes) != 0 {
		t.Fatalf("ruido por debajo del umbral generó cambios: %+v", report.Changes)
	}
}

func TestCompareTruncatesLargeDiffs(t *testing.T) {
	hostsA, hostsB := []model.HistoryHost{}, []model.HistoryHost{}
	for i := 0; i < MaxChanges+25; i++ {
		mac := fmt.Sprintf("00:00:00:%02x:%02x:%02x", i>>16, (i>>8)&0xff, i&0xff)
		hostsA = append(hostsA, model.HistoryHost{ID: int64(i + 1), IP: "10.1.0.1", MAC: mac, Status: model.StatusUp})
	}
	report := Compare(period("hoy", hostsA, model.HistoryAggregate{}), period("antes", hostsB, model.HistoryAggregate{}))
	if !report.Truncated || len(report.Changes) != MaxChanges {
		t.Fatalf("esperaba truncamiento a %d, obtenido %d (truncated=%v)", MaxChanges, len(report.Changes), report.Truncated)
	}
}

func TestDefaultPeriodsTodayVsSevenDaysAgo(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	a, b := DefaultPeriods(now, 7, 24*time.Hour)
	if got := a.To.Sub(a.From); got != 24*time.Hour {
		t.Fatalf("ventana A: %v", got)
	}
	if got := b.To.Sub(b.From); got != 24*time.Hour {
		t.Fatalf("ventana B: %v", got)
	}
	if got := a.From.Sub(b.To); got != 6*24*time.Hour {
		t.Fatalf("hueco entre ventanas: %v", got)
	}
	if b.Label != "hace 7 días" {
		t.Fatalf("etiqueta B: %q", b.Label)
	}
}

func TestFingerprintIsStableAndOrderIndependent(t *testing.T) {
	a := Fingerprint([]string{"n2", "n1"}, []string{"e2", "e1"})
	b := Fingerprint([]string{"e1", "n1", "e2", "n2"}, nil)
	if a != b || len(a) != 16 {
		t.Fatalf("huellas inestables: %q %q", a, b)
	}
}
