package dashboards

import (
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestNormalizeExampleAndRejectBadSources(t *testing.T) {
	d := &model.Dashboard{SiteID: 1, Name: "Mi red"}
	for i, typ := range []string{"dashboard", "hosts", "alerts", "topology"} {
		var w model.Widget
		w.ID = "w" + string(rune('1'+i))
		w.Type = typ
		w.Position.X = (i % 2) * 6
		w.Position.Y = i / 2
		w.Size.W = 6
		w.Size.H = 1
		// A malicious client tries to inject an arbitrary URL.
		w.Config = map[string]any{"source": "http://example.com/steal"}
		d.Widgets = append(d.Widgets, w)
	}
	if err := Normalize(d); err != nil {
		t.Fatal(err)
	}
	if len(d.Widgets) != 4 {
		t.Fatalf("widgets: %d", len(d.Widgets))
	}
	for _, w := range d.Widgets {
		source, _ := w.Config["source"].(string)
		item, ok := ItemFor(w.Type)
		if !ok || source != item.Source || strings.HasPrefix(source, "http") {
			t.Fatalf("fuente no saneada: %+v", w.Config)
		}
	}
}

func TestNormalizeRejectsUnsupportedType(t *testing.T) {
	d := &model.Dashboard{Name: "Mala"}
	var w model.Widget
	w.Type = "unknown"
	d.Widgets = []model.Widget{w}
	if err := Normalize(d); err == nil {
		t.Fatal("tipo desconocido aceptado")
	}
}

func TestNoMoreThanSixteenWidgets(t *testing.T) {
	d := &model.Dashboard{Name: "Llena"}
	for i := 0; i < 17; i++ {
		var w model.Widget
		w.Type = "hosts"
		d.Widgets = append(d.Widgets, w)
	}
	if err := Normalize(d); err == nil {
		t.Fatal("17 widgets aceptados")
	}
}
