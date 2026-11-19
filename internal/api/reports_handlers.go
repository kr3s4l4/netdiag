package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/history"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/report"
)

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	reports, err := s.st.ListReports(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar los informes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": reports})
}

type reportRequest struct {
	SiteID int64  `json:"site_id,omitempty"`
	Kind   string `json:"kind"`
	Format string `json:"format"`
	Title  string `json:"title,omitempty"`
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req reportRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	switch req.Kind {
	case "", "inventory", "executive", "alerts":
	default:
		s.fail(w, r, http.StatusBadRequest, "tipo de informe inválido")
		return
	}
	switch req.Format {
	case "", "pdf", "csv", "json", "html":
	default:
		s.fail(w, r, http.StatusBadRequest, "formato inválido; usa pdf, csv, json o html")
		return
	}
	stats, err := s.st.Dashboard(req.SiteID)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron leer las estadísticas")
		return
	}
	hosts, err := s.st.ListAllHostsForSite(req.SiteID, 20000)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo leer el inventario")
		return
	}
	from, to := parseWindow(r, 7*24*time.Hour)
	alertLimit := 5000
	alerts, err := s.st.ListAlerts(model.AlertFilter{SiteID: req.SiteID, From: &from, To: &to, Page: 1, Limit: alertLimit})
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron leer las alertas")
		return
	}
	// Keep reports readable: cap at the newest 1000 alerts.
	items := alerts.Items
	if len(items) > 1000 {
		items = items[:1000]
	}
	// F32: the executive PDF is a curated one-page summary. It reads the same
	// local sources as /history/compare and /diagnostics/enterprise, replaces
	// the 24 h fleet figures with a seven-day window, and adds only safe
	// edition metadata. No configuration values or credentials are exported.
	edition := "community"
	retentionDays := 7
	samples7D := 0
	var changes []model.HistoryChange
	var narrative []string
	var findings []model.DiagnosticAnomaly
	if req.Kind == "executive" {
		now := time.Now()
		periodA, periodB := history.DefaultPeriods(now, 7, 24*time.Hour)
		if diff, err := history.BuildReport(s.st, req.SiteID, periodA, periodB); err == nil {
			changes = diff.Changes
			narrative = diff.Narrative
		} else if s.log != nil {
			s.log.Warn("no se pudieron leer cambios para el informe ejecutivo", "error", err)
		}
		diagReport := diagnostics.Analyzer{}.Analyze(s.diagnosticsFor(req.SiteID, periodA.From, periodA.To))
		findings = diagReport.Anomalies
		since := now.AddDate(0, 0, -7)
		if availability, rtt, err := s.st.FleetSummaryForSite(req.SiteID, since); err == nil {
			stats.Availability, stats.AvgRTT = availability, rtt
		} else if s.log != nil {
			s.log.Warn("no se pudieron leer métricas de 7 días para el informe ejecutivo", "error", err)
		}
		if samples, err := s.st.MetricsForSite(req.SiteID, since, 50000); err == nil {
			samples7D = len(samples)
		}
		status := s.licenseStatus(now)
		if status.Edition != "" {
			edition = status.Edition
		}
		if status.Limits.RetentionDays > 0 {
			retentionDays = status.Limits.RetentionDays
		}
	}

	gen := s.reports
	if gen == nil {
		s.fail(w, r, http.StatusInternalServerError, "generador de informes no configurado")
		return
	}
	rec, err := gen.Generate(report.Request{
		SiteID:  req.SiteID,
		Demo:    s.isDemo(),
		Kind:    req.Kind,
		Format:  req.Format,
		Title:   req.Title,
		Company: s.companyName(),
		Hosts:   hosts,
		Alerts:  items,
		Stats:   *stats,
		From:    from,
		To:      to,

		Version:       s.version,
		Edition:       edition,
		RetentionDays: retentionDays,
		Samples7D:     samples7D,
		Changes:       changes,
		Narrative:     narrative,
		Findings:      findings,
	})
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo generar el informe: "+err.Error())
		return
	}
	saved, err := s.st.AddReport(rec)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "el informe se generó pero no se pudo registrar: "+err.Error())
		return
	}
	s.audit(r, "report.created", saved.Name, map[string]any{"kind": saved.Kind, "format": saved.Format})
	s.hub.Broadcast("report.created", saved)
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) companyName() string {
	if s.cfg == nil {
		return "NetDiag"
	}
	if name := s.st.GetSetting("company_name", ""); name != "" {
		return name
	}
	return "NetDiag"
}

func (s *Server) handleDownloadReport(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := s.st.GetReport(id)
	if err != nil {
		s.fail(w, r, http.StatusNotFound, "informe no encontrado")
		return
	}
	if err := s.ensureReportPath(rec.Path); err != nil {
		s.fail(w, r, http.StatusForbidden, err.Error())
		return
	}
	if _, err := os.Stat(rec.Path); err != nil {
		s.fail(w, r, http.StatusNotFound, "el archivo del informe ya no existe")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(rec.Name)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, rec.Path)
}

func (s *Server) handleDeleteReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := s.st.GetReport(id)
	if err != nil {
		s.fail(w, r, http.StatusNotFound, "informe no encontrado")
		return
	}
	if s.ensureReportPath(rec.Path) == nil {
		_ = os.Remove(rec.Path)
	}
	if err := s.st.DeleteReport(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "report.deleted", rec.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ensureReportPath blocks path traversal outside the reports directory.
func (s *Server) ensureReportPath(path string) error {
	if path == "" {
		return errText("informe sin ruta")
	}
	base := ""
	if s.reports != nil {
		base = s.reports.Dir
	}
	if base == "" {
		return nil
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return errText("ruta de informe no permitida")
	}
	return nil
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	if name == "" {
		return "netdiag-informe"
	}
	return name
}
