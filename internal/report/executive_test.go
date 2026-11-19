package report

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestExecutivePDFIsOnePageAndRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	g := New(dir, "Laboratorio", "PIE DEL LLAMADOR NO DEBE APARECER")
	rec, err := g.Generate(Request{
		Kind:    "executive",
		Format:  "pdf",
		Title:   "Informe ejecutivo de red",
		Company: "Laboratorio",
		To:      time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
		Stats: model.DashboardStats{
			HostsTotal: 12, HostsUp: 10, HostsDown: 2, AlertsCritical: 3,
			Availability: 98.5, AvgRTT: 12.3,
		},
		Version:       "1.5.2",
		Edition:       "community",
		RetentionDays: 7,
		Samples7D:     120,
		Changes: []model.HistoryChange{{
			Type: "host.added", Label: "Host nuevo 172.20.50.60",
			Evidence: []string{"comunidad public · 3 muestras"},
		}},
		Findings: []model.DiagnosticAnomaly{{
			Severity: "critical", Confidence: 90,
			Title: "Servidor DHCP no autorizado · comunidad secretpass",
		}},
	})
	if err != nil {
		t.Fatalf("generar informe ejecutivo: %v", err)
	}
	data, err := os.ReadFile(rec.Path)
	if err != nil {
		t.Fatalf("leer PDF: %v", err)
	}
	pages := regexp.MustCompile(`/Type\s*/Page\b`).FindAll(data, -1)
	if len(pages) != 1 {
		t.Fatalf("el PDF ejecutivo debe tener 1 página, tiene %d", len(pages))
	}
	text := string(data)
	for _, leak := range []string{"public", "secretpass", "PIE DEL LLAMADOR"} {
		if strings.Contains(text, leak) {
			t.Errorf("el PDF expone %q", leak)
		}
	}
	if !strings.Contains(text, "[redactada]") {
		t.Error("el PDF debería marcar la comunidad SNMP como redactada")
	}
	// fpdf codifica el texto en Latin-1; comprobamos las marcas ASCII del pie.
	for _, want := range []string{"1.5.2", "community", "19/11/2026", "1 de 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("el pie no incluye %q", want)
		}
	}
}

func TestExecutivePDFIncludesExplanationWithoutExtraPage(t *testing.T) {
	dir := t.TempDir()
	g := New(dir, "Laboratorio", "")
	findings := []model.DiagnosticAnomaly{}
	for i := 0; i < 4; i++ {
		findings = append(findings, model.DiagnosticAnomaly{
			Severity: "critical", Confidence: 95,
			Title:       "Hallazgo de prueba con titulo largo para medir la altura de la tabla",
			Cause:       "Otro equipo responde por la IP del gateway.",
			Impact:      "El trafico puede pasar por un equipo no autorizado.",
			NextSteps:   []string{"Comparar la MAC con el router.", "Localizar el puerto en el switch."},
			Limitations: "Sin captura L2.",
			Hypotheses: []model.Hypothesis{{
				Rank: 1, Label: "Suplantacion", Confidence: 95,
			}},
		})
	}
	rec, err := g.Generate(Request{Kind: "executive", Format: "pdf", Findings: findings})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(rec.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(regexp.MustCompile(`/Type\s*/Page\b`).FindAll(data, -1)); got != 1 {
		t.Fatalf("el PDF con explicaciones debe seguir en 1 pagina, tiene %d", got)
	}
	for _, want := range []string{"causa:", "impacto:", "pasos:", "hipotesis 1:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("el PDF no incluye %q", want)
		}
	}
}

func TestExecutivePDFIncludesNarrativeWithoutExtraPage(t *testing.T) {
	dir := t.TempDir()
	g := New(dir, "Laboratorio", "")
	changes := []model.HistoryChange{}
	for i := 0; i < 5; i++ {
		changes = append(changes, model.HistoryChange{
			Type: "host.added", Label: "Dispositivo nuevo con nombre largo para ocupar espacio",
			Evidence: []string{"tres muestras"},
		})
	}
	findings := []model.DiagnosticAnomaly{}
	for i := 0; i < 4; i++ {
		findings = append(findings, model.DiagnosticAnomaly{
			Severity: "critical", Confidence: 95,
			Title:       "Hallazgo de prueba con titulo largo para medir la altura de la tabla",
			Cause:       "Otro equipo responde por la IP del gateway.",
			Impact:      "El trafico puede pasar por un equipo no autorizado.",
			NextSteps:   []string{"Comparar la MAC con el router.", "Localizar el puerto en el switch."},
			Limitations: "Sin captura L2.",
		})
	}
	rec, err := g.Generate(Request{
		Kind: "executive", Format: "pdf",
		Changes:   changes,
		Narrative: []string{"Aparecieron 5 dispositivos nuevos en la ventana comparada.", "La topología ganó 2 enlaces."},
		Findings:  findings,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(rec.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(regexp.MustCompile(`/Type\s*/Page\b`).FindAll(data, -1)); got != 1 {
		t.Fatalf("el PDF con narrativa debe seguir en 1 pagina, tiene %d", got)
	}
	if !strings.Contains(string(data), "Resumen:") || !strings.Contains(string(data), "Aparecieron 5 dispositivos") {
		t.Fatal("el PDF no incluye la narrativa del diff")
	}
}

func TestRedactForReportKeepAggregates(t *testing.T) {
	in := "Disponibilidad 99,2 % · comunidad public · token: abc123 · 12 hosts"
	out := redactForReport(in)
	for _, want := range []string{"99,2 %", "12 hosts", "[redactada]", "[redactado]"} {
		if !strings.Contains(out, want) {
			t.Errorf("resultado %q no contiene %q", out, want)
		}
	}
}
