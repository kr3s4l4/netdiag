package wifi

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// ScanResult summarizes one passive capture pass.
type ScanResult struct {
	APs        int   `json:"aps"`
	Clients    int   `json:"clients"`
	Frames     int   `json:"frames"`
	DurationMS int64 `json:"duration_ms"`
}

// Manager owns passive 802.11 capture and persistence. The monitor interface
// must already be in monitor mode; NetDiag never changes the device mode.
type Manager struct {
	st  *store.Store
	log *slog.Logger

	mu   sync.RWMutex
	cfg  config.WiFi
	scan sync.Mutex
}

// New builds a WiFi manager.
func New(st *store.Store, cfg config.WiFi, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{st: st, cfg: cfg, log: log}
}

// UpdateConfig swaps live settings.
func (m *Manager) UpdateConfig(cfg config.WiFi) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
}

func (m *Manager) settings() config.WiFi {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Scan captures one window, aggregates frames and persists AP/client observations.
// Only one capture can use the interface at a time.
func (m *Manager) Scan(ctx context.Context, siteID int64) (ScanResult, error) {
	m.scan.Lock()
	defer m.scan.Unlock()
	cfg := m.settings()
	if cfg.Interface == "" {
		return ScanResult{}, ErrCaptureUnavailable
	}
	if siteID <= 0 {
		siteID = 1
	}
	aps := map[string]*model.WifiAP{}
	clients := map[string]*model.WifiClient{}
	frames := 0
	started := time.Now()
	err := Capture(ctx, cfg.Interface, time.Duration(cfg.ScanSeconds)*time.Second, cfg.Channels, func(pkt gopacket.Packet) {
		frames++
		apObs, clientObs := ParsePacket(pkt)
		if apObs != nil {
			ap, ok := aps[apObs.BSSID]
			if !ok {
				ap = &model.WifiAP{SiteID: siteID, BSSID: apObs.BSSID, SSID: apObs.SSID}
				aps[apObs.BSSID] = ap
			} else if apObs.SSID != "(oculta)" && (ap.SSID == "" || ap.SSID == "(oculta)") {
				ap.SSID = apObs.SSID
			}
			if apObs.SignalDBM > ap.SignalDBM {
				ap.SignalDBM = apObs.SignalDBM
			}
			ap.Channel = apObs.Channel
			ap.Band = apObs.Band
			ap.Encryption = apObs.Encryption
			ap.Beacons += apObs.Beacons
		}
		if clientObs != nil {
			client, ok := clients[clientObs.MAC]
			if !ok {
				client = &model.WifiClient{SiteID: siteID, MAC: clientObs.MAC}
				clients[clientObs.MAC] = client
			}
			if clientObs.SignalDBM > client.SignalDBM {
				client.SignalDBM = clientObs.SignalDBM
			}
			client.APBSSID = clientObs.APBSSID
		}
	})
	if err != nil {
		return ScanResult{}, err
	}
	clientPerAP := map[string]int{}
	for _, c := range clients {
		clientPerAP[c.APBSSID]++
	}
	for _, ap := range aps {
		ap.Clients = clientPerAP[ap.BSSID]
		ap.Rogue, ap.RogueInfo, ap.RogueConfidence = classifyRogue(*ap, cfg)
		if err := m.st.UpsertWifiAP(ap); err != nil {
			return ScanResult{}, err
		}
	}
	for _, c := range clients {
		if err := m.st.UpsertWifiClient(c); err != nil {
			return ScanResult{}, err
		}
	}
	result := ScanResult{
		APs:        len(aps),
		Clients:    len(clients),
		Frames:     frames,
		DurationMS: time.Since(started).Milliseconds(),
	}
	m.log.Info("captura WiFi completada", "aps", result.APs, "clientes", result.Clients,
		"tramas", result.Frames, "duracion_ms", result.DurationMS)
	return result, nil
}

// Channels aggregates the stored observations into per-channel interference data.
func (m *Manager) Channels(siteID int64) ([]model.WifiChannel, error) {
	aps, err := m.st.ListWifiAPs(siteID, 5000)
	if err != nil {
		return nil, err
	}
	clients, err := m.st.ListWifiClients(siteID, 10000)
	if err != nil {
		return nil, err
	}
	return aggregateChannels(aps, clients), nil
}

// Start runs the periodic passive capture loop until ctx is cancelled.
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
			if !cfg.Enabled || cfg.Interface == "" || now.Before(next) {
				continue
			}
			interval := cfg.Interval.Duration()
			if interval < 30*time.Second {
				interval = 30 * time.Second
			}
			next = now.Add(interval)
			if _, err := m.Scan(ctx, 1); err != nil {
				m.log.Warn("captura WiFi fallida", "error", err)
			}
		}
	}
}

