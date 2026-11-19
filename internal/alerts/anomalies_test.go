package alerts

import (
	"path/filepath"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func TestDedicatedSecurityRuleTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, r := range []*model.AlertRule{
		{Name: "Genérica", Type: "anomaly", Enabled: true, Severity: model.SeverityWarning, Config: map[string]any{"min_confidence": 75}},
		{Name: "ARP dedicada", Type: "arp_spoofing", Enabled: true, Severity: model.SeverityCritical, Config: map[string]any{"min_confidence": 75}},
	} {
		if _, err := st.CreateRule(r); err != nil {
			t.Fatal(err)
		}
	}
	m := New(st, config.Alerts{Enabled: true, MaxPerMinute: 30}, nil)
	findings := []model.DiagnosticAnomaly{
		{ID: "arp:gw", Type: "arp_gateway_mac_changed", Category: "arp", Severity: model.SeverityCritical,
			Title: "MITM ARP", Detail: "Cambió la MAC", Confidence: 95, HostIPs: []string{"192.168.0.1"}},
		{ID: "vlan:fuga", Type: "vlan_leak", Category: "vlan", Severity: model.SeverityWarning,
			Title: "Fuga VLAN", Detail: "Cruce de red", Confidence: 95, HostIPs: []string{"10.0.0.1"}},
	}
	if got := m.EvaluateAnomalies(findings); got != 2 {
		t.Fatalf("esperaba 2 disparos (uno por regla), obtuve %d", got)
	}
	all, err := st.ListAlerts(model.AlertFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Items) != 2 {
		t.Fatalf("esperaba 2 alertas, obtuve %d", len(all.Items))
	}
	byRule := map[string]model.Alert{}
	for _, a := range all.Items {
		byRule[a.RuleName] = a
	}
	arpAlert, ok := byRule["ARP dedicada"]
	if !ok || arpAlert.Title != "MITM ARP" || arpAlert.Severity != model.SeverityCritical {
		t.Fatalf("la ARP debe usar la regla dedicada: %+v", all.Items)
	}
	genericAlert, ok := byRule["Genérica"]
	if !ok || genericAlert.Title != "Fuga VLAN" {
		t.Fatalf("la genérica solo debe cubrir VLAN: %+v", all.Items)
	}
}

func TestEvaluateAnomalies(t *testing.T) {
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rule, err := st.CreateRule(&model.AlertRule{
		Name: "Seguridad ARP", Type: "anomaly", Enabled: true,
		Severity: model.SeverityCritical, Config: map[string]any{"min_confidence": 80, "category": "arp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID <= 0 {
		t.Fatal("la regla creada no tiene ID")
	}
	hostID, _, _, _, err := st.UpsertHost(&model.Host{IP: "192.168.0.1", SiteID: 1, DeviceType: model.DevUnknown, Status: model.StatusUp})
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, config.Alerts{Enabled: true, MaxPerMinute: 30}, nil)
	findings := []model.DiagnosticAnomaly{
		{ID: "arp:1", Type: "arp_gateway_mac_changed", Category: "arp", Severity: model.SeverityCritical,
			Title: "Posible MITM", Detail: "Cambió la MAC del gateway", Confidence: 95, HostIPs: []string{"192.168.0.1"}},
		{ID: "arp:2", Type: "arp_ip_mac_changed", Category: "arp", Severity: model.SeverityWarning,
			Title: "Cambio de MAC", Detail: "Cambió una IP", Confidence: 75, HostIPs: []string{"192.168.0.20"}},
		{ID: "vlan:1", Type: "vlan_leak", Category: "vlan", Severity: model.SeverityWarning,
			Title: "Fuga", Detail: "Otra categoría", Confidence: 95, HostIPs: []string{"10.0.0.1"}},
	}
	if got := m.EvaluateAnomalies(findings); got != 1 {
		t.Fatalf("esperaba 1 alerta (confianza 95 y categoría arp), obtuve %d", got)
	}
	all, err := st.ListAlerts(model.AlertFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Items) != 1 || all.Items[0].Title != "Posible MITM" || all.Items[0].HostIP != "192.168.0.1" {
		t.Fatalf("alerta incorrecta: %+v", all.Items)
	}
	if all.Items[0].HostID != hostID {
		t.Fatalf("la alerta no heredó el host del inventario: %+v", all.Items[0])
	}
	scoped, err := st.ListAlerts(model.AlertFilter{SiteID: 1})
	if err != nil || len(scoped.Items) != 1 {
		t.Fatalf("la alerta debe aparecer filtrando por sede: %d, %v", len(scoped.Items), err)
	}
	// Segunda evaluación: el dedupe evita duplicados.
	if got := m.EvaluateAnomalies(findings); got != 1 {
		t.Fatalf("la segunda evaluación respetó el filtro pero no pudimos comprobar el dedupe por el contador, obtuve %d", got)
	}
	all, _ = st.ListAlerts(model.AlertFilter{})
	if len(all.Items) != 1 {
		t.Fatalf("esperaba 1 alerta tras repetir, hay %d", len(all.Items))
	}
}
