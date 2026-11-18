package api

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
)

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt64(r, "site_id")
	var nets []model.Network
	var err error
	if siteID > 0 {
		nets, err = s.st.ListNetworksBySite(siteID)
	} else {
		nets, err = s.st.ListNetworks()
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar las redes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nets})
}

type networkRequest struct {
	SiteID    int64  `json:"site_id,omitempty"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Interface string `json:"interface"`
	Enabled   *bool  `json:"enabled"`
}

func (s *Server) validateNetwork(req networkRequest) (string, int, error) {
	cidr := strings.TrimSpace(req.CIDR)
	if !strings.Contains(cidr, "/") {
		cidr += "/32"
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", 0, err
	}
	if !prefix.Addr().Is4() {
		return "", 0, errNotIPv4
	}
	prefix = prefix.Masked()
	capacity, err := netutil.CountHosts(prefix.String())
	if err != nil {
		return "", 0, err
	}
	if !s.cfg.Scan.AllowPublic {
		if !netutil.IsPrivate(prefix.Addr().AsSlice()) {
			return "", 0, errPublicCIDR
		}
	}
	return prefix.String(), capacity, nil
}

var (
	errNotIPv4    = errText("solo se admiten redes IPv4")
	errPublicCIDR = errText("la red es pública; activa scan.allow_public para inventariarla")
)

type errText string

func (e errText) Error() string { return string(e) }

func (s *Server) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req networkRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	cidr, capacity, err := s.validateNetwork(req)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	siteID := req.SiteID
	if siteID == 0 {
		siteID = 1
	}
	if siteID <= 0 || !s.st.SiteExists(siteID) {
		s.fail(w, r, http.StatusBadRequest, "la sede indicada no existe")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = cidr
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	n := &model.Network{
		SiteID:    siteID,
		Name:      name,
		CIDR:      cidr,
		Interface: strings.TrimSpace(req.Interface),
		Enabled:   enabled,
		CreatedAt: time.Now(),
	}
	created, err := s.st.CreateNetwork(n)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			s.fail(w, r, http.StatusConflict, "ya existe una red con ese CIDR")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	created.HostCount = capacity
	s.audit(r, "network.created", cidr, map[string]any{"name": name})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateNetwork(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	existing, err := s.st.GetNetwork(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "red no encontrada")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	var req networkRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	cidr, _, err := s.validateNetwork(req)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	existing.CIDR = cidr
	if req.SiteID > 0 && req.SiteID != existing.SiteID {
		if !s.st.SiteExists(req.SiteID) {
			s.fail(w, r, http.StatusBadRequest, "la sede indicada no existe")
			return
		}
		existing.SiteID = req.SiteID
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		existing.Name = name
	}
	if req.Interface != "" {
		existing.Interface = strings.TrimSpace(req.Interface)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := s.st.UpdateNetwork(existing); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			s.fail(w, r, http.StatusConflict, "ya existe una red con ese CIDR")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "network.updated", cidr, nil)
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	n, _ := s.st.GetNetwork(id)
	if err := s.st.DeleteNetwork(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	target := strconv.FormatInt(id, 10)
	if n != nil {
		target = n.CIDR
	}
	s.audit(r, "network.deleted", target, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSuggestNetworks(w http.ResponseWriter, r *http.Request) {
	suggested, err := netutil.SuggestNetworks()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	ifaces, _ := s.systemInterfaces()
	siteID := queryInt64(r, "site_id")
	known := map[string]bool{}
	var nets []model.Network
	if siteID > 0 {
		nets, err = s.st.ListNetworksBySite(siteID)
	} else {
		nets, err = s.st.ListNetworks()
	}
	if err == nil {
		for _, n := range nets {
			known[n.CIDR] = true
		}
	}
	type suggestion struct {
		model.Network
		AlreadyAdded bool `json:"already_added"`
	}
	out := make([]suggestion, 0, len(suggested))
	for _, n := range suggested {
		if siteID > 0 {
			n.SiteID = siteID
		}
		out = append(out, suggestion{Network: n, AlreadyAdded: known[n.CIDR]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "interfaces": ifaces})
}
