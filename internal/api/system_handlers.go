package api

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/topology"
)

func (s *Server) systemInterfaces() ([]model.SystemInterface, error) {
	ifaces, err := netutil.Interfaces()
	if err != nil {
		return nil, err
	}
	for i := range ifaces {
		ifaces[i].Gateway = netutil.GatewayFor(ifaces[i].Name, ifaces)
	}
	return ifaces, nil
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	ifaces, _ := s.systemInterfaces()
	info := model.SystemInfo{
		Version:    s.version,
		Commit:     s.commit,
		BuildDate:  s.buildDate,
		GoVersion:  runtime.Version(),
		Hostname:   hostname,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		UptimeSecs: int64(time.Since(s.startedAt).Seconds()),
		StartedAt:  s.startedAt,
		DataDir:    s.cfg.Storage.DataDir,
		DBSize:     s.dbSize(),
		Interfaces: ifaces,
		Capabilities: map[string]bool{
			"active_arp":  discover.ARPAvailable(),
			"passive_arp": true,
			"icmp":        true,
			"snmp":        true,
			"alerts":      s.cfg.Alerts.Enabled,
			"auth":        s.cfg.Security.Auth,
			"monitor":     s.cfg.Monitor.Enabled,
			"vpn":         s.cfg.VPN.Enabled,
		},
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	cfg := *s.cfg
	if cfg.Scan.SNMPv3Auth != "" {
		cfg.Scan.SNMPv3Auth = maskedSecret
	}
	if cfg.Scan.SNMPv3Priv != "" {
		cfg.Scan.SNMPv3Priv = maskedSecret
	}
	if cfg.Security.MetricsToken != "" {
		cfg.Security.MetricsToken = maskedSecret
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config": cfg,
		"path":   s.configPath,
	})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var incoming config.Config
	if err := decode(r, &incoming); err != nil {
		s.fail(w, r, http.StatusBadRequest, "configuración inválida: "+err.Error())
		return
	}
	// Preserve masked SNMPv3 secrets.
	if incoming.Scan.SNMPv3Auth == maskedSecret || incoming.Scan.SNMPv3Auth == "" {
		incoming.Scan.SNMPv3Auth = s.cfg.Scan.SNMPv3Auth
	}
	if incoming.Scan.SNMPv3Priv == maskedSecret || incoming.Scan.SNMPv3Priv == "" {
		incoming.Scan.SNMPv3Priv = s.cfg.Scan.SNMPv3Priv
	}
	if incoming.Security.MetricsToken == maskedSecret || incoming.Security.MetricsToken == "" {
		incoming.Security.MetricsToken = s.cfg.Security.MetricsToken
	}
	if err := incoming.Normalize(); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	path := s.configPath
	if path == "" {
		path = config.DefaultPath()
	}
	if err := incoming.Save(path); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo guardar la configuración: "+err.Error())
		return
	}
	if s.scanner != nil {
		if err := s.scanner.UpdateConfig(&incoming); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "configuración guardada, pero no se pudo aplicar: "+err.Error())
			return
		}
	}
	s.SetConfig(&incoming)
	s.audit(r, "config.updated", path, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	from, to := parseWindow(r, 7*24*time.Hour)
	f := model.AuditFilter{
		Actor:  queryString(r, "actor"),
		Action: queryString(r, "action"),
		Page:   queryInt(r, "page", 1),
		Limit:  queryInt(r, "limit", 100),
	}
	if queryString(r, "from") != "" {
		f.From = &from
	}
	if queryString(r, "to") != "" {
		f.To = &to
	}
	res, err := s.st.ListAudit(f)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt64(r, "site_id")
	hosts, err := s.st.ListAllHostsForSite(siteID, 20000)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo leer el inventario: "+err.Error())
		return
	}
	neighbors, err := s.st.AllNeighbors()
	if err != nil {
		neighbors = map[int64][]model.Neighbor{}
	}
	var networks []model.Network
	if siteID > 0 {
		networks, err = s.st.ListNetworksBySite(siteID)
	} else {
		networks, err = s.st.ListNetworks()
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	// Optional filter by network.
	if nid := queryInt64(r, "network_id"); nid > 0 {
		var filtered []model.Host
		for _, h := range hosts {
			if h.NetworkID == nid {
				filtered = append(filtered, h)
			}
		}
		var nets []model.Network
		for _, n := range networks {
			if n.ID == nid {
				nets = append(nets, n)
			}
		}
		hosts, networks = filtered, nets
	}
	graph := topology.Build(hosts, neighbors, networks)
	writeJSON(w, http.StatusOK, graph)
}
