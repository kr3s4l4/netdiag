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

func auditRequest(user *model.User, method, path, body, id, id2 string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		req.SetPathValue("id", id)
	}
	if id2 != "" {
		req.SetPathValue("id2", id2)
	}
	if user != nil {
		req = req.WithContext(context.WithValue(req.Context(), ctxUser, user))
	}
	return req
}

func TestAuditSnapshotCreateDiffListAndExport(t *testing.T) {
	s := newMetricsServer(t)
	admin := &model.User{Username: "admin", Role: "admin"}
	viewer := &model.User{Username: "viewer", Role: "viewer"}
	hostID, _, _, _, err := s.st.UpsertHost(&model.Host{
		IP: "10.77.0.1", MAC: "02:00:00:00:77:01", SiteID: 1, DeviceType: model.DevRouter,
	})
	if err != nil {
		t.Fatal(err)
	}

	create := func(name string) model.AuditSnapshot {
		rec := httptest.NewRecorder()
		body := `{"site_id":1,"name":"` + name + `"}`
		s.handleCreateAuditSnapshot(rec, auditRequest(admin, http.MethodPost, "/api/v1/audits/snapshot", body, "", ""))
		if rec.Code != http.StatusCreated {
			t.Fatalf("crear %s: código %d: %s", name, rec.Code, rec.Body.String())
		}
		var snap model.AuditSnapshot
		if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
			t.Fatal(err)
		}
		if snap.Data != nil || snap.ID == 0 || snap.Size == 0 || snap.RawSize == 0 {
			t.Fatalf("respuesta de creación no ligera: %+v", snap)
		}
		detail := httptest.NewRecorder()
		s.handleGetAuditSnapshot(detail, auditRequest(admin, http.MethodGet,
			"/api/v1/audits/"+strconv.FormatInt(snap.ID, 10)+"?data=1", "", strconv.FormatInt(snap.ID, 10), ""))
		if detail.Code != http.StatusOK {
			t.Fatalf("detalle %d: código %d: %s", snap.ID, detail.Code, detail.Body.String())
		}
		var full model.AuditSnapshot
		if err := json.Unmarshal(detail.Body.Bytes(), &full); err != nil {
			t.Fatal(err)
		}
		if full.Data == nil || len(full.Data.Hosts) != 1 || full.Data.Config.MetricsDays == 0 {
			t.Fatalf("snapshot incompleto: %+v", full)
		}
		return snap
	}

	first := create("Base")
	notes, tags, favorite := "cambiado tras auditoría", []string{"audit", "changed"}, false
	if err := s.st.UpdateHostMeta(hostID, &notes, &tags, &favorite); err != nil {
		t.Fatal(err)
	}
	second := create("Después")

	rec := httptest.NewRecorder()
	s.handleDiffAuditSnapshots(rec, auditRequest(admin, http.MethodPost,
		"/api/v1/audits/1/diff/2", "", "1", "2"))
	if rec.Code != http.StatusOK {
		t.Fatalf("diff: código %d: %s", rec.Code, rec.Body.String())
	}
	var diff model.AuditDiff
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil {
		t.Fatal(err)
	}
	if diff.FromID != first.ID || diff.ToID != second.ID || len(diff.Changes) == 0 {
		t.Fatalf("diff incorrecto: %+v", diff)
	}
	found := false
	for _, c := range diff.Changes {
		if c.Type == "host.changed" && c.Field == "notes" {
			found = true
		}
		if len(c.Evidence) == 0 {
			t.Fatalf("cambio sin evidencia: %+v", c)
		}
	}
	if !found {
		t.Fatalf("no se detectó el cambio de notas: %+v", diff.Changes)
	}

	rec = httptest.NewRecorder()
	s.handleListAuditSnapshots(rec, auditRequest(viewer, http.MethodGet, "/api/v1/audits", "", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("listar: código %d", rec.Code)
	}
	var list struct {
		Items []model.AuditSnapshot `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Items) != 2 {
		t.Fatalf("listado incorrecto: %s err=%v", rec.Body.String(), err)
	}

	rec = httptest.NewRecorder()
	s.handleExportAuditSnapshot(rec, auditRequest(viewer, http.MethodGet,
		"/api/v1/audits/1/export?format=gzip", "", "1", ""))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "gzip") {
		t.Fatalf("export gzip: código %d tipo %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() == 0 {
		t.Fatal("export gzip vacío")
	}

	rec = httptest.NewRecorder()
	s.handleCreateAuditSnapshot(rec, auditRequest(viewer, http.MethodPost,
		"/api/v1/audits/snapshot", `{}`, "", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer pudo crear snapshot: %d", rec.Code)
	}
}

func TestAuditSnapshotsReadSecretsAreNotStored(t *testing.T) {
	s := newMetricsServer(t)
	admin := &model.User{Username: "admin", Role: "admin"}
	const channelSecret = "VALOR-SECRETO-CANAL-9281"
	const integrationSecret = "VALOR-SECRETO-INTEGRACION-9281"
	if _, err := s.st.CreateChannel(&model.AlertChannel{
		Name: "smtp", Type: "smtp", Enabled: true,
		Config: map[string]any{"host": "smtp.example.net", "password": channelSecret},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.st.CreateIntegration(&model.Integration{
		Name: "elastic", Type: "elasticsearch", Enabled: true,
		Config: map[string]any{"url": "http://127.0.0.1:9200", "api_key": integrationSecret},
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleCreateAuditSnapshot(rec, auditRequest(admin, http.MethodPost,
		"/api/v1/audits/snapshot", `{"site_id":1,"name":"redacted"}`, "", ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var snap model.AuditSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	detail := httptest.NewRecorder()
	idText := strconv.FormatInt(snap.ID, 10)
	s.handleGetAuditSnapshot(detail, auditRequest(admin, http.MethodGet,
		"/api/v1/audits/"+idText+"?data=1", "", idText, ""))
	if detail.Code != http.StatusOK {
		t.Fatalf("detalle: código %d: %s", detail.Code, detail.Body.String())
	}
	var full model.AuditSnapshot
	if err := json.Unmarshal(detail.Body.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	if full.Data == nil {
		t.Fatal("el detalle no incluyó el payload")
	}
	raw, err := json.Marshal(full.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{channelSecret, integrationSecret} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("el snapshot contiene el valor secreto %q: %s", secret, raw)
		}
	}
	// Los nombres de clave sí se conservan para poder auditar cambios de
	// configuración sin exponer los valores.
	if !strings.Contains(string(raw), `"password"`) || !strings.Contains(string(raw), `"api_key"`) {
		t.Fatalf("no se conservaron los nombres de clave: %s", raw)
	}
}
