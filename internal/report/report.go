// Package report renders inventory and alert reports as PDF, CSV, JSON or HTML.
package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/kr3s4l4/netdiag/internal/fingerprint"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// Request describes a report to render.
type Request struct {
	SiteID  int64
	Demo    bool   // marks synthetic data in every exported format
	Kind    string // inventory | executive | alerts
	Format  string // pdf | csv | json | html
	Title   string
	Company string
	Footer  string
	Hosts   []model.Host
	Alerts  []model.Alert
	Stats   model.DashboardStats
	From    time.Time
	To      time.Time

	// Executive one-page fields. For kind=executive the caller is expected to
	// put the seven-day availability and latency in Stats.Availability and
	// Stats.AvgRTT; the rest is curated evidence, never raw configuration.
	Version       string
	Edition       string
	RetentionDays int
	Samples7D     int
	Changes       []model.HistoryChange
	Narrative     []string
	Findings      []model.DiagnosticAnomaly
}

// Generator writes reports into a directory.
type Generator struct {
	Dir     string
	Company string
	Footer  string
}

// New creates a generator.
func New(dir, company, footer string) *Generator {
	return &Generator{Dir: dir, Company: company, Footer: footer}
}

// Generate renders the report and returns the stored record (Path included).
func (g *Generator) Generate(req Request) (*model.Report, error) {
	if g.Dir == "" {
		return nil, fmt.Errorf("directorio de informes no configurado")
	}
	if err := os.MkdirAll(g.Dir, 0o700); err != nil {
		return nil, err
	}
	if req.Kind == "" {
		req.Kind = "inventory"
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if req.To.IsZero() {
		req.To = time.Now()
	}
	if req.From.IsZero() {
		req.From = req.To.Add(-24 * time.Hour)
	}
	if req.Company == "" {
		req.Company = g.Company
	}
	if req.Footer == "" {
		req.Footer = g.Footer
	}
	if req.Title == "" {
		req.Title = defaultTitle(req.Kind)
	}
	stamp := time.Now().Format("20060102-150405")
	prefix := "netdiag-"
	if req.Demo {
		prefix = "netdiag-demo-"
	}
	base := fmt.Sprintf("%s%s-%s.%s", prefix, req.Kind, stamp, req.Format)
	path := filepath.Join(g.Dir, base)

	var err error
	switch strings.ToLower(req.Format) {
	case "pdf":
		if req.Kind == "executive" {
			err = writeExecutivePDF(path, req)
		} else {
			err = writePDF(path, req)
		}
	case "csv":
		err = writeCSV(path, req)
	case "json":
		err = writeJSON(path, req)
	case "html":
		err = writeHTML(path, req)
	default:
		return nil, fmt.Errorf("formato no soportado: %s", req.Format)
	}
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &model.Report{
		SiteID:    req.SiteID,
		Name:      base,
		Format:    strings.ToLower(req.Format),
		Kind:      req.Kind,
		Size:      info.Size(),
		Path:      path,
		CreatedAt: time.Now(),
	}, nil
}

func defaultTitle(kind string) string {
	switch kind {
	case "executive":
		return "Informe ejecutivo de red"
	case "alerts":
		return "Informe de alertas"
	default:
		return "Inventario de red"
	}
}

// Credentials must never reach an export. Diagnostic evidence sometimes
// includes an SNMP community string; these patterns redact it and common
// key=value secrets before any text is written to the executive PDF.
var (
	reportCredentialRE = regexp.MustCompile(`(?i)((?:password|passwd|contraseña|contrase|token|secret|api[_-]?key|clave|community))\s*[:=]\s*([^\s,;]+)`)
	reportCommunityRE  = regexp.MustCompile(`(?i)((?:comunidad|community))\s+([^\s,;·]+)`)
	reportBearerRE     = regexp.MustCompile(`(?i)(bearer)\s+[A-Za-z0-9._-]+`)
)

func redactForReport(s string) string {
	s = reportCredentialRE.ReplaceAllString(s, "$1: [redactado]")
	s = reportCommunityRE.ReplaceAllString(s, "$1 [redactada]")
	s = reportBearerRE.ReplaceAllString(s, "$1 [redactado]")
	return s
}

var executiveChangeLabels = map[string]string{
	"host.added":                 "Host nuevo",
	"host.removed":               "Host retirado",
	"host.status_changed":        "Cambio de estado",
	"host.availability_changed":  "Disponibilidad alterada",
	"host.rtt_changed":           "Latencia alterada",
	"topology.node_added":        "Nodo añadido",
	"topology.node_removed":      "Nodo retirado",
	"topology.edge_added":        "Enlace añadido",
	"topology.edge_removed":      "Enlace retirado",
	"fleet.availability_changed": "Disponibilidad global",
	"fleet.rtt_changed":          "Latencia global",
}

func executiveSeverityLabel(s string) string {
	switch s {
	case "critical":
		return "Crítico"
	case "warning":
		return "Aviso"
	case "info":
		return "Info"
	default:
		return s
	}
}

// writeExecutivePDF renders a deliberately fixed, single-page A4 summary.
// It never embeds the full inventory, raw alert messages or configuration:
// only curated aggregates and redacted evidence are written.
func writeExecutivePDF(path string, req Request) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(false, 0)
	// Keep page streams uncompressed so the one-page export can be audited
	// (and scanned for secrets) with plain text tools.
	pdf.SetCompression(false)
	pdf.SetTitle(req.Title, true)
	pdf.SetAuthor("NetDiag · kr3s4l4", true)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	safe := func(v string) string { return tr(redactForReport(v)) }

	pdf.AddPage()

	// Header band.
	pdf.SetFillColor(10, 14, 20)
	pdf.Rect(0, 0, 210, 38, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(14, 9)
	pdf.CellFormat(150, 9, safe(req.Title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(14, 19)
	pdf.CellFormat(150, 6, safe(req.Company), "", 1, "L", false, 0, "")
	pdf.SetXY(14, 26)
	pdf.CellFormat(150, 6, safe(req.To.Format("02/01/2006 15:04")), "", 1, "L", false, 0, "")
	if req.Demo {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(150, 26)
		pdf.CellFormat(46, 6, safe("DATOS SINTÉTICOS"), "", 1, "R", false, 0, "")
	}

	pdf.SetTextColor(30, 30, 30)
	section := func(y float64, title string) float64 {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetXY(14, y)
		pdf.CellFormat(182, 6, safe(title), "", 1, "L", false, 0, "")
		return y + 8
	}
	kpi := func(x, y, w float64, value, label string) {
		pdf.SetFillColor(244, 246, 249)
		pdf.Rect(x, y, w, 18, "F")
		pdf.SetTextColor(30, 30, 30)
		pdf.SetFont("Helvetica", "B", 15)
		pdf.SetXY(x+2, y+2.5)
		pdf.CellFormat(w-4, 8, safe(value), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(x+2, y+11)
		pdf.CellFormat(w-4, 5, safe(label), "", 1, "L", false, 0, "")
		pdf.SetTextColor(30, 30, 30)
	}

	y := section(44, "Resumen del inventario")
	kpi(14, y, 43, strconv.Itoa(req.Stats.HostsTotal), "Dispositivos")
	kpi(59.5, y, 43, strconv.Itoa(req.Stats.HostsUp), "En línea")
	kpi(105, y, 43, strconv.Itoa(req.Stats.HostsDown), "Caídos")
	kpi(150.5, y, 43, strconv.Itoa(req.Stats.AlertsCritical), "Alertas críticas")
	y += 23

	y = section(y, "Disponibilidad y latencia · últimos 7 días")
	latency := "Sin muestras"
	if req.Stats.AvgRTT > 0 {
		latency = fmt.Sprintf("%.1f ms", req.Stats.AvgRTT)
	}
	kpi(14, y, 88, fmt.Sprintf("%.1f %%", req.Stats.Availability), "Disponibilidad media")
	kpi(108, y, 88, latency, "Latencia media")
	y += 20
	if req.Samples7D > 0 {
		pdf.SetFont("Helvetica", "I", 7.5)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(14, y)
		pdf.CellFormat(182, 5, safe(fmt.Sprintf("Basado en %d muestras de los últimos 7 días.", req.Samples7D)), "", 1, "L", false, 0, "")
		pdf.SetTextColor(30, 30, 30)
		y += 6
	}

	y = section(y, "Cambios principales")
	// F57: the deterministic diff narrative is rendered as compact evidence,
	// never as an invented cause. At most two sentences fit on the one page.
	narrativeLines := req.Narrative
	if len(narrativeLines) > 2 {
		narrativeLines = narrativeLines[:2]
	}
	if len(narrativeLines) > 0 {
		pdf.SetFont("Helvetica", "I", 7.5)
		pdf.SetTextColor(110, 110, 110)
		for _, line := range narrativeLines {
			pdf.SetXY(14, y)
			pdf.CellFormat(182, 4, safe("Resumen: "+clipWords(line, 150)), "", 1, "L", false, 0, "")
			y += 4
		}
		pdf.SetTextColor(30, 30, 30)
	}
	pdf.SetFillColor(10, 14, 20)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetXY(14, y)
	pdf.CellFormat(38, 6, safe("Tipo"), "", 0, "L", true, 0, "")
	pdf.CellFormat(108, 6, safe("Descripción"), "", 0, "L", true, 0, "")
	pdf.CellFormat(36, 6, safe("Evidencia"), "", 1, "L", true, 0, "")
	y += 6
	pdf.SetTextColor(30, 30, 30)
	pdf.SetFont("Helvetica", "", 7.5)
	changes := req.Changes
	maxChanges := 5
	if len(narrativeLines) > 0 {
		maxChanges = 4
	}
	if len(changes) > maxChanges {
		changes = changes[:maxChanges]
	}
	if len(changes) == 0 {
		pdf.SetXY(14, y)
		pdf.CellFormat(182, 8, safe("Sin cambios registrados en la ventana comparada."), "B", 1, "L", false, 0, "")
		y += 8
	}
	for i, c := range changes {
		if i%2 == 0 {
			pdf.SetFillColor(248, 249, 251)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		label := executiveChangeLabels[c.Type]
		if label == "" {
			label = c.Type
		}
		evidence := ""
		if len(c.Evidence) > 0 {
			evidence = c.Evidence[0]
		}
		pdf.SetXY(14, y)
		pdf.CellFormat(38, 9, safe(label), "B", 0, "L", true, 0, "")
		pdf.CellFormat(108, 9, safe(clipWords(c.Label, 62)), "B", 0, "L", true, 0, "")
		pdf.CellFormat(36, 9, safe(clipWords(evidence, 24)), "B", 1, "L", true, 0, "")
		y += 9
	}
	y += 5

	y = section(y, "Hallazgos de mayor severidad")
	pdf.SetFillColor(10, 14, 20)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetXY(14, y)
	pdf.CellFormat(24, 6, safe("Severidad"), "", 0, "L", true, 0, "")
	pdf.CellFormat(122, 6, safe("Hallazgo"), "", 0, "L", true, 0, "")
	pdf.CellFormat(36, 6, safe("Confianza"), "", 1, "R", true, 0, "")
	y += 6
	pdf.SetTextColor(30, 30, 30)
	pdf.SetFont("Helvetica", "", 7.5)
	findings := append([]model.DiagnosticAnomaly{}, req.Findings...)
	sevRank := map[string]int{"critical": 3, "warning": 2, "info": 1}
	sort.SliceStable(findings, func(i, j int) bool {
		ri, rj := sevRank[findings[i].Severity], sevRank[findings[j].Severity]
		if ri != rj {
			return ri > rj
		}
		return findings[i].Confidence > findings[j].Confidence
	})
	if len(findings) > 4 {
		findings = findings[:4]
	}
	if len(findings) == 0 {
		pdf.SetXY(14, y)
		pdf.CellFormat(182, 8, safe("Sin hallazgos relevantes en el periodo analizado."), "B", 1, "L", false, 0, "")
		y += 8
	}
	for i, a := range findings {
		if i%2 == 0 {
			pdf.SetFillColor(248, 249, 251)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetXY(14, y)
		pdf.CellFormat(24, 11, safe(executiveSeverityLabel(a.Severity)), "B", 0, "L", true, 0, "")
		pdf.CellFormat(122, 11, safe(clipWords(a.Title, 78)), "B", 0, "L", true, 0, "")
		pdf.CellFormat(36, 11, strconv.Itoa(a.Confidence)+" %", "B", 1, "R", true, 0, "")
		y += 11
	}
	y += 4

	// F50: the three questions the reader actually asks. Fixed templates, no
	// generated prose, capped to three findings so the page stays auditable.
	if len(findings) > 0 {
		y = section(y, "Causa, impacto y pasos")
		explainCount := len(findings)
		if explainCount > 3 {
			explainCount = 3
		}
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.SetXY(14, y)
		pdf.CellFormat(46, 5, safe("Hallazgo"), "B", 0, "L", true, 0, "")
		pdf.CellFormat(136, 5, safe("Explicación y verificación"), "B", 1, "L", true, 0, "")
		y += 5
		for _, a := range findings[:explainCount] {
			top := y
			pdf.SetTextColor(30, 30, 30)
			pdf.SetFont("Helvetica", "B", 7)
			pdf.SetXY(14, top)
			pdf.MultiCell(46, 3.4, safe(clipWords(a.Title, 42)), "B", "L", false)
			rightY := pdf.GetY()

			parts := []string{}
			if a.Cause != "" {
				parts = append(parts, "causa: "+a.Cause)
			}
			if a.Impact != "" {
				parts = append(parts, "impacto: "+a.Impact)
			}
			if len(a.NextSteps) > 0 {
				steps := a.NextSteps
				if len(steps) > 2 {
					steps = steps[:2]
				}
				parts = append(parts, "pasos: "+strings.Join(steps, "; "))
			}
			if len(a.Hypotheses) > 0 && a.Hypotheses[0].Label != "" {
				parts = append(parts, fmt.Sprintf("hipotesis 1: %s (%d %%)", a.Hypotheses[0].Label, a.Hypotheses[0].Confidence))
			}
			if a.Limitations != "" {
				parts = append(parts, "límite: "+a.Limitations)
			}
			pdf.SetFont("Helvetica", "", 7)
			pdf.SetXY(62, top)
			pdf.MultiCell(136, 3.4, safe(clipWords(strings.Join(parts, " · "), 240)), "B", "L", false)
			if pdf.GetY() > rightY {
				rightY = pdf.GetY()
			}
			y = rightY + 1
		}
		y += 2
	}

	pdf.SetFont("Helvetica", "I", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(14, y)
	pdf.CellFormat(182, 6, safe("Análisis heurístico local: confirma siempre en el equipo de red. No se incluyen credenciales, comunidades SNMP ni configuración."), "", 1, "L", false, 0, "")

	// Controlled footer: no caller-supplied footer text can leak settings.
	pdf.SetFillColor(10, 14, 20)
	pdf.Rect(0, 276, 210, 21, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "", 7.5)
	edition := req.Edition
	if edition == "" {
		edition = "community"
	}
	version := req.Version
	if version == "" {
		version = "—"
	}
	retention := "—"
	if req.RetentionDays > 0 {
		retention = fmt.Sprintf("%d días", req.RetentionDays)
	}
	footer := fmt.Sprintf("NetDiag %s · edición %s · retención %s · generado %s", version, edition, retention, req.To.Format("02/01/2006 15:04"))
	if req.Demo {
		footer += " · DEMO"
	}
	pdf.SetXY(14, 281)
	pdf.CellFormat(182, 5, safe(footer), "", 1, "L", false, 0, "")
	pdf.SetXY(14, 288)
	pdf.CellFormat(182, 5, safe("Página 1 de 1 · informe ejecutivo · datos agregados sin secretos"), "", 1, "L", false, 0, "")

	return pdf.OutputFileAndClose(path)
}

func writePDF(path string, req Request) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 16, 14)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetTitle(req.Title, true)
	pdf.SetAuthor("NetDiag · kr3s4l4", true)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	page := 0
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		footer := req.Footer
		if footer == "" {
			footer = "Generado por NetDiag · kr3s4l4"
		}
		pdf.CellFormat(120, 5, tr(footer), "", 0, "L", false, 0, "")
		pdf.CellFormat(60, 5, fmt.Sprintf("Página %d", page), "", 0, "R", false, 0, "")
	})

	// Cover
	pdf.AddPage()
	page++
	pdf.SetFillColor(10, 14, 20)
	pdf.Rect(0, 0, 210, 70, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetXY(14, 22)
	pdf.CellFormat(180, 12, tr(req.Title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetX(14)
	company := req.Company
	if company == "" {
		company = "NetDiag"
	}
	pdf.CellFormat(180, 7, tr(company), "", 1, "L", false, 0, "")
	pdf.SetX(14)
	pdf.CellFormat(180, 7, tr(req.To.Format("02/01/2006 15:04")), "", 1, "L", false, 0, "")
	if req.Demo {
		pdf.SetX(14)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(180, 7, tr("DATOS SINTÉTICOS · DEMO"), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
	}

	pdf.SetTextColor(30, 30, 30)
	pdf.SetY(82)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, tr("Resumen"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	summary := [][2]string{
		{"Dispositivos inventariados", strconv.Itoa(req.Stats.HostsTotal)},
		{"En línea", strconv.Itoa(req.Stats.HostsUp)},
		{"Caídos", strconv.Itoa(req.Stats.HostsDown)},
		{"Nuevos en 24 h", strconv.Itoa(req.Stats.HostsNew24h)},
		{"Alertas activas", strconv.Itoa(req.Stats.AlertsActive)},
		{"Alertas críticas", strconv.Itoa(req.Stats.AlertsCritical)},
		{"Disponibilidad media", fmt.Sprintf("%.2f %%", req.Stats.Availability)},
		{"Latencia media", fmt.Sprintf("%.1f ms", req.Stats.AvgRTT)},
	}
	for i, row := range summary {
		if i%2 == 0 {
			pdf.SetFillColor(244, 246, 249)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.CellFormat(135, 7, tr(row[0]), "B", 0, "L", true, 0, "")
		pdf.CellFormat(45, 7, tr(row[1]), "B", 1, "R", true, 0, "")
	}

	if len(req.Stats.ByType) > 0 {
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(180, 8, tr("Dispositivos por tipo"), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		type kv struct {
			k string
			v int
		}
		var list []kv
		for k, v := range req.Stats.ByType {
			list = append(list, kv{k, v})
		}
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if list[j].v > list[i].v {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
		for _, item := range list {
			pdf.CellFormat(135, 6, tr(fingerprint.TypeLabel(model.DeviceType(item.k))), "", 0, "L", false, 0, "")
			pdf.CellFormat(45, 6, strconv.Itoa(item.v), "", 1, "R", false, 0, "")
		}
	}

	// Inventory table
	if len(req.Hosts) > 0 {
		pdf.AddPage()
		page++
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(180, 8, tr("Inventario de dispositivos"), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(10, 14, 20)
		pdf.SetTextColor(255, 255, 255)
		headers := []struct {
			label string
			w     float64
		}{
			{"IP", 28}, {"Nombre", 40}, {"MAC", 32}, {"Tipo", 24}, {"Fabricante", 28}, {"Estado", 14}, {"Puertos", 14},
		}
		for _, h := range headers {
			pdf.CellFormat(h.w, 7, tr(h.label), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetTextColor(30, 30, 30)
		pdf.SetFont("Helvetica", "", 7)
		for i, h := range req.Hosts {
			if i%2 == 0 {
				pdf.SetFillColor(248, 249, 251)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			values := []string{
				clip(h.IP, 18),
				clip(h.Hostname, 28),
				clip(h.MAC, 20),
				clip(fingerprint.TypeLabel(h.DeviceType), 16),
				clip(h.Vendor, 20),
				statusLabel(h.Status),
				strconv.Itoa(h.OpenPortCount),
			}
			for j, hd := range headers {
				pdf.CellFormat(hd.w, 6, tr(values[j]), "B", 0, "L", true, 0, "")
			}
			pdf.Ln(-1)
		}
	}

	// Alerts
	if len(req.Alerts) > 0 {
		pdf.AddPage()
		page++
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(180, 8, tr("Alertas"), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(10, 14, 20)
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(22, 7, tr("Severidad"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(52, 7, tr("Título"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(78, 7, tr("Mensaje"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(28, 7, tr("Fecha"), "1", 1, "L", true, 0, "")
		pdf.SetTextColor(30, 30, 30)
		pdf.SetFont("Helvetica", "", 7)
		for _, a := range req.Alerts {
			pdf.CellFormat(22, 6, tr(a.Severity), "B", 0, "L", false, 0, "")
			pdf.CellFormat(52, 6, tr(clip(a.Title, 36)), "B", 0, "L", false, 0, "")
			pdf.CellFormat(78, 6, tr(clip(a.Message, 60)), "B", 0, "L", false, 0, "")
			pdf.CellFormat(28, 6, a.CreatedAt.Format("02/01 15:04"), "B", 1, "L", false, 0, "")
		}
	}
	return pdf.OutputFileAndClose(path)
}

func writeCSV(path string, req Request) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if req.Kind == "alerts" {
		_ = w.Write([]string{"id", "fecha", "severidad", "estado", "regla", "titulo", "mensaje", "ip", "host", "demo"})
		for _, a := range req.Alerts {
			_ = w.Write([]string{
				strconv.FormatInt(a.ID, 10), a.CreatedAt.Format(time.RFC3339), a.Severity, a.Status,
				a.RuleName, a.Title, a.Message, a.HostIP, a.HostName, strconv.FormatBool(req.Demo),
			})
		}
		return w.Error()
	}
	_ = w.Write([]string{"ip", "mac", "hostname", "tipo", "fabricante", "modelo", "so", "estado", "puertos_abiertos", "puertos", "latencia_ms", "primera_vez", "ultima_vez", "etiquetas", "demo"})
	for _, h := range req.Hosts {
		_ = w.Write([]string{
			h.IP, h.MAC, h.Hostname, fingerprint.TypeLabel(h.DeviceType), h.Vendor, h.Model, h.OS,
			statusLabel(h.Status), strconv.Itoa(h.OpenPortCount), portList(h.Ports),
			strconv.FormatFloat(h.LatencyMS, 'f', 2, 64), h.FirstSeen.Format(time.RFC3339), h.LastSeen.Format(time.RFC3339),
			strings.Join(h.Tags, ","), strconv.FormatBool(req.Demo),
		})
	}
	return w.Error()
}

func writeJSON(path string, req Request) error {
	payload := map[string]any{
		"generated_at": time.Now(),
		"demo":         req.Demo,
		"title":        req.Title,
		"company":      req.Company,
		"from":         req.From,
		"to":           req.To,
		"stats":        req.Stats,
		"hosts":        req.Hosts,
		"alerts":       req.Alerts,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func writeHTML(path string, req Request) error {
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"es\"><head><meta charset=\"utf-8\">")
	b.WriteString("<title>" + html.EscapeString(req.Title) + "</title><style>")
	b.WriteString(`body{font-family:system-ui,sans-serif;margin:2rem;color:#111}
h1{margin:0 0 .25rem}small{color:#666}
.grid{display:flex;flex-wrap:wrap;gap:1rem;margin:1.5rem 0}
.card{border:1px solid #ddd;border-radius:10px;padding:1rem 1.25rem;min-width:150px}
.card b{display:block;font-size:1.6rem}
table{border-collapse:collapse;width:100%;font-size:13px;margin-top:1rem}
th{background:#0a0e14;color:#fff;text-align:left;padding:.5rem}
td{border-bottom:1px solid #eee;padding:.45rem}
tr:nth-child(even){background:#f8f9fb}
.sev-critical{color:#b91c1c;font-weight:600}.sev-warning{color:#b45309}.sev-info{color:#1d4ed8}`)
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>" + html.EscapeString(req.Title) + "</h1>")
	b.WriteString("<small>" + html.EscapeString(req.Company) + " · " + req.To.Format("02/01/2006 15:04"))
	if req.Demo {
		b.WriteString(" · <b>DATOS SINTÉTICOS (DEMO)</b>")
	}
	b.WriteString("</small>")
	b.WriteString("<div class=\"grid\">")
	for _, item := range [][2]string{
		{"Dispositivos", strconv.Itoa(req.Stats.HostsTotal)},
		{"En línea", strconv.Itoa(req.Stats.HostsUp)},
		{"Caídos", strconv.Itoa(req.Stats.HostsDown)},
		{"Alertas activas", strconv.Itoa(req.Stats.AlertsActive)},
		{"Disponibilidad", fmt.Sprintf("%.1f%%", req.Stats.Availability)},
	} {
		b.WriteString("<div class=\"card\"><b>" + item[1] + "</b>" + html.EscapeString(item[0]) + "</div>")
	}
	b.WriteString("</div>")
	b.WriteString("<h2>Inventario</h2><table><tr><th>IP</th><th>Nombre</th><th>MAC</th><th>Tipo</th><th>Fabricante</th><th>Estado</th><th>Puertos</th></tr>")
	for _, h := range req.Hosts {
		b.WriteString("<tr><td>" + html.EscapeString(h.IP) + "</td><td>" + html.EscapeString(h.Hostname) +
			"</td><td>" + html.EscapeString(h.MAC) + "</td><td>" + html.EscapeString(fingerprint.TypeLabel(h.DeviceType)) +
			"</td><td>" + html.EscapeString(h.Vendor) + "</td><td>" + html.EscapeString(statusLabel(h.Status)) +
			"</td><td>" + html.EscapeString(portList(h.Ports)) + "</td></tr>")
	}
	b.WriteString("</table>")
	if len(req.Alerts) > 0 {
		b.WriteString("<h2>Alertas</h2><table><tr><th>Severidad</th><th>Título</th><th>Mensaje</th><th>Fecha</th></tr>")
		for _, a := range req.Alerts {
			b.WriteString("<tr><td class=\"sev-" + html.EscapeString(a.Severity) + "\">" + html.EscapeString(a.Severity) +
				"</td><td>" + html.EscapeString(a.Title) + "</td><td>" + html.EscapeString(a.Message) +
				"</td><td>" + a.CreatedAt.Format("02/01 15:04") + "</td></tr>")
		}
		b.WriteString("</table>")
	}
	b.WriteString("</body></html>")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func statusLabel(s model.HostStatus) string {
	switch s {
	case model.StatusUp:
		return "En línea"
	case model.StatusDown:
		return "Caído"
	case model.StatusArchived:
		return "Archivado"
	default:
		return "Desconocido"
	}
}

func portList(ports []model.Port) string {
	var parts []string
	for _, p := range ports {
		if p.State == "open" || p.State == "" {
			label := strconv.Itoa(p.Number)
			if p.Service != "" {
				label += "/" + p.Service
			}
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, " ")
}

func clip(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}

// clipWords truncates at a word boundary so table cells stay readable.
func clipWords(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := max
	for i := max - 1; i > 0; i-- {
		if r[i] == ' ' || r[i] == '·' {
			cut = i
			break
		}
	}
	return strings.TrimRight(string(r[:cut]), " ·") + "…"
}
