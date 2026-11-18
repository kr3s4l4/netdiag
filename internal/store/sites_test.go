package store

import (
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestDefaultSiteAndCRUD(t *testing.T) {
	st := newTestStore(t)
	sites, err := st.ListSites()
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].ID != 1 || sites[0].Name != "Principal" {
		t.Fatalf("sede por defecto inesperada: %+v", sites)
	}

	created, err := st.CreateSite("Oficina Norte", "Planta 2")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 1 || created.Slug != "oficina-norte" {
		t.Fatalf("sede creada inesperada: %+v", created)
	}
	if !st.SiteExists(created.ID) {
		t.Fatal("SiteExists devolvió falso para una sede existente")
	}

	renamed, err := st.UpdateSite(created.ID, "Oficina Norte 2", "Planta 3")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Oficina Norte 2" || renamed.Description != "Planta 3" {
		t.Fatalf("sede actualizada inesperada: %+v", renamed)
	}
	if _, err := st.UpdateSite(0, "Nope", ""); err != ErrNotFound {
		t.Fatalf("se esperaba ErrNotFound, se obtuvo %v", err)
	}
}

func TestSitesSlugifyUniquenessAndValidation(t *testing.T) {
	st := newTestStore(t)
	a, err := st.CreateSite("Sede Ágil Oeste", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.CreateSite("Sede Agil Oeste", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug == b.Slug {
		t.Fatalf("slugs duplicados: %q", a.Slug)
	}
	if !strings.HasPrefix(b.Slug, "sede-agil-oeste-") {
		t.Fatalf("slug inesperado: %q", b.Slug)
	}
	if _, err := st.CreateSite("   ", ""); err == nil {
		t.Fatal("se esperaba error con nombre vacío")
	}
}

func TestLastSiteCannotBeDeleted(t *testing.T) {
	st := newTestStore(t)
	if err := st.DeleteSite(1, 0); err == nil {
		t.Fatal("se pudo eliminar la única sede")
	}
	second, err := st.CreateSite("Segunda", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSite(1, second.ID); err != nil {
		t.Fatalf("no se pudo eliminar la primera sede: %v", err)
	}
	sites, _ := st.ListSites()
	if len(sites) != 1 || sites[0].ID != second.ID {
		t.Fatalf("estado de sedes inesperado: %+v", sites)
	}
}

func TestNetworkCIDRReuseAcrossSites(t *testing.T) {
	st := newTestStore(t)
	other, err := st.CreateSite("Sucursal", "")
	if err != nil {
		t.Fatal(err)
	}
	const cidr = "192.168.50.0/24"
	n1, _, err := st.EnsureNetwork(&model.Network{SiteID: 1, Name: "LAN central", CIDR: cidr, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	n2, _, err := st.EnsureNetwork(&model.Network{SiteID: other.ID, Name: "LAN sucursal", CIDR: cidr, Enabled: true})
	if err != nil {
		t.Fatalf("el mismo CIDR debería poderse registrar en otra sede: %v", err)
	}
	if n1.ID == n2.ID || n1.SiteID != 1 || n2.SiteID != other.ID {
		t.Fatalf("redes inesperadas: %+v %+v", n1, n2)
	}
	// Re-adding inside the same site must return the existing network.
	again, created, err := st.EnsureNetwork(&model.Network{SiteID: other.ID, Name: "otro", CIDR: cidr})
	if err != nil || created || again.ID != n2.ID {
		t.Fatalf("EnsureNetwork duplicado: %+v created=%v err=%v", again, created, err)
	}
}

func TestHostIdentityIsScopedBySite(t *testing.T) {
	st := newTestStore(t)
	other, err := st.CreateSite("Sucursal", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	id1, created1, _, _, err := st.UpsertHost(&model.Host{
		SiteID: 1, IP: "10.9.0.5", MAC: "aa:bb:cc:11:22:33",
		Status: model.StatusUp, FirstSeen: now, LastSeen: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	id2, created2, _, _, err := st.UpsertHost(&model.Host{
		SiteID: other.ID, IP: "10.9.0.5", MAC: "aa:bb:cc:11:22:33",
		Status: model.StatusUp, FirstSeen: now, LastSeen: now,
	})
	if err != nil {
		t.Fatalf("el mismo host en otra sede debería crear otro registro: %v", err)
	}
	if !created1 || !created2 || id1 == id2 {
		t.Fatalf("identidades colisionadas: id1=%d created1=%v id2=%d created2=%v", id1, created1, id2, created2)
	}
	h1, _ := st.GetHost(id1)
	h2, _ := st.GetHost(id2)
	if h1.Identity == h2.Identity || h1.SiteID != 1 || h2.SiteID != other.ID {
		t.Fatalf("hosts mal separados: %+v %+v", h1, h2)
	}
}

func TestSiteScopedFiltersAndDashboard(t *testing.T) {
	st := newTestStore(t)
	other, err := st.CreateSite("Sucursal", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// Hosts.
	h1, _, _, _, err := st.UpsertHost(&model.Host{SiteID: 1, IP: "172.20.0.2", Status: model.StatusUp, LastSeen: now})
	if err != nil {
		t.Fatal(err)
	}
	h2, _, _, _, err := st.UpsertHost(&model.Host{SiteID: other.ID, IP: "172.20.0.3", Status: model.StatusDown, LastSeen: now})
	if err != nil {
		t.Fatal(err)
	}
	// Networks.
	n1, _, err := st.EnsureNetwork(&model.Network{SiteID: 1, Name: "N1", CIDR: "172.20.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	n2, _, err := st.EnsureNetwork(&model.Network{SiteID: other.ID, Name: "N2", CIDR: "172.21.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	// Scans.
	s1, err := st.CreateScan(&model.Scan{SiteID: 1, NetworkID: n1.ID, CIDR: n1.CIDR, Trigger: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateScan(&model.Scan{SiteID: other.ID, NetworkID: n2.ID, CIDR: n2.CIDR, Trigger: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	// Alerts.
	_, _, err = st.CreateAlert(&model.Alert{HostID: h1, HostIP: "172.20.0.2", Title: "Alerta central", Severity: model.SeverityWarning}, "test-central")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = st.CreateAlert(&model.Alert{HostID: h2, HostIP: "172.20.0.3", Title: "Alerta sucursal", Severity: model.SeverityCritical}, "test-sucursal")
	if err != nil {
		t.Fatal(err)
	}
	// Reports.
	_, err = st.AddReport(&model.Report{SiteID: 1, Name: "central.pdf", Format: "pdf", Kind: "inventory", Path: "/tmp/central.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.AddReport(&model.Report{SiteID: other.ID, Name: "sucursal.pdf", Format: "pdf", Kind: "inventory", Path: "/tmp/sucursal.pdf"})
	if err != nil {
		t.Fatal(err)
	}

	hosts, err := st.ListHosts(model.HostFilter{SiteID: other.ID, Page: 1, Limit: 10})
	if err != nil || hosts.Total != 1 || hosts.Items[0].ID != h2 {
		t.Fatalf("filtro de hosts por sede falló: %+v err=%v", hosts, err)
	}
	nets, err := st.ListNetworksBySite(other.ID)
	if err != nil || len(nets) != 1 || nets[0].ID != n2.ID {
		t.Fatalf("filtro de redes por sede falló: %+v err=%v", nets, err)
	}
	scans, err := st.ListScans(model.ScanFilter{SiteID: 1, Page: 1, Limit: 10})
	if err != nil || scans.Total != 1 || scans.Items[0].ID != s1.ID {
		t.Fatalf("filtro de escaneos por sede falló: %+v err=%v", scans, err)
	}
	active1, _ := st.ActiveScansForSite(1)
	active2, _ := st.ActiveScansForSite(other.ID)
	if len(active1) != 1 || len(active2) != 1 || active1[0].ID == active2[0].ID {
		t.Fatalf("ActiveScansForSite falló: %+v %+v", active1, active2)
	}
	alerts, err := st.ListAlerts(model.AlertFilter{SiteID: other.ID, Page: 1, Limit: 10})
	if err != nil || alerts.Total != 1 || alerts.Items[0].Title != "Alerta sucursal" {
		t.Fatalf("filtro de alertas por sede falló: %+v err=%v", alerts, err)
	}
	astats, err := st.AlertStatsForSite(other.ID)
	if err != nil || astats.Critical != 1 {
		t.Fatalf("estadísticas de alertas por sede fallaron: %+v err=%v", astats, err)
	}
	reports, err := st.ListReports(other.ID)
	if err != nil || len(reports) != 1 || reports[0].SiteID != other.ID {
		t.Fatalf("filtro de informes por sede falló: %+v err=%v", reports, err)
	}
	stats, err := st.Dashboard(1)
	if err != nil || stats.HostsTotal != 1 || stats.Networks != 1 || stats.AlertsActive != 1 {
		t.Fatalf("panel de la sede 1 falló: %+v err=%v", stats, err)
	}
	stats2, err := st.Dashboard(other.ID)
	if err != nil || stats2.HostsTotal != 1 || stats2.Networks != 1 || stats2.AlertsCritical != 1 {
		t.Fatalf("panel de la sede %d falló: %+v err=%v", other.ID, stats2, err)
	}
	all, err := st.Dashboard(0)
	if err != nil || all.HostsTotal != 2 || all.Networks != 2 {
		t.Fatalf("panel global falló: %+v err=%v", all, err)
	}
}

func TestSiteExportImportMergeAndReplace(t *testing.T) {
	st := newTestStore(t)
	source, err := st.CreateSite("Origen", "Sede de prueba")
	if err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateSite("Destino", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	net, _, err := st.EnsureNetwork(&model.Network{SiteID: source.ID, Name: "LAN", CIDR: "10.30.0.0/24", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	hostID, _, _, _, err := st.UpsertHost(&model.Host{
		SiteID: source.ID, NetworkID: net.ID, IP: "10.30.0.10", MAC: "02:00:00:00:00:10",
		Hostname: "servidor", Status: model.StatusUp, FirstSeen: now, LastSeen: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	notes, tags := "nota original", []string{"prod"}
	if err := st.UpdateHostMeta(hostID, &notes, &tags, nil); err != nil {
		t.Fatal(err)
	}

	bundle, err := st.ExportSite(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Schema != "netdiag.site/v1" || len(bundle.Networks) != 1 || len(bundle.Hosts) != 1 {
		t.Fatalf("paquete inesperado: %+v", bundle)
	}
	if bundle.Hosts[0].Identity != "02:00:00:00:00:10" {
		t.Fatalf("la identidad exportada no es portable: %q", bundle.Hosts[0].Identity)
	}

	// Merge into target.
	summary, err := st.ImportSite(target.ID, bundle, false)
	if err != nil {
		t.Fatal(err)
	}
	if summary.NetworksCreated != 1 || summary.HostsCreated != 1 {
		t.Fatalf("resumen de importación inesperado: %+v", summary)
	}
	nets, _ := st.ListNetworksBySite(target.ID)
	hosts, _ := st.ListHostsForExport(target.ID)
	if len(nets) != 1 || len(hosts) != 1 || hosts[0].Notes != "nota original" {
		t.Fatalf("importación merge falló: nets=%+v hosts=%+v", nets, hosts)
	}

	// Second merge must be idempotent.
	summary, err = st.ImportSite(target.ID, bundle, false)
	if err != nil || summary.NetworksCreated != 0 || summary.HostsCreated != 0 || summary.HostsUpdated != 1 {
		t.Fatalf("merge no idempotente: %+v err=%v", summary, err)
	}

	// A replace clears target data before importing.
	local, _, err := st.EnsureNetwork(&model.Network{SiteID: target.ID, Name: "Local", CIDR: "10.99.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = st.UpsertHost(&model.Host{SiteID: target.ID, NetworkID: local.ID, IP: "10.99.0.1", Status: model.StatusUp})
	if err != nil {
		t.Fatal(err)
	}
	summary, err = st.ImportSite(target.ID, bundle, true)
	if err != nil {
		t.Fatal(err)
	}
	if summary.NetworksCreated != 1 || summary.HostsCreated != 1 {
		t.Fatalf("resumen de replace inesperado: %+v", summary)
	}
	nets, _ = st.ListNetworksBySite(target.ID)
	hosts, _ = st.ListHostsForExport(target.ID)
	if len(nets) != 1 || len(hosts) != 1 || hosts[0].IP != "10.30.0.10" {
		t.Fatalf("importación replace falló: nets=%+v hosts=%+v", nets, hosts)
	}
	// Source site must remain untouched.
	sourceHosts, _ := st.ListHostsForExport(source.ID)
	if len(sourceHosts) != 1 || sourceHosts[0].ID != hostID {
		t.Fatalf("el sitio de origen se modificó: %+v", sourceHosts)
	}
}

func TestDeleteSiteMovesDataAndResolvesCIDRClash(t *testing.T) {
	st := newTestStore(t)
	other, err := st.CreateSite("Sucursal", "")
	if err != nil {
		t.Fatal(err)
	}
	const cidr = "10.40.0.0/24"
	n1, _, err := st.EnsureNetwork(&model.Network{SiteID: 1, Name: "Central", CIDR: cidr, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	n2, _, err := st.EnsureNetwork(&model.Network{SiteID: other.ID, Name: "Sucursal", CIDR: cidr, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	hostID, _, _, _, err := st.UpsertHost(&model.Host{SiteID: 1, NetworkID: n1.ID, IP: "10.40.0.7", Status: model.StatusUp})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := st.CreateScan(&model.Scan{SiteID: 1, NetworkID: n1.ID, CIDR: cidr, Trigger: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSite(1, other.ID); err != nil {
		t.Fatalf("no se pudo eliminar la sede: %v", err)
	}
	h, err := st.GetHost(hostID)
	if err != nil || h.SiteID != other.ID || h.NetworkID != n2.ID {
		t.Fatalf("el host no se movió correctamente: %+v err=%v", h, err)
	}
	sc, err := st.GetScan(scan.ID)
	if err != nil || sc.SiteID != other.ID || sc.NetworkID != n2.ID {
		t.Fatalf("el escaneo no se movió correctamente: %+v err=%v", sc, err)
	}
	nets, _ := st.ListNetworksBySite(other.ID)
	if len(nets) != 1 || nets[0].ID != n2.ID {
		t.Fatalf("quedó una red duplicada: %+v", nets)
	}
	if err := st.DeleteSite(other.ID, 0); err == nil {
		t.Fatal("se pudo eliminar la última sede restante")
	}
}
