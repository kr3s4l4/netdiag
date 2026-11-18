package app

import (
	"context"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/api"
	"github.com/kr3s4l4/netdiag/internal/history"
)

// historyLoop captures compact inventory/topology snapshots hourly so temporal
// comparisons do not depend exclusively on scan activity.
func (a *App) historyLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	a.captureHistorySnapshots()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.captureHistorySnapshots()
		}
	}
}

func (a *App) captureHistorySnapshots() {
	sites, err := a.st.ListSites()
	if err != nil {
		a.log.Warn("no se pudieron listar las sedes para el histórico", "error", err)
		return
	}
	for _, site := range sites {
		if err := history.Capture(a.st, site.ID); err != nil {
			a.log.Warn("no se pudo capturar el snapshot histórico", "sede", site.ID, "error", err)
		}
	}
}

// monitorLoop periodically scans enabled networks. It re-reads the live
// configuration every tick so interval and enablement changes apply without a
// restart.
func (a *App) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var next time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg := a.currentCfg()
			if !cfg.Monitor.Enabled {
				next = time.Time{}
				continue
			}
			interval := cfg.Monitor.Interval.Duration()
			if interval < 30*time.Second {
				interval = 30 * time.Second
			}
			now := time.Now()
			if next.IsZero() {
				next = now.Add(interval)
				continue
			}
			if now.Before(next) {
				continue
			}
			next = now.Add(interval)
			a.scanAll()
		}
	}
}

func (a *App) scanAll() {
	cfg := a.currentCfg()
	nets, err := a.st.ListNetworks()
	if err != nil {
		a.log.Warn("no se pudieron listar las redes para el monitoreo", "error", err)
		return
	}
	allowed := make(map[int64]bool, len(cfg.Monitor.NetworkIDs))
	for _, id := range cfg.Monitor.NetworkIDs {
		allowed[id] = true
	}
	deepEvery := cfg.Monitor.DeepInterval.Duration()
	for _, n := range nets {
		if !n.Enabled {
			continue
		}
		if len(allowed) > 0 && !allowed[n.ID] {
			continue
		}
		profile := cfg.Scan.Profile
		if deepEvery > 0 {
			a.mu.Lock()
			last, seen := a.deepLast[n.ID]
			if !seen {
				a.deepLast[n.ID] = time.Now()
			} else if time.Since(last) >= deepEvery {
				profile = "full"
				a.deepLast[n.ID] = time.Now()
			}
			a.mu.Unlock()
		}
		_, err := a.StartScan(api.ScanRequest{
			NetworkID: n.ID,
			CIDR:      n.CIDR,
			Profile:   profile,
		}, "monitor")
		if err != nil && !strings.Contains(err.Error(), "en curso") {
			a.log.Warn("no se pudo iniciar el escaneo programado", "red", n.CIDR, "error", err)
		}
	}
}
