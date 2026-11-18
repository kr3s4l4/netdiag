package vpn

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// Manager observes local VPN endpoints periodically and persists them. All
// findings are heuristic and carry evidence + confidence; no VPN payload is
// decrypted or modified.
type Manager struct {
	st  *store.Store
	log *slog.Logger
	det Detector

	mu  sync.RWMutex
	cfg config.VPN
}

// New builds a VPN manager.
func New(st *store.Store, cfg config.VPN, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{st: st, log: log, cfg: cfg}
}

// UpdateConfig swaps live settings.
func (m *Manager) UpdateConfig(cfg config.VPN) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

func (m *Manager) settings() config.VPN {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Scan runs one observation pass for a site and returns the number of stored
// tunnels. It is safe to call concurrently with the periodic loop.
func (m *Manager) Scan(siteID int64) (int, error) {
	cfg := m.settings()
	obs := m.det.Observe()
	stored := 0
	for _, o := range obs {
		if o.Confidence < cfg.MinConfidence {
			continue
		}
		if siteID <= 0 {
			siteID = 1
		}
		t := &model.VPNTunnel{
			SiteID:     siteID,
			Type:       o.Type,
			LocalIP:    o.LocalIP,
			RemoteIP:   o.RemoteIP,
			LocalPort:  o.LocalPort,
			RemotePort: o.RemotePort,
			BytesSent:  o.BytesSent,
			BytesRecv:  o.BytesRecv,
			State:      o.State,
			Evidence:   o.Evidence,
			Confidence: o.Confidence,
		}
		if err := m.st.UpsertVPNTunnel(t); err != nil {
			return stored, err
		}
		stored++
	}
	cut := time.Now().Add(-cfg.StaleAfter.Duration())
	if _, err := m.st.MarkStaleVPNTunnels(cut); err != nil {
		return stored, err
	}
	return stored, nil
}

// Start runs the periodic observation loop until ctx is cancelled. A disabled
// config stops scheduling but does not tear down an already running loop.
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	next := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cfg := m.settings()
			if !cfg.Enabled || now.Before(next) {
				continue
			}
			interval := cfg.Interval.Duration()
			if interval < 30*time.Second {
				interval = 30 * time.Second
			}
			next = now.Add(interval)
			if _, err := m.Scan(1); err != nil {
				m.log.Warn("observación VPN fallida", "error", err)
			}
		}
	}
}
