package app

import (
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// recordSNMPDeep persists the IP-MIB/Q-BRIDGE evidence collected during a scan
// and evaluates the SNMP heuristics against the retained history.
func (a *App) recordSNMPDeep(siteID, scanID int64, hosts []model.Host, at time.Time) {
	cfg := config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, SNMPHistoryDays: 30}
	if a.cfg != nil {
		cfg = a.cfg.Diagnostics
	}
	if !cfg.SNMPDeep {
		return
	}
	saved := 0
	for _, h := range hosts {
		if h.SNMP == nil || h.IP == "" {
			continue
		}
		deep := h.SNMP.Deep
		if len(deep.IPAddresses)+len(deep.ARP)+len(deep.VLANs)+len(deep.FDB) == 0 {
			continue
		}
		if err := a.st.SaveSNMPDeep(siteID, scanID, h.IP, h.SNMP.Community, at, deep); err != nil {
			a.log.Warn("no se guardó el SNMP profundo", "host", h.IP, "error", err)
			continue
		}
		saved++
	}
	if saved > 0 {
		a.log.Debug("SNMP profundo guardado", "scan", scanID, "dispositivos", saved)
	}
	a.pruneSNMPDeep(cfg)

	// Evaluate even when this scan collected no new deep rows: retained
	// evidence can still reveal a conflict, just like the ARP and DHCP hooks.
	days := cfg.SNMPHistoryDays
	if days < 1 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)
	snap, err := a.st.ListSNMPDeep(siteID, since, 50000)
	if err != nil {
		return
	}
	arp, err := a.st.ListARPObservations(siteID, since, 50000)
	if err != nil {
		arp = nil
	}
	neighbors, err := a.st.NeighborsForSite(siteID)
	if err != nil {
		neighbors = nil
	}
	findings := diagnostics.DetectSNMP(snap, arp, hosts, neighbors, cfg)
	if len(findings) > 0 {
		a.alerts.EvaluateAnomalies(findings, siteID)
	}
}

// pruneSNMPDeep keeps the raw tables inside the configured retention window.
func (a *App) pruneSNMPDeep(cfg config.Diagnostics) {
	days := cfg.SNMPHistoryDays
	if days < 1 {
		days = 30
	}
	if err := a.st.PurgeSNMPDeep(time.Now().AddDate(0, 0, -days)); err != nil {
		a.log.Warn("no se pudo podar el histórico SNMP", "error", err)
	}
}
