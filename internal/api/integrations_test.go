package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func TestIntegrationCRUDAndSecretMasking(t *testing.T) {
	s := newMetricsServer(t)
	admin := &model.User{Username: "admin", Role: "admin"}
	body := `{"name":"Elastic","type":"elasticsearch","enabled":true,"config":{"url":"http://127.0.0.1:9200","api_key":"super-secret"}}`
	rec := httptest.NewRecorder()
	req := requestAs(admin, http.MethodPost, "/api/v1/integrations", body, "")
	s.handleCreateIntegration(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var created model.Integration
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Config["api_key"] != maskedSecret {
		t.Fatalf("el secreto debe enmascararse: %+v", created.Config)
	}
	if !created.HasSecret {
		t.Fatal("has_secret debe ser true")
	}

	// Updating with the sentinel must keep the stored secret.
	update := `{"name":"Elastic 2","config":{"api_key":"********"}}`
	rec = httptest.NewRecorder()
	req = requestAs(admin, http.MethodPut, "/api/v1/integrations/1", update, "1")
	s.handleUpdateIntegration(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	stored, err := s.st.GetIntegration(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Elastic 2" || stored.Config["api_key"] != "super-secret" {
		t.Fatalf("la actualización conservó mal el secreto: %+v", stored)
	}

	// Invalid type is rejected.
	rec = httptest.NewRecorder()
	req = requestAs(admin, http.MethodPost, "/api/v1/integrations", `{"name":"X","type":"carrier-pigeon"}`, "")
	s.handleCreateIntegration(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, tiene %d", rec.Code)
	}

	// Listing exposes types and masks every secret.
	rec = httptest.NewRecorder()
	s.handleListIntegrations(rec, requestAs(admin, http.MethodGet, "/api/v1/integrations", "", ""))
	var list struct {
		Items []model.Integration `json:"items"`
		Types []string            `json:"types"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Types) != len(store.IntegrationTypes) || len(list.Items) != 1 {
		t.Fatalf("listado inesperado: %s", rec.Body.String())
	}
	if list.Items[0].Config["api_key"] != maskedSecret {
		t.Fatalf("secreto filtrado en el listado: %+v", list.Items[0].Config)
	}

	// Delete.
	rec = httptest.NewRecorder()
	req = requestAs(admin, http.MethodDelete, "/api/v1/integrations/1", "", "1")
	s.handleDeleteIntegration(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIntegrationRBACViewerCannotWrite(t *testing.T) {
	s := newMetricsServer(t)
	viewer := &model.User{Username: "viewer", Role: "viewer"}
	admin := &model.User{Username: "admin", Role: "admin"}
	body := `{"name":"Hook","type":"webhook","config":{"url":"https://example.com/hook"}}`

	rec := httptest.NewRecorder()
	s.handleCreateIntegration(rec, requestAs(viewer, http.MethodPost, "/api/v1/integrations", body, ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer no debe crear integraciones: código %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleCreateIntegration(rec, requestAs(admin, http.MethodPost, "/api/v1/integrations", body, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin debe crear integraciones: código %d: %s", rec.Code, rec.Body.String())
	}

	// Viewer may list integrations (read-only).
	rec = httptest.NewRecorder()
	s.handleListIntegrations(rec, requestAs(viewer, http.MethodGet, "/api/v1/integrations", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer debe poder listar: código %d", rec.Code)
	}
}
