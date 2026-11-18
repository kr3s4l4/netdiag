package store

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestHistoryPeriodAggregatesMetricsAndSnapshot(t *testing.T) {
	st := newTestStore(t)
	id, _, _, _, err := st.UpsertHost(&model.Host{IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", SiteID: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old := now.Add(-7 * 24 * time.Hour)

	add := func(at time.Time, rtt float64, up bool) {
		if err := st.AddMetric(id, rtt, up, 0, at); err != nil {
			t.Fatal(err)
		}
	}
	add(old.Add(-time.Hour), 12, true)
	add(old.Add(-30*time.Minute), 16, true)
	add(old.Add(-10*time.Minute), 0, false)
	add(now.Add(-time.Hour), 4, true)
	add(now.Add(-10*time.Minute), 6, true)

	snapTS := old.Add(-5 * time.Minute).Truncate(time.Millisecond)
	snap := &model.HistorySnapshot{
		SiteID: 1, TS: snapTS,
		Hosts:    []model.HistoryHost{{ID: id, IP: "10.0.0.1", Status: model.StatusUp}},
		Topology: model.HistoryTopology{Nodes: 2, Edges: 1, NodeIDs: []string{"n1", "n2"}, EdgeIDs: []string{"e1"}, Fingerprint: "abc"},
	}
	if err := st.SaveHistorySnapshot(snap); err != nil {
		t.Fatal(err)
	}
	if snap.ID == 0 {
		t.Fatal("snapshot sin id")
	}

	oldPeriod := model.HistoryPeriod{Label: "hace 7 días", From: old.Add(-24 * time.Hour), To: old}
	built, err := st.BuildHistoryPeriod(1, oldPeriod)
	if err != nil {
		t.Fatal(err)
	}
	if built.Metrics.Samples != 3 || built.Metrics.Availability != 66.67 || built.Metrics.AvgRTT != 14 {
		t.Fatalf("agregados incorrectos: %+v", built.Metrics)
	}
	if built.Metrics.HostsUp != 1 || built.Metrics.HostsDown != 1 {
		t.Fatalf("conteo de hosts por estado incorrecto: %+v", built.Metrics)
	}
	if len(built.Hosts) != 1 || built.Hosts[0].Samples != 3 || built.Hosts[0].DownSamples != 1 {
		t.Fatalf("agregado por host incorrecto: %+v", built.Hosts)
	}
	if built.Topology.SnapshotAt == nil || !built.Topology.SnapshotAt.Equal(snapTS) {
		t.Fatalf("snapshot no asociado a la ventana: %+v", built.Topology)
	}
	if built.Topology.Fingerprint != "abc" || len(built.Topology.NodeIDs) != 2 {
		t.Fatalf("topología del snapshot incorrecta: %+v", built.Topology)
	}

	today := model.HistoryPeriod{Label: "hoy", From: now.Add(-24 * time.Hour), To: now}
	builtToday, err := st.BuildHistoryPeriod(1, today)
	if err != nil {
		t.Fatal(err)
	}
	if builtToday.Metrics.Samples != 2 || builtToday.Metrics.AvgRTT != 5 {
		t.Fatalf("agregados de hoy incorrectos: %+v", builtToday.Metrics)
	}
	if builtToday.Topology.SnapshotAt != nil {
		t.Fatal("se reutilizó un snapshot fuera de la ventana")
	}
}

func TestHistorySnapshotIsolationBySiteAndPurge(t *testing.T) {
	st := newTestStore(t)
	for _, siteID := range []int64{1, 2} {
		snap := &model.HistorySnapshot{SiteID: siteID, TS: time.Now().Add(-40 * 24 * time.Hour)}
		if err := st.SaveHistorySnapshot(snap); err != nil {
			t.Fatal(err)
		}
	}
	recent := &model.HistorySnapshot{SiteID: 1, TS: time.Now()}
	if err := st.SaveHistorySnapshot(recent); err != nil {
		t.Fatal(err)
	}
	found, err := st.HistorySnapshotIn(1, time.Now().Add(-24*time.Hour), time.Now().Add(time.Hour))
	if err != nil || found == nil || found.ID != recent.ID {
		t.Fatalf("snapshot reciente no encontrado: %v %+v", err, found)
	}
	// site_id=0 agrega todas las sedes, igual que el resto de la API.
	anySite, err := st.HistorySnapshotIn(0, time.Now().Add(-24*time.Hour), time.Now().Add(time.Hour))
	if err != nil || anySite == nil || anySite.ID != recent.ID {
		t.Fatalf("snapshot global no encontrado: %v %+v", err, anySite)
	}
	if err := st.PurgeHistorySnapshots(30); err != nil {
		t.Fatal(err)
	}
	found, err = st.HistorySnapshotIn(1, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	if err != nil || found != nil {
		t.Fatalf("snapshot antiguo no purgado: %v %+v", err, found)
	}
}
