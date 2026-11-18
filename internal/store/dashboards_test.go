package store

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/dashboards"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestDashboardCRUDAndExample(t *testing.T) {
	st := newTestStore(t)
	example := ExampleDashboard(1)
	if err := dashboards.Normalize(example); err != nil {
		t.Fatal(err)
	}
	if len(example.Widgets) != 4 {
		t.Fatalf("ejemplo con %d widgets", len(example.Widgets))
	}
	if err := st.CreateDashboard(example); err != nil {
		t.Fatal(err)
	}
	if example.ID == 0 || example.CreatedAt.IsZero() {
		t.Fatal("dashboard sin id/fecha")
	}
	got, err := st.GetDashboard(example.ID)
	if err != nil || len(got.Widgets) != 4 {
		t.Fatalf("lectura: %v %+v", err, got)
	}
	if got.Widgets[0].Config["source"] != "/api/v1/dashboard" {
		t.Fatalf("fuente no persistida: %+v", got.Widgets[0].Config)
	}

	got.Name = "Mi red editada"
	got.Widgets = got.Widgets[:2]
	if err := st.UpdateDashboard(got); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListDashboards(1)
	if err != nil || len(list) != 1 || list[0].Name != "Mi red editada" || len(list[0].Widgets) != 2 {
		t.Fatalf("listado: %v %+v", err, list)
	}
	if list, err := st.ListDashboards(2); err != nil || len(list) != 0 {
		t.Fatalf("filtrado por sede: %v %+v", err, list)
	}
	if err := st.DeleteDashboard(got.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetDashboard(got.ID); err != ErrNotFound {
		t.Fatalf("dashboard borrado todavía accesible: %v", err)
	}
}

func TestDashboardNameUniquePerSite(t *testing.T) {
	st := newTestStore(t)
	if err := st.CreateDashboard(&model.Dashboard{SiteID: 1, Name: "Repetido"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateDashboard(&model.Dashboard{SiteID: 2, Name: "Repetido"}); err != nil {
		t.Fatalf("mismo nombre en otra sede rechazado: %v", err)
	}
	second := &model.Dashboard{SiteID: 1, Name: "Repetido"}
	if err := st.CreateDashboard(second); err == nil {
		t.Fatal("nombre duplicado en la misma sede aceptado")
	}
}
