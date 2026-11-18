package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func decodeView(t *testing.T, rec *httptest.ResponseRecorder) store.SavedView {
	t.Helper()
	var v store.SavedView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("respuesta no es una vista: %s err=%v", rec.Body.String(), err)
	}
	return v
}

func TestViewsHandlersCRUDAndIsolation(t *testing.T) {
	s := newMetricsServer(t)
	owner := &model.User{ID: 11, Username: "operador", Role: "operator"}
	other := &model.User{ID: 22, Username: "otro", Role: "viewer"}

	rec := httptest.NewRecorder()
	s.handleCreateView(rec, requestAs(owner, http.MethodPost, "/api/v1/views",
		`{"name":"Mis críticas","kind":"alerts","site_id":1,"query":"core <b>","severity":"critical","status":"active","columns":["title","severity"],"sort":"created_at","desc":true}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear vista: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	created := decodeView(t, rec)
	if created.ID == 0 || created.UserID != 0 || created.Name != "Mis críticas" {
		// UserID is never serialized; it stays internal to the store row.
		t.Fatalf("vista creada inesperada: %+v", created)
	}
	if strings.Contains(created.Query, "<b>") {
		t.Fatalf("la consulta no se saneó: %q", created.Query)
	}
	id := strconv.FormatInt(created.ID, 10)

	rec = httptest.NewRecorder()
	s.handleListViews(rec, requestAs(owner, http.MethodGet, "/api/v1/views", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("listar vistas: código %d", rec.Code)
	}
	var list struct {
		Items    []store.SavedView `json:"items"`
		Total    int               `json:"total"`
		MaxViews int               `json:"max_views"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 5 || len(list.Items) != 5 || list.MaxViews != store.MaxViewsPerUser {
		t.Fatalf("se esperaban 4 vistas por defecto + 1 propia: %+v", list)
	}

	rec = httptest.NewRecorder()
	s.handleGetView(rec, requestAs(owner, http.MethodGet, "/api/v1/views/"+id, "", id))
	if rec.Code != http.StatusOK || decodeView(t, rec).ID != created.ID {
		t.Fatalf("obtener vista propia: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}

	// Otro usuario no puede leer, editar ni borrar la vista, aunque sepa el id.
	rec = httptest.NewRecorder()
	s.handleGetView(rec, requestAs(other, http.MethodGet, "/api/v1/views/"+id, "", id))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("otro usuario leyó la vista: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleUpdateView(rec, requestAs(other, http.MethodPut, "/api/v1/views/"+id,
		`{"name":"Robada","kind":"hosts"}`, id))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("otro usuario editó la vista: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleDeleteView(rec, requestAs(other, http.MethodDelete, "/api/v1/views/"+id, "", id))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("otro usuario borró la vista: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleListViews(rec, requestAs(other, http.MethodGet, "/api/v1/views", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("listar como otro usuario: código %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Total != 4 {
		t.Fatalf("el otro usuario vio vistas ajenas: %+v", list)
	}

	// Los roles de solo lectura pueden guardar sus propias vistas.
	rec = httptest.NewRecorder()
	s.handleCreateView(rec, requestAs(other, http.MethodPost, "/api/v1/views",
		`{"name":"Solo lectura","kind":"hosts"}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("viewer no pudo guardar su vista: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleUpdateView(rec, requestAs(owner, http.MethodPut, "/api/v1/views/"+id,
		`{"name":"Críticas edited","kind":"alerts","severity":"critical","status":"active","sort":"created_at"}`, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("actualizar vista: código %d cuerpo=%s", rec.Code, rec.Body.String())
	}
	if got := decodeView(t, rec); got.Name != "Críticas edited" {
		t.Fatalf("no se actualizó el nombre: %+v", got)
	}

	rec = httptest.NewRecorder()
	s.handleDeleteView(rec, requestAs(owner, http.MethodDelete, "/api/v1/views/"+id, "", id))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("eliminar vista: código %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleGetView(rec, requestAs(owner, http.MethodGet, "/api/v1/views/"+id, "", id))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("la vista eliminada sigue accessible: %d", rec.Code)
	}
}

func TestViewsHandlersValidationAndAuth(t *testing.T) {
	s := newMetricsServer(t)
	u := &model.User{ID: 33, Username: "admin", Role: "admin"}

	cases := []struct {
		name, body string
		want       int
		code       string
	}{
		{"sin nombre", `{"kind":"hosts"}`, http.StatusBadRequest, "invalid_view"},
		{"tipo inválido", `{"name":"x","kind":"tablas"}`, http.StatusBadRequest, "invalid_view"},
		{"nombre largo", `{"name":"` + strings.Repeat("a", 81) + `","kind":"hosts"}`, http.StatusBadRequest, "invalid_view"},
		{"columna inyectada", `{"name":"x","kind":"hosts","columns":["ip; DROP TABLE hosts"]}`, http.StatusBadRequest, "invalid_view"},
		{"sede inexistente", `{"name":"x","kind":"hosts","site_id":999}`, http.StatusBadRequest, "invalid_view"},
		{"cuerpo inválido", `{`, http.StatusBadRequest, "invalid_body"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		s.handleCreateView(rec, requestAs(u, http.MethodPost, "/api/v1/views", c.body, ""))
		if rec.Code != c.want {
			t.Errorf("%s: código %d, se esperaba %d (%s)", c.name, rec.Code, c.want, rec.Body.String())
			continue
		}
		if !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
			t.Errorf("%s: falta el código %s en %s", c.name, c.code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	s.handleCreateView(rec, requestAs(nil, http.MethodPost, "/api/v1/views", `{"name":"x","kind":"hosts"}`, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin usuario se creó una vista: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handleGetView(rec, requestAs(nil, http.MethodGet, "/api/v1/views/1", "", "1"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin usuario se leyó una vista: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleCreateView(rec, requestAs(u, http.MethodPost, "/api/v1/views",
		`{"name":"<script>x</script>","kind":"hosts","query":"<img src=x>"}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("no se aceptó el nombre escapado: %s", rec.Body.String())
	}
	v := decodeView(t, rec)
	if strings.Contains(rec.Body.String(), "<script>") || strings.Contains(v.Query, "<img") {
		t.Fatalf("HTML sin escapar en la respuesta: %s", rec.Body.String())
	}
}

func TestViewsHandlersCatalogShareAndResolve(t *testing.T) {
	s := newMetricsServer(t)
	u := &model.User{ID: 44, Username: "op", Role: "operator"}

	rec := httptest.NewRecorder()
	s.handleViewCatalog(rec, requestAs(u, http.MethodGet, "/api/v1/views/catalog", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("catálogo: código %d", rec.Code)
	}
	var catalog struct {
		Defaults []store.SavedView   `json:"defaults"`
		Columns  map[string][]string `json:"columns"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Defaults) != 4 || len(catalog.Columns[store.ViewKindHosts]) == 0 {
		t.Fatalf("catálogo incompleto: %+v", catalog)
	}

	rec = httptest.NewRecorder()
	s.handleCreateView(rec, requestAs(u, http.MethodPost, "/api/v1/views",
		`{"name":"Críticas & <global>","kind":"alerts","site_id":1,"query":"core & edge","severity":"critical","status":"active","window":"7d","columns":["title","severity"],"sort":"created_at","desc":true}`, ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear vista para compartir: %s", rec.Body.String())
	}
	created := decodeView(t, rec)
	id := strconv.FormatInt(created.ID, 10)

	rec = httptest.NewRecorder()
	s.handleShareView(rec, requestAs(u, http.MethodGet, "/api/v1/views/"+id+"/share", "", id))
	if rec.Code != http.StatusOK {
		t.Fatalf("compartir vista: %s", rec.Body.String())
	}
	var share struct {
		Path   string     `json:"path"`
		Params url.Values `json:"params"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#/alerts?", "kind=alerts", "severity=critical", "site_id=1", "desc=1"} {
		if !strings.Contains(share.Path, want) {
			t.Errorf("la URL compartida no contiene %q: %s", want, share.Path)
		}
	}
	if strings.Contains(share.Path, "<") || strings.Contains(share.Path, ">") {
		t.Fatalf("la URL compartida contiene HTML: %s", share.Path)
	}

	raw := strings.SplitN(share.Path, "?", 2)[1]
	// El enlace funciona para otro operador: no depende de la fila guardada.
	rec = httptest.NewRecorder()
	req := requestAs(&model.User{ID: 55, Username: "otro", Role: "viewer"}, http.MethodGet,
		"/api/v1/views/resolve?"+raw, "", "")
	s.handleResolveView(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolver enlace compartido: %s", rec.Body.String())
	}
	resolved := decodeView(t, rec)
	if resolved.Query != created.Query || resolved.Severity != "critical" || resolved.Status != "active" ||
		resolved.Window != "7d" || !resolved.Desc || resolved.SiteID != 1 {
		t.Fatalf("el enlace perdió filtros: %+v (esperado %+v)", resolved, created)
	}

	rec = httptest.NewRecorder()
	s.handleResolveView(rec, requestAs(u, http.MethodGet, "/api/v1/views/resolve?kind=bogus", "", ""))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("se resolvió un enlace inválido: %d", rec.Code)
	}
}

func TestOpenAPIIncludesSavedViews(t *testing.T) {
	spec := buildOpenAPI("1.5.5", "abc", "2026-11-18")
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("falta paths en OpenAPI")
	}
	for _, p := range []string{
		"/api/v1/views", "/api/v1/views/catalog", "/api/v1/views/resolve",
		"/api/v1/views/{id}", "/api/v1/views/{id}/share",
	} {
		if _, ok := paths[p]; !ok {
			t.Errorf("falta la ruta %s en OpenAPI", p)
		}
	}
	components := spec["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{"SavedView", "SavedViewRequest", "ViewShare"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("falta el esquema %s", name)
		}
	}
	item := paths["/api/v1/views"].(map[string]any)
	get := item["get"].(map[string]any)
	if len(get["security"].([]map[string]any)) == 0 {
		t.Error("listar vistas debería exigir sesión")
	}
}
