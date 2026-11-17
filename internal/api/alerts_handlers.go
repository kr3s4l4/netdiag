package api

import (
	"net/http"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

var channelTypes = []string{"webhook", "slack", "discord", "telegram", "pagerduty", "smtp", "syslog"}

const maskedSecret = "********"

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	from, to := parseWindow(r, 7*24*3600e9)
	f := model.AlertFilter{
		Status:   queryString(r, "status"),
		Severity: queryString(r, "severity"),
		HostID:   queryInt64(r, "host_id"),
		Search:   queryString(r, "search"),
		Page:     queryInt(r, "page", 1),
		Limit:    queryInt(r, "limit", 50),
	}
	if v := queryString(r, "from"); v != "" {
		f.From = &from
	}
	if v := queryString(r, "to"); v != "" {
		f.To = &to
	}
	res, err := s.st.ListAlerts(f)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar las alertas: "+err.Error())
		return
	}
	stats, _ := s.st.AlertStats()
	writeJSON(w, http.StatusOK, map[string]any{
		"items": res.Items,
		"total": res.Total,
		"page":  res.Page,
		"limit": res.Limit,
		"stats": stats,
	})
}

func (s *Server) handleAlertStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.st.AlertStats()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	s.setAlertStatus(w, r, model.AlertAcked)
}

func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	s.setAlertStatus(w, r, model.AlertResolved)
}

func (s *Server) setAlertStatus(w http.ResponseWriter, r *http.Request, status string) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.SetAlertStatus(id, status); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	alert, err := s.st.GetAlert(id)
	if err == nil {
		s.hub.Broadcast("alert.updated", alert)
	}
	s.audit(r, "alert."+status, "alert", map[string]any{"alert_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

// ---- rules ----

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.st.ListRules()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar las reglas: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

type ruleRequest struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Enabled     *bool          `json:"enabled"`
	Severity    string         `json:"severity"`
	Config      map[string]any `json:"config"`
	ChannelIDs  []int64        `json:"channel_ids"`
	CooldownSec *int           `json:"cooldown_sec"`
}

func (s *Server) validateRule(req *ruleRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "el nombre de la regla es obligatorio"
	}
	if !store.ValidateRuleType(req.Type) {
		return "tipo de regla inválido"
	}
	switch req.Severity {
	case "", model.SeverityInfo, model.SeverityWarning, model.SeverityCritical:
	default:
		return "severidad inválida"
	}
	if req.CooldownSec != nil && (*req.CooldownSec < 0 || *req.CooldownSec > 604800) {
		return "el cooldown debe estar entre 0 y 604800 segundos"
	}
	return ""
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req ruleRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if msg := s.validateRule(&req); msg != "" {
		s.fail(w, r, http.StatusBadRequest, msg)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	severity := req.Severity
	if severity == "" {
		severity = model.SeverityWarning
	}
	cooldown := 900
	if req.CooldownSec != nil {
		cooldown = *req.CooldownSec
	}
	rule := &model.AlertRule{
		Name:        req.Name,
		Type:        req.Type,
		Enabled:     enabled,
		Severity:    severity,
		Config:      req.Config,
		ChannelIDs:  req.ChannelIDs,
		CooldownSec: cooldown,
	}
	created, err := s.st.CreateRule(rule)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.alerts.InvalidateRules()
	s.audit(r, "rule.created", req.Name, nil)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rule, err := s.st.GetRule(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "regla no encontrada")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	var req ruleRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if msg := s.validateRule(&req); msg != "" {
		s.fail(w, r, http.StatusBadRequest, msg)
		return
	}
	rule.Name = req.Name
	rule.Type = req.Type
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Severity != "" {
		rule.Severity = req.Severity
	}
	if req.Config != nil {
		rule.Config = req.Config
	}
	rule.ChannelIDs = req.ChannelIDs
	if req.CooldownSec != nil {
		rule.CooldownSec = *req.CooldownSec
	}
	if err := s.st.UpdateRule(rule); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.alerts.InvalidateRules()
	s.audit(r, "rule.updated", rule.Name, nil)
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rule, err := s.st.GetRule(id)
	if err != nil {
		s.fail(w, r, http.StatusNotFound, "regla no encontrada")
		return
	}
	if rule.Builtin {
		s.fail(w, r, http.StatusBadRequest, "las reglas integradas no se eliminan; deshabilítalas")
		return
	}
	if err := s.st.DeleteRule(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.alerts.InvalidateRules()
	s.audit(r, "rule.deleted", rule.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---- channels ----

func maskChannel(ch model.AlertChannel) model.AlertChannel {
	cfg := make(map[string]any, len(ch.Config))
	for k, v := range ch.Config {
		cfg[k] = v
	}
	for _, field := range store.ChannelSecretFields(ch.Type) {
		if v, ok := cfg[field]; ok {
			if s, _ := v.(string); s != "" {
				cfg[field] = maskedSecret
			}
		}
	}
	ch.Config = cfg
	return ch
}

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.st.ListChannels()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar los canales: "+err.Error())
		return
	}
	masked := make([]model.AlertChannel, 0, len(channels))
	for _, ch := range channels {
		masked = append(masked, maskChannel(ch))
	}
	secretFields := map[string][]string{}
	for _, t := range channelTypes {
		secretFields[t] = store.ChannelSecretFields(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":         masked,
		"types":         channelTypes,
		"secret_fields": secretFields,
	})
}

type channelRequest struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Enabled *bool          `json:"enabled"`
	Config  map[string]any `json:"config"`
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req channelRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		s.fail(w, r, http.StatusBadRequest, "el nombre del canal es obligatorio")
		return
	}
	if !store.ValidateChannelType(req.Type) {
		s.fail(w, r, http.StatusBadRequest, "tipo de canal inválido")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch := &model.AlertChannel{Name: req.Name, Type: req.Type, Enabled: enabled, Config: req.Config}
	created, err := s.st.CreateChannel(ch)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "channel.created", req.Name, map[string]any{"type": req.Type})
	writeJSON(w, http.StatusCreated, maskChannel(*created))
}

