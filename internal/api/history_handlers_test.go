package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestHistoryCompareEndpointDefaultsToSevenDays(t *testing.T) {
	s := newMetricsServer(t)
	id, _, _, _, err := s.st.UpsertHost(&model.Host{IP: "10.9.0.1", MAC: "aa:bb:cc:dd:ee:02", SiteID: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := s.st.AddMetric(id, 20, true, 0, now.Add(-7*24*time.Hour-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.st.AddMetric(id, 4, true, 0, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.handleHistoryCompare(rec, requestAs(&model.User{Username: "admin", Role: "admin"},
		http.MethodPost, "/api/v1/history/compare", `{}`, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var report model.DiffReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.PeriodA.Label != "hoy (24h)" || report.PeriodB.Label != "hace 7 días" {
		t.Fatalf("periodos por defecto incorrectos: %q / %q", report.PeriodA.Label, report.PeriodB.Label)
	}
	if report.PeriodA.Metrics.Samples != 1 || report.PeriodB.Metrics.Samples != 1 {
		t.Fatalf("muestras por periodo incorrectas: %+v / %+v", report.PeriodA.Metrics, report.PeriodB.Metrics)
	}
	found := false
	for _, c := range report.Changes {
		if c.Type == "host.rtt_changed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no se detectó el cambio de RTT: %+v", report.Changes)
	}
}

func TestHistoryCompareEndpointValidatesWindows(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleHistoryCompare(rec, requestAs(nil, http.MethodPost, "/api/v1/history/compare", `{"days":999}`, ""))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("days inválido aceptado: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleHistoryCompare(rec, requestAs(nil, http.MethodPost, "/api/v1/history/compare",
		`{"period_a":{"from":"2026-11-18T12:00:00Z","to":"2026-11-17T12:00:00Z"}}`, ""))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "anterior") {
		t.Fatalf("ventana invertida aceptada: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHistoryCompareOpenAPIRoute(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleOpenAPI(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/api/v1/history/compare") {
		t.Fatalf("ruta no documentada: %d", rec.Code)
	}
}
