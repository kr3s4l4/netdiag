package api

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/fingerprint"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt64(r, "site_id")
	res, err := s.st.ListScans(model.ScanFilter{
		SiteID: siteID,
		Status: queryString(r, "status"),
		Page:   queryInt(r, "page", 1),
		Limit:  queryInt(r, "limit", 50),
	})
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo listar el historial: "+err.Error())
		return
	}
	active, _ := s.st.ActiveScansForSite(siteID)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  res.Items,
		"total":  res.Total,
		"page":   res.Page,
		"limit":  res.Limit,
		"active": active,
	})
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	scan, err := s.st.GetScan(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "escaneo no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req ScanRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	switch req.Profile {
	case "", "quick", "common", "full", "custom":
	default:
		s.fail(w, r, http.StatusBadRequest, "perfil inválido; usa quick, common, full o custom")
		return
	}
	if req.Profile == "custom" && len(req.Ports) == 0 {
		s.fail(w, r, http.StatusBadRequest, "el perfil custom requiere una lista de puertos")
		return
	}
	if req.NetworkID > 0 {
		n, err := s.st.GetNetwork(req.NetworkID)
		if err != nil {
			s.fail(w, r, http.StatusNotFound, "red no encontrada")
			return
		}
		if !n.Enabled {
			s.fail(w, r, http.StatusConflict, "la red está deshabilitada")
			return
		}
		if req.SiteID > 0 && req.SiteID != n.SiteID {
			s.fail(w, r, http.StatusBadRequest, "la red pertenece a otra sede")
			return
		}
		req.SiteID = n.SiteID
		if req.CIDR == "" {
			req.CIDR = n.CIDR
		}
	}
	if req.SiteID == 0 {
		req.SiteID = 1
	}
	if !s.st.SiteExists(req.SiteID) {
		s.fail(w, r, http.StatusBadRequest, "la sede indicada no existe")
		return
	}
	if strings.TrimSpace(req.CIDR) == "" {
		s.fail(w, r, http.StatusBadRequest, "indica una red o un CIDR para escanear")
		return
	}
	cidr := strings.TrimSpace(req.CIDR)
	if !strings.Contains(cidr, "/") {
		cidr += "/32"
	}
	if _, err := netip.ParsePrefix(cidr); err != nil {
		s.fail(w, r, http.StatusBadRequest, "CIDR inválido: "+err.Error())
		return
	}
	req.CIDR = cidr
	if len(req.Ports) > 0 {
		if _, err := fingerprint.ParsePorts(portsToString(req.Ports)); err != nil {
			s.fail(w, r, http.StatusBadRequest, "lista de puertos inválida: "+err.Error())
			return
		}
	}
	scan, err := s.scanner.StartScan(req, s.actorName(r))
	if err != nil {
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "scan.started", scan.CIDR, map[string]any{"scan_id": scan.ID, "profile": scan.Profile})
	s.hub.Broadcast("scan.started", scan)
	s.created(w, scan)
}

func (s *Server) handleCancelScan(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.scanner.CancelScan(id); err != nil {
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "scan.canceled", "scan", map[string]any{"scan_id": id})
	s.hub.Broadcast("scan.canceled", map[string]any{"id": id})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func portsToString(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.Join(parts, ",")
}
