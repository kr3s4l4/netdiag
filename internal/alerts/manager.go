// Package alerts evaluates alert rules and delivers notifications through
// configured channels. Delivery is asynchronous so scanning is never blocked
// by a slow webhook.
package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// Manager owns rule evaluation and delivery.
type Manager struct {
	st     *store.Store
	cfg    config.Alerts
	cfgMu  sync.RWMutex
	log    *slog.Logger
	client *HTTPClient

	mu         sync.Mutex
	rules      []model.AlertRule
	rulesAt    time.Time
	sentWindow []time.Time

	notify func(model.Alert)

	queue  chan job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type job struct {
	alert    model.Alert
	channels []model.AlertChannel
}

// New creates the manager.
func New(st *store.Store, cfg config.Alerts, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	if cfg.MaxPerMinute <= 0 {
		cfg.MaxPerMinute = 30
	}
	return &Manager{
		st:     st,
		cfg:    cfg,
		log:    log,
		client: NewHTTPClient(cfg.WebhookTimeout.Duration(), cfg.AllowPrivateTargets),
		queue:  make(chan job, 256),
	}
}

// UpdateConfig applies alert settings without restarting the process.
func (m *Manager) UpdateConfig(cfg config.Alerts) {
	if cfg.MaxPerMinute <= 0 {
		cfg.MaxPerMinute = 30
	}
	m.cfgMu.Lock()
	m.cfg = cfg
	m.client = NewHTTPClient(cfg.WebhookTimeout.Duration(), cfg.AllowPrivateTargets)
	m.cfgMu.Unlock()
}

func (m *Manager) settings() (config.Alerts, *HTTPClient) {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg, m.client
}

func (m *Manager) enabled() bool {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg.Enabled
}

func (m *Manager) allowPrivateTargets() bool {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg.AllowPrivateTargets
}

func (m *Manager) smtpInsecure() bool {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg.SMTPInsecure
}

func (m *Manager) httpClient() *HTTPClient {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.client
}

// SetNotifier registers a callback invoked for every newly created alert.
func (m *Manager) SetNotifier(fn func(model.Alert)) {
	m.mu.Lock()
	m.notify = fn
	m.mu.Unlock()
}

// Start launches delivery workers.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.ctx != nil {
		m.mu.Unlock()
		return
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()
	for i := 0; i < 2; i++ {
		m.wg.Add(1)
		go m.worker()
	}
}

// Stop drains in-flight deliveries.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.ctx = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	close(m.queue)
	m.wg.Wait()
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for j := range m.queue {
		for _, ch := range j.channels {
			if err := m.deliver(context.Background(), ch, j.alert); err != nil {
				m.log.Warn("no se pudo entregar alerta", "canal", ch.Name, "tipo", ch.Type, "error", err)
				_ = m.st.MarkChannelTest(ch.ID, err.Error())
				continue
			}
			_ = m.st.MarkChannelTest(ch.ID, "")
		}
	}
}

// Rules returns cached rules, refreshing every 20 seconds.
func (m *Manager) Rules() []model.AlertRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.rulesAt) < 20*time.Second && m.rules != nil {
		return m.rules
	}
	rules, err := m.st.ListRules()
	if err != nil {
		if m.rules != nil {
			return m.rules
		}
		return nil
	}
	m.rules = rules
	m.rulesAt = time.Now()
	return rules
}

// InvalidateRules forces a reload on next evaluation.
func (m *Manager) InvalidateRules() {
	m.mu.Lock()
	m.rulesAt = time.Time{}
	m.mu.Unlock()
}

