package alerts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
)

// HTTPClient wraps net/http with an SSRF guard. The guard inspects the resolved
// address inside DialContext, which also defeats DNS rebinding.
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a guarded client.
func NewHTTPClient(timeout time.Duration, allowPrivate bool) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: outboundControl(allowPrivate),
	}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &HTTPClient{client: &http.Client{Transport: tr, Timeout: timeout}}
}

// outboundControl blocks loopback, metadata and (by default) private targets.
func outboundControl(allowPrivate bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
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
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("destino no permitido: %s", ip)
		}
		if !allowPrivate && isPrivateIP(ip) {
			return fmt.Errorf("destino privado bloqueado (activa alerts.allow_private_targets): %s", ip)
		}
		return nil
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsPrivate() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

func (m *Manager) deliver(ctx context.Context, ch model.AlertChannel, a model.Alert) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt*attempt) * 300 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err = m.deliverOnce(ctx, ch, a)
		if err == nil {
			return nil
		}
	}
	return err
}

func (m *Manager) deliverOnce(ctx context.Context, ch model.AlertChannel, a model.Alert) error {
	switch ch.Type {
	case "webhook":
		return m.sendWebhook(ctx, ch.Config, a)
	case "slack":
		return m.sendSlack(ctx, ch.Config, a)
	case "discord":
		return m.sendDiscord(ctx, ch.Config, a)
	case "telegram":
		return m.sendTelegram(ctx, ch.Config, a)
	case "pagerduty":
		return m.sendPagerDuty(ctx, ch.Config, a)
	case "smtp":
		return m.sendSMTP(ch.Config, a)
	case "syslog":
		return m.sendSyslog(ch.Config, a)
	}
	return fmt.Errorf("tipo de canal no soportado: %s", ch.Type)
}

