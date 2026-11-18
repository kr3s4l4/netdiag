package api

import (
	"net/http"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/wifi"
)

// handleListWifiAPs returns observed access points, strongest signal first.
func (s *Server) handleListWifiAPs(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListWifiAPs(queryInt64(r, "site_id"), queryInt(r, "limit", 1000))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// handleListWifiClients returns observed stations.
func (s *Server) handleListWifiClients(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListWifiClients(queryInt64(r, "site_id"), queryInt(r, "limit", 2000))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// handleListWifiChannels aggregates AP/client load per channel.
func (s *Server) handleListWifiChannels(w http.ResponseWriter, r *http.Request) {
	if s.wifi == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "analizador WiFi no disponible")
		return
	}
	items, err := s.wifi.Channels(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// handleWifiScan triggers one passive capture window. The interface must
// already be in monitor mode and the binary must include pcap support.
func (s *Server) handleWifiScan(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	if s.wifi == nil || !wifi.Available() {
		s.fail(w, r, http.StatusServiceUnavailable,
			"captura 802.11 no disponible: usa un binario compilado con -tags pcap y una interfaz en modo monitor")
		return
	}
	result, err := s.wifi.Scan(r.Context(), queryInt64(r, "site_id"))
	if err != nil {
		if err == wifi.ErrCaptureUnavailable {
			s.fail(w, r, http.StatusServiceUnavailable, err.Error())
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "wifi.scan", "wifi", map[string]any{"aps": result.APs, "clients": result.Clients})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

// handleClearWifiObservations drops stored APs and clients for a site.
func (s *Server) handleClearWifiObservations(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	siteID := queryInt64(r, "site_id")
	if err := s.st.ClearWifiObservations(siteID); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "wifi.observations.cleared", "wifi", map[string]any{"site_id": siteID})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleWifiStatus exposes capability and configuration without secrets.
func (s *Server) handleWifiStatus(w http.ResponseWriter, r *http.Request) {
	cfg := config.WiFi{}
	if s.cfg != nil {
		cfg = s.cfg.WiFi
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"capture_available": wifi.Available(),
		"enabled":           cfg.Enabled,
		"interface":         cfg.Interface,
		"interval":          cfg.Interval.Duration().String(),
		"scan_seconds":      cfg.ScanSeconds,
		"channels":          cfg.Channels,
		"authorized_bssids": len(cfg.AuthorizedBSSIDs),
		"known_ssids":       len(cfg.KnownSSIDs),
		"note":              "La interfaz debe estar previamente en modo monitor; NetDiag no cambia el modo del dispositivo.",
	})
}
