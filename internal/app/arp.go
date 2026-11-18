package app

import (
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// recordARPObservations stores the IP→MAC bindings found in a finished scan.
// The scanner never sees its own MAC in the results, so the history reflects
// only remote devices.
func (a *App) recordARPObservations(siteID, scanID int64, hosts []model.Host, at time.Time) {
	obs := make([]model.ARPObservation, 0, len(hosts))
	for _, h := range hosts {
		if h.IP == "" || h.MAC == "" {
			continue
		}
		obs = append(obs, model.ARPObservation{
			SiteID:     siteID,
			ScanID:     scanID,
			IP:         h.IP,
			MAC:        h.MAC,
			Hostname:   h.Hostname,
			IsGateway:  h.IsGateway,
			ObservedAt: at,
		})
	}
	if len(obs) == 0 {
		return
	}
	if err := a.st.SaveARPObservations(obs); err != nil {
		a.log.Warn("no se guardó el histórico ARP", "scan", scanID, "error", err)
		return
	}
	a.pruneARPObservations()

	cfg := config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, MACIPLimit: 5, ARPHistoryDays: 30}
	if a.cfg != nil {
		cfg = a.cfg.Diagnostics
	}
	days := cfg.ARPHistoryDays
	if days < 1 {
		days = 30
	}
	history, err := a.st.ListARPObservations(siteID, time.Now().AddDate(0, 0, -days), 50000)
	if err != nil {
		return
	}
	findings := diagnostics.DetectARP(history, cfg)
	if len(findings) > 0 {
		a.alerts.EvaluateAnomalies(findings)
	}
}

// pruneARPObservations keeps the history inside the configured retention
// window. The default is 30 days, the same window used for managed hosts.
func (a *App) pruneARPObservations() {
	days := 30
	if a.cfg != nil && a.cfg.Diagnostics.ARPHistoryDays > 0 {
		days = a.cfg.Diagnostics.ARPHistoryDays
	}
	if err := a.st.PurgeARPObservations(time.Now().AddDate(0, 0, -days)); err != nil {
		a.log.Warn("no se pudo podar el histórico ARP", "error", err)
	}
}
