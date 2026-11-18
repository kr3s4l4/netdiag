package audits

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func snapshot(name string, data *model.AuditSnapshotData) *model.AuditSnapshot {
	return &model.AuditSnapshot{ID: 1, Name: name, TS: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), Data: data}
}

func types(d *model.AuditDiff) map[string]int {
	out := map[string]int{}
	for _, c := range d.Changes {
		out[c.Type]++
	}
	return out
}

func TestDiffDetectsInventoryConfigurationAndTopologyChanges(t *testing.T) {
	from := snapshot("Base", &model.AuditSnapshotData{
		Hosts: []model.AuditHost{{
			Identity: "s1:aa", IP: "10.0.0.1", MAC: "aa", Hostname: "router",
			Status: model.StatusUp, DeviceType: model.DevRouter,
			Ports: []model.Port{{Number: 22, Protocol: "tcp", State: "open", Service: "ssh"}},
		}},
		Networks:     []model.AuditNetwork{{ID: 1, Name: "LAN", CIDR: "10.0.0.0/24", Enabled: true}},
		Rules:        []model.AuditRule{{ID: 1, Name: "caída", Type: "host_down", Enabled: true}},
		Topology:     model.AuditTopologyData{Nodes: []model.GraphNode{{ID: "host:1", Label: "router"}}},
		Alerts:       model.AuditAlertSummary{Active: 2, Critical: 1},
		Config:       model.AuditConfigSummary{MonitorEnabled: true, MetricsDays: 30},
		Integrations: []model.AuditIntegration{{ID: 1, Name: "syslog", Type: "syslog", Enabled: true}},
	})
	to := snapshot("Después", &model.AuditSnapshotData{
		Hosts: []model.AuditHost{
			{
				Identity: "s1:aa", IP: "10.0.0.2", MAC: "aa", Hostname: "router-core",
				Status: model.StatusDown, DeviceType: model.DevRouter,
				Ports: []model.Port{{Number: 443, Protocol: "tcp", State: "open", Service: "https"}},
			},
			{Identity: "s1:bb", IP: "10.0.0.5", MAC: "bb", Hostname: "nuevo", Status: model.StatusUp},
		},
		Networks:     []model.AuditNetwork{{ID: 1, Name: "LAN", CIDR: "10.0.0.0/23", Enabled: true}},
		Rules:        []model.AuditRule{{ID: 1, Name: "caída", Type: "host_down", Enabled: false}},
		Topology:     model.AuditTopologyData{Nodes: []model.GraphNode{{ID: "host:2", Label: "nuevo"}}},
		Alerts:       model.AuditAlertSummary{Active: 5, Critical: 0},
		Config:       model.AuditConfigSummary{MonitorEnabled: false, MetricsDays: 30},
		Integrations: []model.AuditIntegration{{ID: 1, Name: "syslog", Type: "syslog", Enabled: true}},
	})

	d := Diff(from, to)
	got := types(d)
	for _, kind := range []string{
		"host.added", "host.changed", "host.port_added", "host.port_removed",
		"network.changed", "rule.changed", "topology.node_added", "topology.node_removed",
		"alerts.changed", "config.changed",
	} {
		if got[kind] == 0 {
			t.Errorf("falta %s en %+v", kind, got)
		}
	}
	if d.Summary["total"] != len(d.Changes) || len(d.Changes) == 0 {
		t.Fatalf("resumen incoherente: %+v", d.Summary)
	}
	for _, c := range d.Changes {
		if len(c.Evidence) == 0 {
			t.Fatalf("cambio sin evidencia: %+v", c)
		}
	}
}

func TestDiffIgnoresVolatileFieldsAndVolatileMetrics(t *testing.T) {
	data := func(last time.Time) *model.AuditSnapshotData {
		return &model.AuditSnapshotData{
			Hosts: []model.AuditHost{{
				Identity: "s1:aa", IP: "10.0.0.1", MAC: "aa", Status: model.StatusUp,
				LastSeen: last, FirstSeen: last,
			}},
		}
	}
	from := snapshot("A", data(time.Now().Add(-time.Hour)))
	to := snapshot("B", data(time.Now()))
	d := Diff(from, to)
	if len(d.Changes) != 0 {
		t.Fatalf("se compararon campos volátiles: %+v", d.Changes)
	}
}

func TestDiffHandlesMissingData(t *testing.T) {
	d := Diff(snapshot("A", nil), snapshot("B", &model.AuditSnapshotData{}))
	if len(d.Limitations) == 0 || len(d.Changes) != 0 {
		t.Fatalf("limitaciones ausentes: %+v", d)
	}
}