// EvaluateHost applies every enabled rule to a host update. prev is nil for new
// hosts and changes comes from store.UpsertHost.
func (m *Manager) EvaluateHost(prev *model.Host, host model.Host, changes []string) {
	if !m.enabled() {
		return
	}
	chset := map[string]bool{}
	for _, c := range changes {
		chset[c] = true
	}
	for _, rule := range m.Rules() {
		if !rule.Enabled {
			continue
		}
		var (
			fire    bool
			title   string
			message string
			dedupe  string
			sev     = rule.Severity
		)
		switch rule.Type {
		case "host_down":
			if host.Status == model.StatusDown && prev != nil && prev.Status == model.StatusUp {
				fire = true
				title = "Dispositivo caído"
				message = fmt.Sprintf("%s (%s) dejó de responder", displayName(host), host.IP)
				dedupe = fmt.Sprintf("host_down:%d", host.ID)
				sev = firstNonEmpty(rule.Severity, model.SeverityCritical)
			}
			if chset["online"] {
				_, _ = m.st.ResolveAlertsByDedupe(fmt.Sprintf("host_down:%d", host.ID))
			}
		case "host_new":
			if chset["discovered"] {
				fire = true
				title = "Nuevo dispositivo"
				message = fmt.Sprintf("Se descubrió %s (%s)", displayName(host), host.IP)
				dedupe = fmt.Sprintf("host_new:%d", host.ID)
			}
		case "latency_high":
			threshold := floatConfig(rule.Config, "threshold_ms", 150)
			if host.Status == model.StatusUp && host.LatencyMS > threshold && !chset["discovered"] {
				fire = true
				title = "Latencia alta"
				message = fmt.Sprintf("%s (%s) responde en %.1f ms (umbral %.0f ms)", displayName(host), host.IP, host.LatencyMS, threshold)
				dedupe = fmt.Sprintf("latency:%d:%s", host.ID, strings.TrimSuffix(fmt.Sprintf("%.0f", threshold), ".0"))
			}
		case "port_opened":
			if chset["ports_opened"] && prev != nil {
				opened := diffOpen(prev.Ports, host.Ports, true)
				if len(opened) > 0 {
					fire = true
					title = "Puertos abiertos"
					message = fmt.Sprintf("%s (%s): %s", displayName(host), host.IP, joinPorts(opened))
					dedupe = fmt.Sprintf("ports_open:%d:%s", host.ID, joinPorts(opened))
				}
			}
		case "port_closed":
			if chset["ports_closed"] && prev != nil {
				closed := diffOpen(prev.Ports, host.Ports, false)
				if len(closed) > 0 {
					fire = true
					title = "Puertos cerrados"
					message = fmt.Sprintf("%s (%s): %s", displayName(host), host.IP, joinPorts(closed))
					dedupe = fmt.Sprintf("ports_close:%d:%s", host.ID, joinPorts(closed))
				}
			}
		case "service_changed":
			if chset["service_changed"] {
				fire = true
				title = "Cambio de servicios"
				message = fmt.Sprintf("La firma de servicios de %s (%s) cambió", displayName(host), host.IP)
				dedupe = fmt.Sprintf("service:%d:%d", host.ID, time.Now().Unix()/3600)
			}
		case "device_changed":
			if chset["device_changed"] {
				fire = true
				title = "Cambio de identidad"
				message = fmt.Sprintf("%s (%s) cambió a tipo %s", displayName(host), host.IP, host.DeviceType)
				dedupe = fmt.Sprintf("device:%d:%s", host.ID, host.DeviceType)
			}
		}
		if !fire {
			continue
		}
		m.raise(rule, host, sev, title, message, dedupe)
	}
}

// EvaluateNetwork raises a subnet_usage alert when occupancy crosses the rule
// threshold. It is called after a scan finishes.
func (m *Manager) EvaluateNetwork(network model.Network, used, capacity int) {
	if !m.enabled() || capacity <= 0 {
		return
	}
	pct := float64(used) / float64(capacity) * 100
	for _, rule := range m.Rules() {
		if !rule.Enabled || rule.Type != "subnet_usage" {
			continue
		}
		threshold := floatConfig(rule.Config, "percent", 90)
		if pct < threshold {
			continue
		}
		host := model.Host{Hostname: network.Name, IP: network.CIDR, NetworkID: network.ID}
		title := "Uso de subred alto"
		message := fmt.Sprintf("%s (%s) usa %d de %d direcciones (%.1f%%)", network.Name, network.CIDR, used, capacity, pct)
		dedupe := fmt.Sprintf("subnet:%d", network.ID)
		m.raise(rule, host, rule.Severity, title, message, dedupe)
	}
}

