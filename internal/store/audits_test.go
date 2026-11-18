package store

import (
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestAuditSnapshotsRoundTrip(t *testing.T) {
	st := newTestStore(t)
	hosts := make([]model.AuditHost, 0, 200)
	for i := 0; i < 200; i++ {
		hosts = append(hosts, model.AuditHost{
			Identity: "s1:host-" + strings.Repeat("x", 20), IP: "10.0.0.1",
			Hostname: "dispositivo-con-un-nombre-largo", Tags: []string{"produccion", "critical"},
		})
	}
	snap := &model.AuditSnapshot{
		SiteID: 1, Name: "Base", Notes: "antes de cambios",
		Data: &model.AuditSnapshotData{Schema: "netdiag/audit/v1", Hosts: hosts},
	}
	if err := st.CreateAuditSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	if snap.ID == 0 || snap.Size <= 0 || snap.RawSize <= 0 {
		t.Fatalf("metadatos incompletos: %+v", snap)
	}
	if snap.Size >= snap.RawSize {
		t.Fatalf("gzip no comprimió: raw=%d stored=%d", snap.RawSize, snap.Size)
	}
	if snap.Name != "Base" {
		t.Fatalf("nombre alterado: %q", snap.Name)
	}

	items, err := st.ListAuditSnapshots(1, 50)
	if err != nil || len(items) != 1 {
		t.Fatalf("listado: %v %+v", err, items)
	}
	if items[0].Data != nil || items[0].Size != snap.Size {
		t.Fatalf("el listado no debe incluir el payload: %+v", items[0])
	}

	meta, err := st.GetAuditSnapshot(snap.ID, false)
	if err != nil || meta == nil || meta.Data != nil {
		t.Fatalf("detalle sin datos: %v %+v", err, meta)
	}
	full, err := st.GetAuditSnapshot(snap.ID, true)
	if err != nil || full == nil || full.Data == nil {
		t.Fatalf("detalle con datos: %v %+v", err, full)
	}
	if full.Data.Schema != "netdiag/audit/v1" || len(full.Data.Hosts) != 200 {
		t.Fatalf("payload corrupto: %+v", full.Data)
	}

	if err := st.DeleteAuditSnapshot(snap.ID); err != nil {
		t.Fatal(err)
	}
	missing, err := st.GetAuditSnapshot(snap.ID, true)
	if err != nil || missing != nil {
		t.Fatalf("snapshot no eliminado: %v %+v", err, missing)
	}
}

func TestAuditSnapshotsDefaultNameAndSiteIsolation(t *testing.T) {
	st := newTestStore(t)
	a := &model.AuditSnapshot{Data: &model.AuditSnapshotData{Schema: "v1"}}
	b := &model.AuditSnapshot{SiteID: 2, Data: &model.AuditSnapshotData{Schema: "v1"}}
	if err := st.CreateAuditSnapshot(a); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAuditSnapshot(b); err != nil {
		t.Fatal(err)
	}
	if a.SiteID != 1 || !strings.HasPrefix(a.Name, "Auditoría ") {
		t.Fatalf("valores por defecto incorrectos: %+v", a)
	}
	site1, err := st.ListAuditSnapshots(1, 50)
	if err != nil || len(site1) != 1 || site1[0].ID != a.ID {
		t.Fatalf("aislamiento por sede: %v %+v", err, site1)
	}
	all, err := st.ListAuditSnapshots(0, 50)
	if err != nil || len(all) != 2 {
		t.Fatalf("listado global: %v %+v", err, all)
	}
}