// classifyRogue applies the configured reference inventory. It returns the
// flag, a human-readable reason and a confidence value (1-100). An empty
// inventory yields no verdict at all: absence of evidence is not evidence.
func classifyRogue(ap model.WifiAP, cfg config.WiFi) (bool, string, int) {
	authorized := containsFold(cfg.AuthorizedBSSIDs, ap.BSSID)
	known := ap.SSID != "" && ap.SSID != "(oculta)" && containsExact(cfg.KnownSSIDs, ap.SSID)
	if len(cfg.AuthorizedBSSIDs) == 0 && len(cfg.KnownSSIDs) == 0 {
		return false, "", 0
	}
	if len(cfg.AuthorizedBSSIDs) > 0 && authorized {
		return false, "", 0
	}
	if len(cfg.AuthorizedBSSIDs) > 0 && !authorized {
		if known {
			return true, "BSSID no figura en authorized_bssids; el SSID coincide con known_ssids pero puede suplantarse", 65
		}
		return true, "BSSID no figura en authorized_bssids", 90
	}
	if ap.SSID == "(oculta)" {
		return false, "", 0
	}
	if !known {
		return true, "SSID no figura en known_ssids", 55
	}
	return false, "", 0
}

func containsFold(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func containsExact(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// aggregateChannels groups APs/clients by band+channel and marks interference.
func aggregateChannels(aps []model.WifiAP, clients []model.WifiClient) []model.WifiChannel {
	type key struct {
		band    string
		channel int
	}
	clientCount := map[string]int{}
	for _, c := range clients {
		clientCount[c.APBSSID]++
	}
	groups := map[key]*model.WifiChannel{}
	for _, ap := range aps {
		if ap.Channel <= 0 {
			continue
		}
		band := ap.Band
		if band == "" {
			band = BandForChannel(ap.Channel)
		}
		k := key{band: band, channel: ap.Channel}
		g, ok := groups[k]
		if !ok {
			g = &model.WifiChannel{Channel: ap.Channel, Band: band}
			groups[k] = g
		}
		g.APs++
		g.Clients += clientCount[ap.BSSID]
	}
	out := make([]model.WifiChannel, 0, len(groups))
	for _, g := range groups {
		for _, ap := range aps {
			if ap.BSSID == "" || ap.Channel <= 0 {
				continue
			}
			band := ap.Band
			if band == "" {
				band = BandForChannel(ap.Channel)
			}
			if band != g.Band || ap.Channel == g.Channel {
				continue
			}
			if overlaps(g.Band, g.Channel, ap.Channel) {
				g.Overlap++
			}
		}
		g.Load = channelLoad(g.APs, g.Clients, g.Overlap)
		g.Suggest = suggestChannel(g.Band, aps)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].Band < out[j].Band
	})
	return out
}

func overlaps(band string, a, b int) bool {
	if band == "2.4GHz" {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= 4
	}
	return a == b
}

func channelLoad(aps, clients, overlap int) string {
	score := aps + clients/3 + overlap
	if score <= 2 {
		return "baja"
	}
	if score <= 6 {
		return "media"
	}
	return "alta"
}

func suggestChannel(band string, aps []model.WifiAP) string {
	candidates := standardChannels(band)
	if len(candidates) == 0 {
		return ""
	}
	best, bestScore := candidates[0], int(^uint(0)>>1)
	for _, ch := range candidates {
		score := 0
		for _, ap := range aps {
			if ap.Channel <= 0 {
				continue
			}
			apBand := ap.Band
			if apBand == "" {
				apBand = BandForChannel(ap.Channel)
			}
			if apBand == band && overlaps(band, ch, ap.Channel) {
				score++
			}
		}
		if score < bestScore {
			best, bestScore = ch, score
		}
	}
	return "canal " + itoa(best)
}

func standardChannels(band string) []int {
	switch band {
	case "2.4GHz":
		return []int{1, 6, 11}
	case "5GHz":
		return []int{36, 44, 52, 60, 100, 108, 149, 157}
	case "6GHz":
		return []int{37, 53, 69, 85, 101, 117, 133, 149, 165}
	default:
		return nil
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
