package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceHandlerRejectsIPv6(t *testing.T) {
	s := newMetricsServer(t)
	body, _ := json.Marshal(TraceRequest{Target: "::1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trace", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleTrace(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("código %d, cuerpo %s", rec.Code, rec.Body.String())
	}
}

func TestTraceHandlerRequiresTarget(t *testing.T) {
	s := newMetricsServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trace", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleTrace(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("código %d", rec.Code)
	}
}

func TestListNeighborNetworksEmpty(t *testing.T) {
	s := newMetricsServer(t)
	rec := httptest.NewRecorder()
	s.handleListNeighborNetworks(rec, httptest.NewRequest(http.MethodGet, "/api/v1/discover/neighbors", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	var out struct {
		Networks []any `json:"networks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Networks == nil || len(out.Networks) != 0 {
		t.Fatalf("se esperaba lista vacía: %s", rec.Body.String())
	}
}
