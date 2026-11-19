package app

import (
	"context"
	"time"

	"github.com/kr3s4l4/netdiag/internal/capture"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/diagnostics"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// recordPassive runs one short metadata-only capture after a scan. It is
// opt-in: without diagnostics.passive_capture the scanner never opens a raw
// socket, and portable builds simply log that the capability is unavailable.
func (a *App) recordPassive(scan *model.Scan, networkIface string, at time.Time) {
	cfg := config.Diagnostics{MinConfidence: 60, MaxAnomalies: 200, PassiveHistoryDays: 7}
	if a.cfg != nil {
		cfg = a.cfg.Diagnostics
	}
	if !cfg.PassiveCapture {
		return
	}
	obs := make([]model.PassiveObservation, 0, 128)
	if !capture.Available() {
		// A portable build cannot capture, but retained evidence may still
		// produce findings. Keep evaluating it, just like the ARP/DHCP hooks.
		a.log.Debug("captura pasiva no compilada; se evalúa el histórico", "scan", scan.ID)
	} else {
		iface := cfg.PassiveInterface
		if iface == "" {
			iface = networkIface
		}
		if iface == "" {
			a.log.Warn("captura pasiva sin interfaz", "scan", scan.ID)
			return
		}

		ccfg := capture.Config{
			Interface:   iface,
			Duration:    time.Duration(cfg.PassiveSeconds) * time.Second,
			MaxPackets:  cfg.PassiveMaxPackets,
			Promiscuous: cfg.PassivePromiscuous,
		}
		ccfg.Normalize()
		res, err := capture.Capture(context.Background(), ccfg, func(o model.PassiveObservation) {
			o.SiteID = scan.SiteID
			o.ScanID = scan.ID
			if o.ObservedAt.IsZero() {
				o.ObservedAt = at
			}
			obs = append(obs, o)
		})
		if err != nil {
			a.log.Warn("captura pasiva no disponible", "scan", scan.ID, "iface", iface, "error", err)
		} else {
			a.log.Info("captura pasiva finalizada", "scan", scan.ID, "iface", iface,
				"paquetes", res.Packets, "observaciones", res.Observed, "bpf", res.BPFFilter)
		}
	}

	if len(obs) > 0 {
		if err := a.st.SavePassiveObservations(obs); err != nil {
			a.log.Warn("no se guardó la captura pasiva", "scan", scan.ID, "error", err)
			return
		}
	}
	days := cfg.PassiveHistoryDays
	if days < 1 {
		days = 7
	}
	if err := a.st.PurgePassiveObservations(time.Now().AddDate(0, 0, -days)); err != nil {
		a.log.Warn("no se pudo podar el histórico pasivo", "error", err)
	}

	history, err := a.st.ListPassiveObservations(scan.SiteID, time.Now().AddDate(0, 0, -days), 50000)
	if err != nil {
		return
	}
	arp, err := a.st.ListARPObservations(scan.SiteID, time.Now().AddDate(0, 0, -days), 50000)
	if err != nil {
		arp = nil
	}
	findings := diagnostics.DetectPassive(history, arp, cfg)
	if len(findings) > 0 {
		a.alerts.EvaluateAnomalies(findings, scan.SiteID)
	}
}
