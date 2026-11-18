package api

import (
	"net/http"

	"github.com/kr3s4l4/netdiag/internal/config"
)

// handleListVPNTunnels returns the latest heuristic VPN observations.
func (s *Server) handleListVPNTunnels(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListVPNTunnels(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	cfg := config.VPN{}
	if s.cfg != nil {
		cfg = s.cfg.VPN
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
		"config": map[string]any{
			"enabled":        cfg.Enabled,
			"interval":       cfg.Interval.Duration().String(),
			"min_confidence": cfg.MinConfidence,
			"stale_after":    cfg.StaleAfter.Duration().String(),
		},
	})
}

// handleVPNScan triggers an immediate passive observation pass. Operators may
// refresh observations; changing the VPN configuration remains admin-only.
func (s *Server) handleVPNScan(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	if s.vpn == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "detector VPN no disponible")
		return
	}
	stored, err := s.vpn.Scan(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "vpn.scan", "vpn", map[string]any{"stored": stored})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stored": stored})
}

// handleDeleteVPNTunnel removes a stale or false-positive observation.
func (s *Server) handleDeleteVPNTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.DeleteVPNTunnel(id); err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "túnel no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "vpn.tunnel.deleted", "vpn", map[string]any{"id": id})
	w.WriteHeader(http.StatusNoContent)
}
