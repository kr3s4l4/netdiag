package api

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type updateApplyRequest struct {
	Version string `json:"version,omitempty"`
	Restart *bool  `json:"restart,omitempty"`
}

// handleUpdateCheck exposes the signed version check to any authenticated user.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		s.failCode(w, r, http.StatusServiceUnavailable, "update_unavailable", "la comprobación de actualizaciones no está disponible")
		return
	}
	res, err := s.updates.Check(r.Context())
	if err != nil {
		s.fail(w, r, http.StatusBadGateway, "no se pudo comprobar la versión: "+err.Error())
		return
	}
	s.ok(w, res)
}

// handleUpdateApply downloads, verifies and installs a signed release. Only
// administrators may mutate the running binary. The actual restart is deferred
// until after the HTTP response so the caller can observe the result.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.updates == nil {
		s.failCode(w, r, http.StatusServiceUnavailable, "update_unavailable", "el gestor de actualizaciones no está disponible")
		return
	}
	var req updateApplyRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
			return
		}
	}
	req.Version = strings.TrimSpace(req.Version)
	restart := req.Restart == nil || *req.Restart
	result, err := s.updates.Apply(r.Context(), req.Version)
	if err != nil {
		s.fail(w, r, http.StatusBadGateway, "no se pudo aplicar la actualización: "+err.Error())
		return
	}
	s.audit(r, "update.applied", result.ToVersion, map[string]any{
		"from_version": result.FromVersion,
		"backup":       result.BackupPath,
		"restart":      restart,
	})
	s.hub.Broadcast("update.applied", result)
	if restart {
		go s.restartAfterResponse()
	}
	s.ok(w, result)
}

func (s *Server) restartAfterResponse() {
	time.Sleep(500 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.hub.Broadcast("update.restarting", map[string]any{"at": time.Now().UTC()})
	if err := s.updates.Restart(ctx); err != nil {
		s.log.Error("no se pudo reiniciar tras la actualización", "error", err)
		s.hub.Broadcast("update.restart_failed", map[string]any{"error": err.Error()})
	}
}

// handleUpdateRollback restores the previous binary and marker state.
func (s *Server) handleUpdateRollback(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.updates == nil {
		s.failCode(w, r, http.StatusServiceUnavailable, "update_unavailable", "el gestor de actualizaciones no está disponible")
		return
	}
	result, err := s.updates.Rollback()
	if err != nil {
		s.fail(w, r, http.StatusBadGateway, "no se pudo revertir la actualización: "+err.Error())
		return
	}
	s.audit(r, "update.rolled_back", result.Version, nil)
	s.ok(w, result)
}

// handleUpdateStatus returns cached version information without contacting the
// manifest server, useful for the settings card on slow or offline links.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		s.failCode(w, r, http.StatusServiceUnavailable, "update_unavailable", "el gestor de actualizaciones no está disponible")
		return
	}
	pending := s.updates.Pending()
	s.ok(w, map[string]any{
		"current_version":      s.updates.CurrentVersion(),
		"enabled":              s.cfg.Update.Enabled,
		"check_interval_hours": s.cfg.Update.CheckIntervalHours,
		"pending":              pending,
	})
}
