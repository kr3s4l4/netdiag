package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestFindingReviewHandlersAndAudit(t *testing.T) {
	s := newMetricsServer(t)
	viewer := &model.User{Username: "viewer", Role: "viewer"}
	operator := &model.User{Username: "oper", Role: "operator"}
	const findingType = "arp_gateway_mac_changed"
	const hostKey = "mac:02:00:00:00:00:01"

	listBody := func(user *model.User, query string) []model.FindingReview {
		t.Helper()
		rec := httptest.NewRecorder()
		s.handleListFindingReviews(rec, requestAs(user, http.MethodGet,
			"/api/v1/diagnostics/reviews"+query, "", ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("listar revisiones: código %d cuerpo=%s", rec.Code, rec.Body.String())
		}
		var out struct {
			Items []model.FindingReview `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Items
	}

	if items := listBody(viewer, "?site_id=1"); len(items) != 0 {
		t.Fatalf("listado inicial no vacío: %+v", items)
	}

	rec := httptest.NewRecorder()
	s.handleUpsertFindingReview(rec, requestAs(viewer, http.MethodPost,
		"/api/v1/diagnostics/reviews?site_id=1",
		`{"finding_type":"`+findingType+`","host_key":"`+hostKey+`","status":"confirmed"}`, ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer pudo revisar: código %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleUpsertFindingReview(rec, requestAs(operator, http.MethodPost,
		"/api/v1/diagnostics/reviews?site_id=1",
		`{"finding_type":"`+findingType+`","host_key":"`+hostKey+`","status":"wrong"}`, ""))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado inválido aceptado: código %d", rec.Code)
	}

	body := `{"finding_type":"` + findingType + `","host_key":"` + hostKey + `","status":"confirmed","note":"Validado en local"}`
	rec = httptest.NewRecorder()
	s.handleUpsertFindingReview(rec, requestAs(operator, http.MethodPost,
		"/api/v1/diagnostics/reviews?site_id=1", body, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("guardar revisión: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	var saved model.FindingReview
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Status != "confirmed" || saved.Reviewer != "oper" || saved.Note != "Validado en local" || saved.SiteID != 1 {
		t.Fatalf("revisión inesperada: %+v", saved)
	}
	if !saved.ExpiresAt.After(time.Now().Add(29 * 24 * time.Hour)) {
		t.Fatalf("caducidad inesperada: %v", saved.ExpiresAt)
	}

	if items := listBody(viewer, ""); len(items) != 1 || items[0].HostKey != hostKey {
		t.Fatalf("listado sin site_id no incluye la revisión: %+v", items)
	}
	if items := listBody(viewer, "?site_id=1&status=false_positive"); len(items) != 0 {
		t.Fatalf("filtro de estado no aplicado: %+v", items)
	}

	path := "/api/v1/diagnostics/reviews?site_id=1&finding_type=" + url.QueryEscape(findingType) +
		"&host_key=" + url.QueryEscape(hostKey)
	rec = httptest.NewRecorder()
	s.handleDeleteFindingReview(rec, requestAs(viewer, http.MethodDelete, path, "", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer pudo borrar: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleDeleteFindingReview(rec, requestAs(operator, http.MethodDelete, path, "", ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("borrar revisión: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	if items := listBody(viewer, "?site_id=1"); len(items) != 0 {
		t.Fatalf("revisión no borrada: %+v", items)
	}

	audits, err := s.st.ListAudit(model.AuditFilter{Page: 1, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entry := range audits.Items {
		if entry.Actor == "oper" {
			seen[entry.Action] = true
		}
	}
	if !seen["finding_review.upsert"] || !seen["finding_review.deleted"] {
		t.Fatalf("auditoría incompleta: %v", seen)
	}
}
