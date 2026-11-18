package store

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestSavedViewDefaultsSeedOnce(t *testing.T) {
	st := newTestStore(t)
	if err := st.EnsureDefaultViews(7); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListViews(7, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("se esperaban 4 vistas por defecto, hay %d", len(items))
	}
	want := []string{"Caídos", "Críticas", "Cambios de la semana", "Nuevos hosts"}
	got := []string{}
	for _, v := range items {
		got = append(got, v.Name)
		if !v.Builtin {
			t.Errorf("la vista %q debería marcarse como integrada", v.Name)
		}
	}
	for _, name := range want {
		if !containsString(got, name) {
			t.Errorf("falta la vista por defecto %q en %v", name, got)
		}
	}
	alerts, err := st.ListViews(7, ViewKindAlerts, 0)
	if err != nil || len(alerts) != 2 {
		t.Fatalf("filtrar por tipo: %d vistas, err=%v", len(alerts), err)
	}
	if err := st.DeleteView(items[0].ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureDefaultViews(7); err != nil {
		t.Fatal(err)
	}
	items, _ = st.ListViews(7, "", 0)
	if len(items) != 3 {
		t.Fatalf("una vista eliminada no debe reaparecer: hay %d", len(items))
	}
	// Cada usuario tiene su propio conjunto de vistas.
	other, err := st.ListViews(8, "", 0)
	if err != nil || len(other) != 0 {
		t.Fatalf("el usuario 8 no debería ver vistas del 7: %d, err=%v", len(other), err)
	}
}

func TestSavedViewCRUDAndUserIsolation(t *testing.T) {
	st := newTestStore(t)
	const owner, other = int64(10), int64(20)
	v := &SavedView{
		UserID: owner, SiteID: 1, Name: "Mis críticas", Kind: ViewKindAlerts,
		Severity: "critical", Status: "active", Sort: "created_at", Desc: true,
	}
	if err := st.CreateView(v); err != nil {
		t.Fatal(err)
	}
	if v.ID == 0 || v.CreatedAt.IsZero() {
		t.Fatalf("la vista no recibió ID/timestamps: %+v", v)
	}

	list, err := st.ListViews(owner, "", 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("listado del propietario: %d, err=%v", len(list), err)
	}
	list, err = st.ListViews(owner, "", 2)
	if err != nil || len(list) != 0 {
		t.Fatalf("una vista de la sede 1 no debe aparecer en la sede 2: %d, err=%v", len(list), err)
	}
	list, err = st.ListViews(other, "", 0)
	if err != nil || len(list) != 0 {
		t.Fatalf("otro usuario vio la vista: %d, err=%v", len(list), err)
	}

	if _, err := st.GetView(v.ID, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("otro usuario pudo leer la vista: %v", err)
	}
	modified := *v
	modified.Name = "Vista ajena"
	if err := st.UpdateView(&modified, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("otro usuario pudo editar la vista: %v", err)
	}
	if err := st.DeleteView(v.ID, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("otro usuario pudo borrar la vista: %v", err)
	}

	modified.Name = "Críticas propias"
	modified.Query = "sw-core"
	if err := st.UpdateView(&modified, owner); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetView(v.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Críticas propias" || got.Query != "sw-core" || got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("actualización incorrecta: %+v", got)
	}
	if err := st.DeleteView(v.ID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetView(v.ID, owner); !errors.Is(err, ErrNotFound) {
		t.Fatalf("la vista sigue existiendo: %v", err)
	}
}

func TestSavedViewSanitizationAndValidation(t *testing.T) {
	st := newTestStore(t)
	v := &SavedView{
		UserID: 1, Name: "  <script>alert(1)</script>  ", Kind: ViewKindHosts,
		Query: "  <img src=x onerror=alert(1)>  ", Sort: "ip",
	}
	if err := st.CreateView(v); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(v.Name, "<") || strings.Contains(v.Query, "<") {
		t.Fatalf("se persistió HTML sin escapar: %q %q", v.Name, v.Query)
	}
	if v.Name != "&lt;script&gt;alert(1)&lt;/script&gt;" {
		t.Fatalf("nombre inesperado: %q", v.Name)
	}
	got, err := st.GetView(v.ID, 1)
	if err != nil || got.Name != v.Name || got.Query != v.Query {
		t.Fatalf("sanitización no persistida: %+v err=%v", got, err)
	}
	// Sanitizar dos veces no debe duplicar los escapeos.
	if err := ValidateView(got); err != nil || got.Name != v.Name {
		t.Fatalf("sanitización no idempotente: %q err=%v", got.Name, err)
	}

	cases := []struct {
		name string
		mut  func(*SavedView)
	}{
		{"nombre vacío", func(v *SavedView) { v.Name = "   " }},
		{"nombre largo", func(v *SavedView) { v.Name = strings.Repeat("a", 81) }},
		{"consulta larga", func(v *SavedView) { v.Query = strings.Repeat("q", 201) }},
		{"tipo inválido", func(v *SavedView) { v.Kind = "hosts; DROP TABLE hosts" }},
		{"severidad inválida", func(v *SavedView) { v.Kind, v.Severity = ViewKindAlerts, "crítico" }},
		{"estado inválido", func(v *SavedView) { v.Status = "bogus" }},
		{"columna inválida", func(v *SavedView) { v.Columns = []string{"ip) OR 1=1 --"} }},
		{"orden inválido", func(v *SavedView) { v.Sort = "ip DESC; DROP TABLE hosts" }},
		{"ventana inválida", func(v *SavedView) { v.Window = "100000d" }},
		{"demasiadas columnas", func(v *SavedView) {
			v.Columns = []string{"ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip", "ip"}
		}},
	}
	for _, c := range cases {
		tmp := &SavedView{UserID: 99, Name: "válida", Kind: ViewKindHosts, Query: "ip", Sort: "ip"}
		c.mut(tmp)
		if err := ValidateView(tmp); err == nil {
			t.Errorf("%s: se aceptó una vista inválida: %+v", c.name, tmp)
		}
	}
}

func TestSavedViewLimitPerUser(t *testing.T) {
	st := newTestStore(t)
	for i := 0; i < MaxViewsPerUser; i++ {
		v := &SavedView{UserID: 1, Name: fmt.Sprintf("vista-%03d", i), Kind: ViewKindHosts}
		if err := st.CreateView(v); err != nil {
			t.Fatalf("crear vista %d: %v", i, err)
		}
	}
	if err := st.CreateView(&SavedView{UserID: 1, Name: "exceso", Kind: ViewKindHosts}); !errors.Is(err, ErrViewLimit) {
		t.Fatalf("se superó el límite: %v", err)
	}
	if err := st.CreateView(&SavedView{UserID: 1, Name: "vista-001", Kind: ViewKindHosts}); !errors.Is(err, ErrViewDuplicate) {
		t.Fatalf("se duplicó el nombre: %v", err)
	}
	if err := st.CreateView(&SavedView{UserID: 2, Name: "vista-001", Kind: ViewKindHosts}); err != nil {
		t.Fatalf("el límite es por usuario: %v", err)
	}
}

func TestSavedViewShareParamsRoundTrip(t *testing.T) {
	v := &SavedView{
		Name: "Críticas & <importantes>", Kind: ViewKindAlerts, SiteID: 3,
		Query: "core & edge <script>", Severity: "critical", Status: "active",
		Window: "7d", Columns: []string{"title", "severity"}, Sort: "created_at", Desc: true,
	}
	if err := ValidateView(v); err != nil {
		t.Fatal(err)
	}
	path := ViewSharePath(v)
	if !strings.HasPrefix(path, "#/alerts?") {
		t.Fatalf("ruta de compartición inesperada: %q", path)
	}
	if strings.Contains(path, "<") || strings.Contains(path, ">") {
		t.Fatalf("la URL compartida contiene HTML sin codificar: %q", path)
	}
	raw := strings.SplitN(path, "?", 2)[1]
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseShareParams(q)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != v.Kind || parsed.SiteID != v.SiteID || parsed.Query != v.Query ||
		parsed.Severity != v.Severity || parsed.Status != v.Status || parsed.Window != v.Window ||
		parsed.Sort != v.Sort || !parsed.Desc || strings.Join(parsed.Columns, ",") != strings.Join(v.Columns, ",") {
		t.Fatalf("la URL no conserva los filtros: %+v vs %+v", parsed, v)
	}
	if _, err := ParseShareParams(url.Values{"kind": {"alerts"}, "site_id": {"x"}}); err == nil {
		t.Fatal("se aceptó una sede no numérica en la URL")
	}
}
