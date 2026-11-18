package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestEnterpriseDiagnosticsEndpoint(t *testing.T) {
	s := newMetricsServer(t)
	_, _, _, _, err := s.st.UpsertHost(&model.Host{IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:ff", DeviceType: model.DevAccessPoint, SiteID: 1})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleEnterpriseDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/enterprise?site_id=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var report model.DiagnosticsReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SiteID != 1 || !report.EnterpriseMode {
		t.Fatalf("informe inesperado: %+v", report)
	}
	if len(report.Limitations) < 4 {
		t.Fatalf("faltan limitaciones documentadas: %+v", report.Limitations)
	}
	if report.HostsAnalyzed != 1 {
		t.Fatalf("esperaba 1 host analizado, tiene %d", report.HostsAnalyzed)
	}
	found := false
	for _, a := range report.Anomalies {
		if a.Type == "rogue_ap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperaba rogue_ap en el informe: %+v", report.Anomalies)
	}
}

func TestSTPDiagnosticsEndpointShape(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleSTPDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/stp", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		SiteID      int64           `json:"site_id"`
		Loops       []model.STPLoop `json:"loops"`
		Anomalies   []any           `json:"anomalies"`
		Limitations []string        `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Loops == nil || body.Anomalies == nil || body.Limitations == nil {
		t.Fatalf("respuesta incompleta: %s", rec.Body.String())
	}
}

func TestVLANDiagnosticsEndpointShape(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleVLANDiagnostics(rec, httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/vlan", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Leaks       []model.VLANLeak `json:"leaks"`
		Anomalies   []any            `json:"anomalies"`
		Limitations []string         `json:"limitations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Leaks == nil || body.Anomalies == nil || body.Limitations == nil {
		t.Fatalf("respuesta incompleta: %s", rec.Body.String())
	}
}