// EvaluateScanFailure reports a scan that ended with an error.
func (m *Manager) EvaluateScanFailure(scan model.Scan) {
	if !m.enabled() {
		return
	}
	for _, rule := range m.Rules() {
		if !rule.Enabled || rule.Type != "scan_failed" {
			continue
		}
		host := model.Host{IP: scan.CIDR, NetworkID: scan.NetworkID}
		m.raise(rule, host, rule.Severity, "Escaneo fallido",
			fmt.Sprintf("El escaneo de %s terminó con error: %s", scan.CIDR, scan.Error),
			fmt.Sprintf("scan_failed:%d", scan.ID))
	}
}

// securityRuleCategories maps a dedicated security rule to the anomaly
// category it owns. A dedicated rule takes precedence over the generic
// "anomaly" rule for that category, so an operator never gets two alerts for
// the same finding.
var securityRuleCategories = map[string]string{
	"arp_spoofing":    "arp",
	"dhcp_rogue":      "dhcp",
	"snmp_anomaly":    "snmp",
	"passive_anomaly": "capture",
}

// EvaluateAnomalies raises alerts for heuristic findings such as ARP conflicts.
// It handles both the generic "anomaly" rule and the dedicated security rules.
// It returns the number of matching rule/finding pairs; dedupe and cooldown are
// handled by raise, so repeated scans do not flood the operator.
func (m *Manager) EvaluateAnomalies(findings []model.DiagnosticAnomaly, defaultSiteID ...int64) int {
	if !m.enabled() || len(findings) == 0 {
		return 0
	}
	rules := m.Rules()
	dedicated := map[string]bool{}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if category, ok := securityRuleCategories[rule.Type]; ok {
			dedicated[category] = true
		}
	}

	raised := 0
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		category := stringConfig(rule.Config, "category", "")
		generic := rule.Type == "anomaly"
		switch rule.Type {
		case "anomaly":
			// Generic fallback: skip families with a dedicated rule.
		case "arp_spoofing", "dhcp_rogue", "snmp_anomaly", "passive_anomaly":
			category = securityRuleCategories[rule.Type]
		default:
			continue
		}
		minConf := int(floatConfig(rule.Config, "min_confidence", 75))
		for _, a := range findings {
			if a.Confidence < minConf {
				continue
			}
			if category != "" && a.Category != category {
				continue
			}
			if generic && dedicated[a.Category] {
				continue
			}
			ip := firstAnomalyIP(a.HostIPs)
			host := model.Host{IP: ip, Hostname: a.Title}
			// Resolve the inventory host so the alert inherits its site and
			// appears in site-scoped lists and host timelines.
			if found, err := m.st.GetHostByIP(ip); err == nil && found != nil {
				host.ID = found.ID
				host.SiteID = found.SiteID
				if found.Hostname != "" {
					host.Hostname = found.Hostname
				}
			}
			// A rogue DHCP server often has no inventory host. Keep the alert
			// in its site so site-scoped lists and counters still show it.
			if host.ID == 0 && len(defaultSiteID) > 0 && defaultSiteID[0] > 0 {
				host.SiteID = defaultSiteID[0]
			}
			m.raise(rule, host, firstNonEmpty(rule.Severity, a.Severity), a.Title, a.Detail, "anomaly:"+a.ID)
			raised++
		}
	}
	return raised
}

