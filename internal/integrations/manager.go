// Package integrations exports NetDiag events to external systems. All
// deliveries are best effort, bounded and guarded against SSRF; credentials
// are stored encrypted by internal/store.
package integrations

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// Event is the normalized payload delivered to every exporter.
type Event struct {
	Kind      string         `json:"kind"`
	SiteID    int64          `json:"site_id,omitempty"`
	Severity  string         `json:"severity,omitempty"`
	Title     string         `json:"title,omitempty"`
	Message   string         `json:"message,omitempty"`
	HostIP    string         `json:"host_ip,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Manager owns the delivery queue and the guarded HTTP client.
type Manager struct {
	st  *store.Store
	cfg config.Integrations
	log *slog.Logger

	client *http.Client
	queue  chan Event
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	recent  []time.Time
	started bool
}

// New creates a manager. A nil store is allowed for providers that do not
// persist test results (used in tests).
func New(st *store.Store, cfg config.Integrations, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	m := &Manager{st: st, cfg: cfg, log: log}
	m.client = m.newClient()
	return m
}

// UpdateConfig swaps settings and rebuilds the network guard in place.
func (m *Manager) UpdateConfig(cfg config.Integrations) {
	m.mu.Lock()
	m.cfg = cfg
	m.client = m.newClient()
	m.mu.Unlock()
}

func (m *Manager) settings() config.Integrations {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) httpClient() *http.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

func (m *Manager) timeout() time.Duration {
	d := m.settings().Timeout.Duration()
	if d <= 0 {
		return 10 * time.Second
	}
	return d
}

// Start launches the workers. It is safe to call more than once.
func (m *Manager) Start(ctx context.Context) {
	cfg := m.settings()
	if !cfg.Enabled {
		return
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.ctx, m.cancel = context.WithCancel(ctx)
	size := cfg.QueueSize
	if size < 16 {
		size = 512
	}
	m.queue = make(chan Event, size)
	m.mu.Unlock()

	for i := 0; i < 2; i++ {
		m.wg.Add(1)
		go m.worker()
	}
}

// Stop drains in-flight deliveries and releases all resources.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	m.started = false
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

// Publish enqueues an event. It never blocks the caller: a saturated queue
// drops the event with a warning instead of stalling a scan.
func (m *Manager) Publish(ev Event) {
	settings := m.settings()
	if !settings.Enabled {
		return
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	m.mu.Lock()
	queue := m.queue
	m.mu.Unlock()
	if queue == nil {
		return
	}
	select {
	case queue <- ev:
	default:
		m.log.Warn("cola de integraciones llena; evento descartado", "kind", ev.Kind)
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case ev := <-m.queue:
			if !m.allow(ev.Timestamp) {
				continue
			}
			items, err := m.st.ListIntegrations()
			if err != nil {
				m.log.Warn("no se pudieron listar integraciones", "error", err)
				continue
			}
			for _, it := range items {
				if !it.Enabled {
					continue
				}
				ctx, cancel := context.WithTimeout(m.ctx, m.timeout())
				err := m.deliver(ctx, it, ev)
				cancel()
				if err != nil && m.st != nil {
					_ = m.st.MarkIntegrationTest(it.ID, err.Error())
					m.log.Warn("entrega de integración fallida", "id", it.ID, "type", it.Type, "error", err)
				}
			}
		}
	}
}

// allow applies a global sliding-window rate limit across all exporters.
func (m *Manager) allow(now time.Time) bool {
	limit := m.settings().MaxPerMinute
	if limit <= 0 {
		limit = 60
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cut := now.Add(-time.Minute)
	kept := m.recent[:0]
	for _, ts := range m.recent {
		if ts.After(cut) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= limit {
		m.recent = kept
		return false
	}
	m.recent = append(kept, now)
	return true
}

// Test sends one synthetic event through a single integration and records the
// result. It is synchronous so the UI can report success or failure.
func (m *Manager) Test(ctx context.Context, id int64) error {
	it, err := m.st.GetIntegration(id)
	if err != nil {
		return err
	}
	ev := Event{
		Kind: "integration.test", SiteID: 1, Severity: model.SeverityInfo,
		Title: "Prueba de integración", Message: "Evento de prueba de NetDiag",
		Timestamp: time.Now(),
	}
	ctx, cancel := context.WithTimeout(ctx, m.timeout())
	defer cancel()
	err = m.deliver(ctx, *it, ev)
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = m.st.MarkIntegrationTest(it.ID, msg)
	return err
}

func (m *Manager) deliver(ctx context.Context, it model.Integration, ev Event) error {
	if it.Config == nil {
		it.Config = map[string]any{}
	}
	switch it.Type {
	case "elasticsearch":
		return m.sendElasticsearch(ctx, it.Config, ev)
	case "syslog":
		return m.sendSyslog(ctx, it.Config, ev)
	case "webhook":
		return m.sendWebhook(ctx, it.Config, ev)
	case "slack":
		return m.sendChat(ctx, it.Config, ev, "slack")
	case "discord":
		return m.sendChat(ctx, it.Config, ev, "discord")
	case "msteams":
		return m.sendChat(ctx, it.Config, ev, "msteams")
	case "telegram":
		return m.sendTelegram(ctx, it.Config, ev)
	case "pagerduty":
		return m.sendPagerDuty(ctx, it.Config, ev)
	case "kafka":
		return m.sendKafka(ctx, it.Config, ev)
	case "redis":
		return m.sendRedis(ctx, it.Config, ev)
	default:
		return fmt.Errorf("tipo de integración no soportado: %s", it.Type)
	}
}

// newClient builds an HTTP client that resolves and inspects the target IP on
// every dial, which also prevents DNS-rebinding to a private address.
func (m *Manager) newClient() *http.Client {
	timeout := m.cfg.Timeout.Duration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	allowPrivate := m.cfg.AllowPrivateTargets
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("destino inválido: %s", host)
			}
			if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
				return errors.New("destino de metadatos bloqueado")
			}
			if !allowPrivate && (netutil.IsPrivate(ip) || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
				return fmt.Errorf("destino privado o local bloqueado: %s (activa integrations.allow_private_targets)", ip)
			}
			return nil
		},
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

func cfgString(cfg map[string]any, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func cfgBool(cfg map[string]any, key string, def bool) bool {
	v, ok := cfg[key]
	if !ok || v == nil {
		return def
	}
	switch value := v.(type) {
	case bool:
		return value
	case string:
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	case float64:
		return value != 0
	}
	return def
}

func cfgInt(cfg map[string]any, key string, def int) int {
	v, ok := cfg[key]
	if !ok || v == nil {
		return def
	}
	switch value := v.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		n, err := strconv.Atoi(value)
		if err == nil {
			return n
		}
	}
	return def
}