func (m *Manager) postJSON(ctx context.Context, target string, payload any, headers map[string]string) error {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL con esquema no permitido: %s", u.Scheme)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NetDiag/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := m.httpClient()
	if client == nil {
		return errors.New("cliente HTTP no configurado")
	}
	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (m *Manager) sendWebhook(ctx context.Context, cfg map[string]any, a model.Alert) error {
	target := cfgString(cfg, "url")
	if target == "" {
		return errors.New("webhook sin URL")
	}
	payload := map[string]any{
		"source":  "netdiag",
		"version": 1,
		"event":   "alert",
		"alert":   a,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if extra, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range extra {
			headers[k] = fmt.Sprint(v)
		}
	}
	if secret := cfgString(cfg, "secret"); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		headers["X-NetDiag-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	return m.postJSON(ctx, target, payload, headers)
}

func (m *Manager) sendSlack(ctx context.Context, cfg map[string]any, a model.Alert) error {
	target := cfgString(cfg, "url")
	if target == "" {
		return errors.New("Slack sin webhook URL")
	}
	text := fmt.Sprintf("%s *%s*\n%s\n%s", severityEmoji(a.Severity), a.Title, a.Message, a.HostIP)
	return m.postJSON(ctx, target, map[string]any{"text": text}, nil)
}

func (m *Manager) sendDiscord(ctx context.Context, cfg map[string]any, a model.Alert) error {
	target := cfgString(cfg, "url")
	if target == "" {
		return errors.New("Discord sin webhook URL")
	}
	text := fmt.Sprintf("%s **%s**\n%s\n`%s`", severityEmoji(a.Severity), a.Title, a.Message, a.HostIP)
	return m.postJSON(ctx, target, map[string]any{"content": text}, nil)
}

func (m *Manager) sendTelegram(ctx context.Context, cfg map[string]any, a model.Alert) error {
	token := cfgString(cfg, "token")
	chat := cfgString(cfg, "chat_id")
	if token == "" || chat == "" {
		return errors.New("Telegram requiere token y chat_id")
	}
	target := "https://api.telegram.org/bot" + token + "/sendMessage"
	text := fmt.Sprintf("%s %s\n%s\nIP: %s", severityEmoji(a.Severity), a.Title, a.Message, a.HostIP)
	payload := map[string]any{"chat_id": chat, "text": text, "disable_web_page_preview": true}
	return m.postJSON(ctx, target, payload, nil)
}

func (m *Manager) sendPagerDuty(ctx context.Context, cfg map[string]any, a model.Alert) error {
	key := cfgString(cfg, "routing_key")
	if key == "" {
		return errors.New("PagerDuty sin routing_key")
	}
	severity := "warning"
	switch a.Severity {
	case model.SeverityCritical:
		severity = "critical"
	case model.SeverityInfo:
		severity = "info"
	}
	source := a.HostIP
	if source == "" {
		source = "netdiag"
	}
	payload := map[string]any{
		"routing_key":  key,
		"event_action": "trigger",
		"dedup_key":    a.Title + "|" + source,
		"payload": map[string]any{
			"summary":   a.Title + ": " + a.Message,
			"source":    source,
			"severity":  severity,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"custom_details": map[string]any{
				"rule": a.RuleName,
				"host": a.HostName,
			},
		},
	}
	return m.postJSON(ctx, "https://events.pagerduty.com/v2/enqueue", payload, nil)
}

func (m *Manager) sendSMTP(cfg map[string]any, a model.Alert) error {
	host := cfgString(cfg, "host")
	if host == "" {
		return errors.New("SMTP sin host")
	}
	port := cfgInt(cfg, "port", 587)
	from := cfgString(cfg, "from")
	to := cfgStrings(cfg, "to")
	if from == "" {
		from = cfgString(cfg, "username")
	}
	if len(to) == 0 {
		return errors.New("SMTP sin destinatarios")
	}
	implicit := cfgBool(cfg, "tls", port == 465)
	startTLS := cfgBool(cfg, "starttls", port == 587)
	username := cfgString(cfg, "username")
	password := cfgString(cfg, "password")
	insecure := m.smtpInsecure()

	subject := "[NetDiag] " + a.Title
	body := fmt.Sprintf("%s\r\n\r\n%s\r\n\r\nSeveridad: %s\r\nDispositivo: %s\r\nIP: %s\r\nRegla: %s\r\n",
		a.Title, a.Message, a.Severity, a.HostName, a.HostIP, a.RuleName)
	msg := "From: " + from + "\r\nTo: " + strings.Join(to, ", ") + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n" +
		"Date: " + time.Now().Format(time.RFC1123Z) + "\r\n\r\n" + body

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: m.timeout(), Control: outboundControl(m.allowPrivateTargets())}
	var conn net.Conn
	var err error
	if implicit {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, InsecureSkipVerify: insecure})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if startTLS && !implicit {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host, InsecureSkipVerify: insecure}); err != nil {
				return err
			}
		}
	}
	if username != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (m *Manager) sendSyslog(cfg map[string]any, a model.Alert) error {
	addr := cfgString(cfg, "addr")
	if addr == "" {
		return errors.New("syslog sin dirección")
	}
	if !strings.Contains(addr, ":") {
		addr += ":514"
	}
	network := cfgString(cfg, "network")
	if network == "" {
		network = "udp"
	}
	severity := 5 // notice
	if a.Severity == model.SeverityCritical {
		severity = 2 // critical
	} else if a.Severity == model.SeverityWarning {
		severity = 4 // warning
	}
	pri := 16*8 + severity // local0
	msg := fmt.Sprintf("<%d>1 %s %s netdiag - - - [netdiag severity=\"%s\" rule=\"%s\"] %s: %s (%s)",
		pri, time.Now().UTC().Format(time.RFC3339), netutil.LocalHostname(), a.Severity, a.RuleName, a.Title, a.Message, a.HostIP)

	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: outboundControl(m.allowPrivateTargets())}
	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(msg))
	return err
}

func (m *Manager) timeout() time.Duration {
	cfg, _ := m.settings()
	if d := cfg.WebhookTimeout.Duration(); d > 0 {
		return d
	}
	return 10 * time.Second
}

func severityEmoji(sev string) string {
	switch sev {
	case model.SeverityCritical:
		return "🔴"
	case model.SeverityWarning:
		return "🟠"
	default:
		return "🔵"
	}
}

func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	switch v := cfg[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	}
	return ""
}

func cfgStrings(cfg map[string]any, key string) []string {
	if cfg == nil {
		return nil
	}
	switch v := cfg[key].(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	case string:
		var out []string
		for _, part := range strings.Split(v, ",") {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func cfgBool(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case bool:
		return v
	case string:
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func cfgInt(cfg map[string]any, key string, def int) int {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}