func firstAnomalyIP(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

func (m *Manager) raise(rule model.AlertRule, host model.Host, severity, title, message, dedupe string) {
	if severity == "" {
		severity = model.SeverityWarning
	}
	if !m.allow() {
		m.log.Warn("límite de alertas por minuto alcanzado", "titulo", title)
		return
	}
	if rule.CooldownSec > 0 {
		if last, ok := m.st.LastAlertTime(dedupe); ok && time.Since(last) < time.Duration(rule.CooldownSec)*time.Second {
			return
		}
	}
	a := &model.Alert{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		SiteID:    host.SiteID,
		HostID:    host.ID,
		HostIP:    host.IP,
		HostName:  host.Hostname,
		Severity:  severity,
		Title:     title,
		Message:   message,
		Data:      map[string]any{"device_type": string(host.DeviceType), "mac": host.MAC},
		DedupeKey: dedupe,
	}
	created, isNew, err := m.st.CreateAlert(a, dedupe)
	if err != nil {
		m.log.Error("no se pudo crear alerta", "error", err)
		return
	}
	if !isNew || created == nil {
		return
	}
	m.mu.Lock()
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		go notify(*created)
	}
	channels := m.channelsFor(rule)
	if len(channels) == 0 {
		return
	}
	select {
	case m.queue <- job{alert: *created, channels: channels}:
	default:
		m.log.Warn("cola de alertas llena, notificación descartada", "alerta", created.ID)
	}
}

func (m *Manager) channelsFor(rule model.AlertRule) []model.AlertChannel {
	all, err := m.st.ListChannels()
	if err != nil {
		return nil
	}
	want := map[int64]bool{}
	for _, id := range rule.ChannelIDs {
		want[id] = true
	}
	var out []model.AlertChannel
	for _, ch := range all {
		if ch.Enabled && want[ch.ID] {
			out = append(out, ch)
		}
	}
	return out
}

// allow enforces the global notification rate limit.
func (m *Manager) allow() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cut := time.Now().Add(-time.Minute)
	kept := m.sentWindow[:0]
	for _, t := range m.sentWindow {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	m.sentWindow = kept
	cfg, _ := m.settings()
	if cfg.MaxPerMinute <= 0 {
		cfg.MaxPerMinute = 30
	}
	if len(m.sentWindow) >= cfg.MaxPerMinute {
		return false
	}
	m.sentWindow = append(m.sentWindow, time.Now())
	return true
}

// TestChannel sends an explicit test notification synchronously.
func (m *Manager) TestChannel(id int64) error {
	ch, err := m.st.GetChannel(id)
	if err != nil {
		return err
	}
	a := model.Alert{
		ID:       0,
		RuleName: "Prueba",
		Severity: model.SeverityInfo,
		Title:    "Notificación de prueba de NetDiag",
		Message:  "Si ves este mensaje, el canal está configurado correctamente.",
		Data:     map[string]any{"test": true},
	}
	err = m.deliver(context.Background(), *ch, a)
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = m.st.MarkChannelTest(ch.ID, msg)
	return err
}

func displayName(h model.Host) string {
	if h.Hostname != "" {
		return h.Hostname
	}
	if h.MAC != "" {
		return h.MAC
	}
	return h.IP
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringConfig(cfg map[string]any, key, def string) string {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return def
}

func floatConfig(cfg map[string]any, key string, def float64) float64 {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

func diffOpen(prev, next []model.Port, opened bool) []int {
	pm := map[int]bool{}
	nm := map[int]bool{}
	for _, p := range prev {
		if p.State == "open" || p.State == "" {
			pm[p.Number] = true
		}
	}
	for _, p := range next {
		if p.State == "open" || p.State == "" {
			nm[p.Number] = true
		}
	}
	var out []int
	if opened {
		for p := range nm {
			if !pm[p] {
				out = append(out, p)
			}
		}
	} else {
		for p := range pm {
			if !nm[p] {
				out = append(out, p)
			}
		}
	}
	sort.Ints(out)
	return out
}

func joinPorts(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d", p))
	}
	return strings.Join(parts, ", ")
}
