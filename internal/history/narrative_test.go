package history

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestNarrativeIdentityChange(t *testing.T) {
	a := period("hoy", []model.HistoryHost{
		{ID: 2, IP: "10.0.0.5", MAC: "cc:cc:cc:cc:cc:cc", Hostname: "srv", Status: model.StatusUp, Samples: 5},
	}, model.HistoryAggregate{Samples: 5, Availability: 100, AvgRTT: 2})
	b := period("antes", []model.HistoryHost{
		{ID: 1, IP: "10.0.0.5", MAC: "aa:aa:aa:aa:aa:aa", Hostname: "srv", Status: model.StatusUp, Samples: 5},
	}, model.HistoryAggregate{Samples: 5, Availability: 100, AvgRTT: 2})
	report := Compare(a, b)
	joined := strings.Join(report.Narrative, " ")
	if !strings.Contains(joined, "cambió de identidad (MAC)") {
		t.Fatalf("no se narró el cambio de MAC: %s", joined)
	}
	if !strings.Contains(joined, "10.0.0.5") {
		t.Fatalf("la narrativa no cita la IP: %s", joined)
	}
}

func TestNarrativeNoChanges(t *testing.T) {
	hosts := []model.HistoryHost{{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusUp, Samples: 3}}
	report := Compare(period("hoy", hosts, model.HistoryAggregate{}), period("antes", hosts, model.HistoryAggregate{}))
	if len(report.Narrative) == 0 || report.Narrative[0] != "No hay cambios registrados en la ventana comparada." {
		t.Fatalf("narrativa inesperada: %+v", report.Narrative)
	}
}

func TestNarrativeDeterministic(t *testing.T) {
	a := period("hoy", []model.HistoryHost{{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusDown, Samples: 2}},
		model.HistoryAggregate{Samples: 2, Availability: 50})
	b := period("antes", []model.HistoryHost{{ID: 1, IP: "10.0.0.1", MAC: "aa:aa:aa:aa:aa:aa", Status: model.StatusUp, Samples: 2}},
		model.HistoryAggregate{Samples: 2, Availability: 100})
	first, _ := json.Marshal(Compare(a, b).Narrative)
	second, _ := json.Marshal(Compare(a, b).Narrative)
	if string(first) != string(second) {
		t.Fatal("la narrativa no es determinista")
	}
}

func TestLegacyDiffJSONHasNoNarrative(t *testing.T) {
	raw, err := json.Marshal(model.DiffReport{Summary: map[string]int{"total": 0}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"narrative"`) {
		t.Fatalf("JSON antiguo contiene narrative: %s", raw)
	}
}
