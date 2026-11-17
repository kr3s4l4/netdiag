package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestUpsertHostRekeysToMACAndPreservesMetadata(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	id, created, _, _, err := st.UpsertHost(&model.Host{
		IP:         "192.168.1.10",
		DeviceType: model.DevUnknown,
		Status:     model.StatusUp,
		FirstSeen:  now.Add(-time.Hour),
		LastSeen:   now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || id == 0 {
		t.Fatalf("se esperaba un host nuevo, created=%v id=%d", created, id)
	}
	h, err := st.GetHost(id)
	if err != nil {
		t.Fatal(err)
	}
	if h.Identity != "ip:192.168.1.10" {
		t.Fatalf("identidad inesperada: %q", h.Identity)
	}

	notes := "switch del armario"
	tags := []string{"infra", "planta-1"}
	fav := true
	if err := st.UpdateHostMeta(id, &notes, &tags, &fav); err != nil {
		t.Fatal(err)
	}

	id2, created2, prev, changes, err := st.UpsertHost(&model.Host{
		IP:         "192.168.1.10",
		MAC:        "AA:BB:CC:DD:EE:01",
		Hostname:   "sw-1",
		Vendor:     "Cisco",
		DeviceType: model.DevSwitch,
		Status:     model.StatusUp,
		Ports:      []model.Port{{Number: 22, Protocol: "tcp", State: "open", Service: "ssh"}},
		LastSeen:   now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("el host no debería haberse duplicado")
	}
	if id2 != id {
		t.Fatalf("el ID cambió al rekeyear: %d → %d", id, id2)
	}
	if prev == nil || prev.MAC != "" {
		t.Fatalf("el estado previo no se devolvió correctamente: %+v", prev)
	}
	if len(changes) == 0 {
		t.Fatal("se esperaban cambios detectados")
	}
	h, err = st.GetHost(id)
	if err != nil {
		t.Fatal(err)
	}
	if h.Identity != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("identidad no rekeyada: %q", h.Identity)
	}
	if h.Notes != notes || !h.Favorite || len(h.Tags) != 2 {
		t.Fatalf("los metadatos del usuario se perdieron: %+v", h)
	}
	if h.OpenPortCount != 1 || len(h.Ports) != 1 {
		t.Fatalf("puertos no guardados: %+v", h.Ports)
	}

	res, err := st.ListHosts(model.HostFilter{Search: "sw-1", Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("búsqueda por hostname devolvió %d resultados", res.Total)
	}
}

func TestUpsertHostKeepsPortsWhenNilAndClearsWhenEmpty(t *testing.T) {
	st := newTestStore(t)
	base := model.Host{IP: "10.0.0.5", MAC: "aa:bb:cc:00:00:05", Status: model.StatusUp}
	id, _, _, _, err := st.UpsertHost(&base)
	if err != nil {
		t.Fatal(err)
	}
	withPorts := base
	withPorts.Ports = []model.Port{{Number: 80, Protocol: "tcp", State: "open"}}
	if _, _, _, _, err := st.UpsertHost(&withPorts); err != nil {
		t.Fatal(err)
	}
	h, _ := st.GetHost(id)
	if h.OpenPortCount != 1 {
		t.Fatalf("se esperaba 1 puerto, hay %d", h.OpenPortCount)
	}
	noScan := base
	noScan.Ports = nil
	if _, _, _, _, err := st.UpsertHost(&noScan); err != nil {
		t.Fatal(err)
	}
	h, _ = st.GetHost(id)
	if h.OpenPortCount != 1 {
		t.Fatal("un escaneo sin puertos no debe borrar los anteriores")
	}
	empty := base
	empty.Ports = []model.Port{}
	if _, _, _, _, err := st.UpsertHost(&empty); err != nil {
		t.Fatal(err)
	}
	h, _ = st.GetHost(id)
	if h.OpenPortCount != 0 {
		t.Fatal("una lista vacía debe limpiar los puertos")
	}
}

func TestNetworksAndScans(t *testing.T) {
	st := newTestStore(t)
	n, created, err := st.EnsureNetwork(&model.Network{Name: "LAN", CIDR: "192.168.50.0/24", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("la red debería ser nueva")
	}
	again, created, err := st.EnsureNetwork(&model.Network{Name: "LAN", CIDR: "192.168.50.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if created || again.ID != n.ID {
		t.Fatal("EnsureNetwork no es idempotente")
	}

	sc, err := st.CreateScan(&model.Scan{NetworkID: n.ID, CIDR: n.CIDR, Profile: "quick", Trigger: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if sc.Status != model.ScanQueued {
		t.Fatalf("estado inicial inesperado: %q", sc.Status)
	}
	if err := st.StartScan(sc.ID, 254); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateScanProgress(sc.ID, "puertos", 100, 254, 7, 39.4); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishScan(sc.ID, model.ScanDone, 7, ""); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetScan(sc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.ScanDone || got.HostsFound != 7 || got.Total != 254 {
		t.Fatalf("escaneo persistido incorrectamente: %+v", got)
	}
	if got.StartedAt == nil || got.FinishedAt == nil {
		t.Fatal("faltan marcas de tiempo del escaneo")
	}
	list, err := st.ListScans(model.ScanFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 {
		t.Fatalf("historial inesperado: %d", list.Total)
	}
}

func TestMarkMissingHosts(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	a, _, _, _, err := st.UpsertHost(&model.Host{IP: "10.1.0.2", MAC: "aa:bb:cc:00:00:02", Status: model.StatusUp, NetworkID: 4, LastSeen: now})
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, _, err := st.UpsertHost(&model.Host{IP: "10.1.0.3", MAC: "aa:bb:cc:00:00:03", Status: model.StatusUp, NetworkID: 4, LastSeen: now})
	if err != nil {
		t.Fatal(err)
	}
	missing, err := st.MarkMissingHosts(4, []int64{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0].ID != b {
		t.Fatalf("host ausente incorrecto: %+v", missing)
	}
	ha, _ := st.GetHost(a)
	hb, _ := st.GetHost(b)
	if ha.Status != model.StatusUp {
		t.Fatal("el host visto no debe marcarse caído")
	}
	if hb.Status != model.StatusDown {
		t.Fatalf("el host ausente debería estar down: %q", hb.Status)
	}
}

func TestAlertsDedupeAndStats(t *testing.T) {
	st := newTestStore(t)
	if err := st.EnsureBuiltinRules(); err != nil {
		t.Fatal(err)
	}
	rules, err := st.ListRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 6 {
		t.Fatalf("se esperaban 6 reglas integradas, hay %d", len(rules))
	}
	// Idempotente.
	if err := st.EnsureBuiltinRules(); err != nil {
		t.Fatal(err)
	}
	rules, _ = st.ListRules()
	if len(rules) != 6 {
		t.Fatalf("las reglas se duplicaron: %d", len(rules))
	}

	a1, created, err := st.CreateAlert(&model.Alert{
		RuleName: "Host caído",
		Severity: model.SeverityCritical,
		Title:    "Host caído",
		Message:  "192.168.1.20 no responde",
		HostIP:   "192.168.1.20",
	}, "host_down:192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("la primera alerta debería crearse")
	}
	dup, created, err := st.CreateAlert(&model.Alert{Title: "Host caído"}, "host_down:192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if created || dup.ID != a1.ID {
		t.Fatal("la deduplicación no funcionó")
	}
	if err := st.SetAlertStatus(a1.ID, model.AlertResolved); err != nil {
		t.Fatal(err)
	}
	stats, err := st.AlertStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Resolved != 1 || stats.Active != 0 {
		t.Fatalf("estadísticas inesperadas: %+v", stats)
	}
}

func TestChannelsEncryptSecrets(t *testing.T) {
	st := newTestStore(t)
	ch, err := st.CreateChannel(&model.AlertChannel{
		Name:    "NOC",
		Type:    "webhook",
		Enabled: true,
		Config:  map[string]any{"url": "https://example.com/hook", "secret": "s3cr3t"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ch.HasSecret {
		t.Fatal("el canal debería tener secreto")
	}
	got, err := st.GetChannel(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config["secret"] != "s3cr3t" {
		t.Fatalf("el secreto no se descifró correctamente: %+v", got.Config)
	}
}

func TestDashboard(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	if _, _, _, _, err := st.UpsertHost(&model.Host{IP: "10.2.0.2", MAC: "aa:bb:cc:00:10:02", DeviceType: model.DevServer, Status: model.StatusUp, LastSeen: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := st.UpsertHost(&model.Host{IP: "10.2.0.3", MAC: "aa:bb:cc:00:10:03", DeviceType: model.DevPrinter, Status: model.StatusDown, LastSeen: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.EnsureNetwork(&model.Network{Name: "LAN", CIDR: "10.2.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	stats, err := st.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if stats.HostsTotal != 2 || stats.HostsUp != 1 || stats.HostsDown != 1 {
		t.Fatalf("contadores inesperados: %+v", stats)
	}
	if stats.Networks != 1 {
		t.Fatalf("redes inesperadas: %d", stats.Networks)
	}
	if stats.ByType[string(model.DevServer)] != 1 {
		t.Fatalf("tipos inesperados: %+v", stats.ByType)
	}
}
