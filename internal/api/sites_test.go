package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func requestAs(user *model.User, method, path, body, id string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		req.SetPathValue("id", id)
	}
	if user != nil {
		req = req.WithContext(context.WithValue(req.Context(), ctxUser, user))
	}
	return req
}

func TestSitesHandlersAndRBAC(t *testing.T) {
	s := newMetricsServer(t)
	admin := &model.User{Username: "admin", Role: "admin"}
	viewer := &model.User{Username: "viewer", Role: "viewer"}

	// Viewer can list sites but cannot create one.
	rec := httptest.NewRecorder()
	s.handleListSites(rec, requestAs(viewer, http.MethodGet, "/api/v1/sites", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("listar sedes como viewer: código %d", rec.Code)
	}
	var listed struct {
		Items []model.Site `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed.Items) != 1 {
		t.Fatalf("listado inesperado: %s err=%v", rec.Body.String(), err)
	}

	rec = httptest.NewRecorder()
	s.handleCreateSite(rec, requestAs(viewer, http.MethodPost, "/api/v1/sites", `{"name":"No permitido"}`, ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer creó una sede: código %d", rec.Code)
	}

	// Admin creates a site.
	rec = httptest.NewRecorder()
	s.handleCreateSite(rec, requestAs(admin, http.MethodPost, "/api/v1/sites", `{"name":"Sucursal","description":" Norte "}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear sede: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	var created model.Site
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Slug != "sucursal" || created.Description != "Norte" {
		t.Fatalf("sede creada inesperada: %+v", created)
	}

	// Viewer cannot delete; admin can, but not the last remaining site.
	rec = httptest.NewRecorder()
	s.handleDeleteSite(rec, requestAs(viewer, http.MethodDelete, "/api/v1/sites/1", "", "1"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer eliminó una sede: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleDeleteSite(rec, requestAs(admin, http.MethodDelete, "/api/v1/sites/1?move_to=2", "", "1"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("admin no pudo eliminar la sede: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleDeleteSite(rec, requestAs(admin, http.MethodDelete, "/api/v1/sites/2", "", "2"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("se eliminó la última sede: código %d", rec.Code)
	}
}

func TestSitesExportImportHandlers(t *testing.T) {
	s := newMetricsServer(t)
	admin := &model.User{Username: "admin", Role: "admin"}
	operator := &model.User{Username: "op", Role: "operator"}

	rec := httptest.NewRecorder()
	s.handleCreateSite(rec, requestAs(admin, http.MethodPost, "/api/v1/sites", `{"name":"Origen"}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	var site model.Site
	_ = json.Unmarshal(rec.Body.Bytes(), &site)
	id := strconv.FormatInt(site.ID, 10)

	rec = httptest.NewRecorder()
	s.handleExportSite(rec, requestAs(operator, http.MethodGet, "/api/v1/sites/"+id+"/export", "", id))
	if rec.Code != http.StatusOK {
		t.Fatalf("exportar sede: código %d", rec.Code)
	}
	var bundle model.SiteBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Schema != "netdiag.site/v1" {
		t.Fatalf("schema inesperado: %q", bundle.Schema)
	}
	body, _ := json.Marshal(bundle)

	// Operator can import (operational write) with a JSON body.
	rec = httptest.NewRecorder()
	s.handleImportSite(rec, requestAs(operator, http.MethodPost, "/api/v1/sites/"+id+"/import?mode=merge", string(body), id))
	if rec.Code != http.StatusOK {
		t.Fatalf("importar sede: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}

	// A bad schema is rejected.
	bad := strings.Replace(string(body), "netdiag.site/v1", "otro/v9", 1)
	rec = httptest.NewRecorder()
	s.handleImportSite(rec, requestAs(admin, http.MethodPost, "/api/v1/sites/"+id+"/import", bad, id))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("se aceptó un paquete incompatible: código %d", rec.Code)
	}
}
