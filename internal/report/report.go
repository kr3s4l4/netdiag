// Package report renders inventory and alert reports as PDF, CSV, JSON or HTML.
package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
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
		err = writePDF(path, req)
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
