package app

import (
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// recordDHCPObservations persists the replies of the active UDP probe and runs
// the DHCP heuristics against the retained history. A scan with no replies is
// still useful: older observations can reveal a server that disappeared.
func (a *App) recordDHCPObservations(siteID, scanID int64, result *discover.Result, at time.Time) {
	if result == nil {
		return
	}
	cfg := config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, DHCPHistoryDays: 30}
	if a.cfg != nil {
		cfg = a.cfg.Diagnostics
	}
	if result.DHCPProbeError != "" {
		a.log.Debug("sondeo DHCP sin datos", "scan", scanID, "error", result.DHCPProbeError)
	}

	if len(result.DHCPObservations) > 0 {
		obs := make([]model.DHCPObservation, 0, len(result.DHCPObservations))
		for _, o := range result.DHCPObservations {
			o.SiteID = siteID
			o.ScanID = scanID
			o.ObservedAt = at
			obs = append(obs, o)
		}
		if err := a.st.SaveDHCPObservations(obs); err != nil {
			a.log.Warn("no se guardaron las observaciones DHCP", "scan", scanID, "error", err)
		}
		a.pruneDHCPObservations(cfg)
	}

	days := cfg.DHCPHistoryDays
	if days < 1 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)
	history, err := a.st.ListDHCPObservations(siteID, since, 50000)
	if err != nil {
		return
	}
	arpHistory, err := a.st.ListARPObservations(siteID, since, 50000)
	if err != nil {
		arpHistory = nil
	}
	findings := diagnostics.DetectDHCP(history, arpHistory, cfg)
	if len(findings) > 0 {
		a.alerts.EvaluateAnomalies(findings, siteID)
	}
}

func (a *App) pruneDHCPObservations(cfg config.Diagnostics) {
	days := cfg.DHCPHistoryDays
	if days < 1 {
		days = 30
	}
	if err := a.st.PurgeDHCPObservations(time.Now().AddDate(0, 0, -days)); err != nil {
		a.log.Warn("no se pudo podar el histórico DHCP", "error", err)
	}
}
