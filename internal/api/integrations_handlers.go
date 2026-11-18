package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

type integrationRequest struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Enabled *bool          `json:"enabled"`
	Config  map[string]any `json:"config"`
}

// maskIntegration copies an integration and replaces secret fields with a
// sentinel so clients can keep them when updating without ever reading them.
func maskIntegration(it *model.Integration) model.Integration {
	out := *it
	out.Config = map[string]any{}
	for k, v := range it.Config {
		out.Config[k] = v
	}
	for _, f := range store.IntegrationSecretFields(it.Type) {
		if _, ok := out.Config[f]; ok {
			out.Config[f] = maskedSecret
		}
	}
	return out
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListIntegrations()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]model.Integration, 0, len(items))
	for i := range items {
		out = append(out, maskIntegration(&items[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "types": store.IntegrationTypes,
		"secret_fields": map[string][]string{
			"elasticsearch": store.IntegrationSecretFields("elasticsearch"),
			"webhook":       store.IntegrationSecretFields("webhook"),
			"slack":         store.IntegrationSecretFields("slack"),
			"discord":       store.IntegrationSecretFields("discord"),
			"msteams":       store.IntegrationSecretFields("msteams"),
			"telegram":      store.IntegrationSecretFields("telegram"),
			"pagerduty":     store.IntegrationSecretFields("pagerduty"),
			"kafka":         store.IntegrationSecretFields("kafka"),
			"redis":         store.IntegrationSecretFields("redis"),
		},
	})
}

// mergeSecretFields keeps existing encrypted values when the client sends an
// empty string or the masking sentinel.
func mergeSecretFields(next, prev map[string]any, typ string) {
	for _, f := range store.IntegrationSecretFields(typ) {
		v, ok := next[f]
		if !ok {
			if old, found := prev[f]; found {
				next[f] = old
			}
			continue
		}
		str, _ := v.(string)
		if str == "" || str == maskedSecret {
			if old, found := prev[f]; found {
				next[f] = old
			} else {
				delete(next, f)
			}
		}
	}
}

func (s *Server) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req integrationRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Name == "" {
		s.fail(w, r, http.StatusBadRequest, "el nombre de la integración es obligatorio")
		return
	}
	if !store.ValidateIntegrationType(req.Type) {
		s.fail(w, r, http.StatusBadRequest, "tipo de integración no soportado")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	it := &model.Integration{Name: req.Name, Type: req.Type, Enabled: enabled, Config: req.Config}
	created, err := s.st.CreateIntegration(it)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "integration.created", req.Name, map[string]any{"type": req.Type})
	writeJSON(w, http.StatusCreated, maskIntegration(created))
}

func (s *Server) handleUpdateIntegration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	prev, err := s.st.GetIntegration(id)
	if err != nil {
		s.fail(w, r, http.StatusNotFound, "integración no encontrada")
		return
	}
	var req integrationRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if req.Name != "" {
		prev.Name = strings.TrimSpace(req.Name)
	}
	if req.Type != "" && req.Type != prev.Type {
		if !store.ValidateIntegrationType(strings.ToLower(req.Type)) {
			s.fail(w, r, http.StatusBadRequest, "tipo de integración no soportado")
			return
		}
		prev.Type = strings.ToLower(req.Type)
	}
	if req.Enabled != nil {
		prev.Enabled = *req.Enabled
	}
	if req.Config != nil {
		merged := make(map[string]any, len(prev.Config)+len(req.Config))
		for k, v := range prev.Config {
			merged[k] = v
		}
		for k, v := range req.Config {
			merged[k] = v
		}
		mergeSecretFields(merged, prev.Config, prev.Type)
		prev.Config = merged
	}
	if err := s.st.UpdateIntegration(prev); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.st.GetIntegration(id)
	if err != nil {
		updated = prev
	}
	s.audit(r, "integration.updated", prev.Name, nil)
	writeJSON(w, http.StatusOK, maskIntegration(updated))
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	it, _ := s.st.GetIntegration(id)
	if err := s.st.DeleteIntegration(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	name := ""
	if it != nil {
		name = it.Name
	}
	s.audit(r, "integration.deleted", name, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestIntegration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if s.integrations == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "integraciones no disponibles")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.integrations.Test(ctx, id); err != nil {
		s.audit(r, "integration.test_failed", "integration", map[string]any{"id": id, "error": err.Error()})
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.audit(r, "integration.test", "integration", map[string]any{"id": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