func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	ch, err := s.st.GetChannel(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "canal no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	var req channelRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if req.Name != "" {
		ch.Name = strings.TrimSpace(req.Name)
	}
	if req.Type != "" && req.Type != ch.Type {
		if !store.ValidateChannelType(req.Type) {
			s.fail(w, r, http.StatusBadRequest, "tipo de canal inválido")
			return
		}
		ch.Type = req.Type
	}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if req.Config != nil {
		merged := make(map[string]any, len(ch.Config)+len(req.Config))
		for k, v := range ch.Config {
			merged[k] = v
		}
		for k, v := range req.Config {
			merged[k] = v
		}
		// Masked or blank secrets mean "keep the stored value".
		for _, field := range store.ChannelSecretFields(ch.Type) {
			if v, ok := merged[field]; ok {
				if str, _ := v.(string); str == "" || str == maskedSecret {
					if old, ok := ch.Config[field]; ok {
						merged[field] = old
					} else {
						delete(merged, field)
					}
				}
			}
		}
		ch.Config = merged
	}
	if err := s.st.UpdateChannel(ch); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.st.GetChannel(id)
	if err != nil {
		updated = ch
	}
	s.audit(r, "channel.updated", ch.Name, nil)
	writeJSON(w, http.StatusOK, maskChannel(*updated))
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	ch, _ := s.st.GetChannel(id)
	if err := s.st.DeleteChannel(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	name := ""
	if ch != nil {
		name = ch.Name
	}
	s.audit(r, "channel.deleted", name, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.alerts.TestChannel(id); err != nil {
		s.audit(r, "channel.test_failed", "channel", map[string]any{"channel_id": id, "error": err.Error()})
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.audit(r, "channel.test", "channel", map[string]any{"channel_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
