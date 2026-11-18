package integrations

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// post sends a body with bounded size and rejects non-2xx responses.
func (m *Manager) post(ctx context.Context, target, contentType string, body []byte, headers map[string]string) error {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("destino HTTP inválido: %q", target)
	}
	if len(body) > 2<<20 {
		return fmt.Errorf("cuerpo demasiado grande: %d bytes", len(body))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "NetDiag/1.4")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d en %s", resp.StatusCode, u.Redacted())
	}
	return nil
}

func postJSON(ctx context.Context, m *Manager, target string, payload any, headers map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return m.post(ctx, target, "application/json", body, headers)
}

// sendElasticsearch indexes the event as one NDJSON bulk operation. The
// endpoint is typically http://localhost:9200; private targets require
// integrations.allow_private_targets: true.
func (m *Manager) sendElasticsearch(ctx context.Context, cfg map[string]any, ev Event) error {
	base := strings.TrimRight(cfgString(cfg, "url"), "/")
	if base == "" {
		base = "http://localhost:9200"
	}
	index := cfgString(cfg, "index")
	if index == "" {
		index = "netdiag-events"
	}
	action, _ := json.Marshal(map[string]any{"index": map[string]any{"_index": index}})
	doc, err := eventJSON(ev)
	if err != nil {
		return err
	}
	body := append(action, '\n')
	body = append(body, doc...)
	body = append(body, '\n')
	headers := map[string]string{}
	apiKey := cfgString(cfg, "api_key")
	if apiKey != "" {
		headers["Authorization"] = "ApiKey " + apiKey
	} else if user := cfgString(cfg, "username"); user != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/_bulk", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-ndjson")
		req.Header.Set("User-Agent", "NetDiag/1.4")
		req.SetBasicAuth(user, cfgString(cfg, "password"))
		resp, err := m.httpClient().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("Elasticsearch HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return m.post(ctx, base+"/_bulk", "application/x-ndjson", body, headers)
}

// sendWebhook posts the normalized event, optionally signed with a shared
// secret in X-NetDiag-Signature (sha256=HMAC).
func (m *Manager) sendWebhook(ctx context.Context, cfg map[string]any, ev Event) error {
	target := cfgString(cfg, "url")
	if target == "" {
		return fmt.Errorf("webhook sin url")
	}
	body, err := eventJSON(ev)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if secret := cfgString(cfg, "secret"); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		headers["X-NetDiag-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	return m.post(ctx, target, "application/json", body, headers)
}

func (m *Manager) sendChat(ctx context.Context, cfg map[string]any, ev Event, provider string) error {
	target := cfgString(cfg, "url")
	if target == "" {
		return fmt.Errorf("%s sin url", provider)
	}
	text := fmt.Sprintf("[%s] %s — %s", strings.ToUpper(ev.Severity), ev.Title, ev.Message)
	var payload any
	switch provider {
	case "slack":
		payload = map[string]any{"text": text}
	case "discord":
		payload = map[string]any{"content": text}
	case "msteams":
		payload = map[string]any{
			"@type": "MessageCard", "@context": "http://schema.org/extensions",
			"summary": ev.Title, "text": text,
		}
	default:
		return fmt.Errorf("proveedor de chat no soportado: %s", provider)
	}
	return postJSON(ctx, m, target, payload, map[string]string{})
}

func (m *Manager) sendTelegram(ctx context.Context, cfg map[string]any, ev Event) error {
	token := cfgString(cfg, "token")
	chatID := cfgString(cfg, "chat_id")
	if token == "" || chatID == "" {
		return fmt.Errorf("telegram requiere token y chat_id")
	}
	base := cfgString(cfg, "api_url")
	if base == "" {
		base = "https://api.telegram.org"
	}
	target := strings.TrimRight(base, "/") + "/bot" + token + "/sendMessage"
	text := fmt.Sprintf("[%s] %s — %s", strings.ToUpper(ev.Severity), ev.Title, ev.Message)
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	return m.post(ctx, target, "application/json", body, map[string]string{})
}

func (m *Manager) sendPagerDuty(ctx context.Context, cfg map[string]any, ev Event) error {
	routingKey := cfgString(cfg, "routing_key")
	if routingKey == "" {
		return fmt.Errorf("pagerduty sin routing_key")
	}
	target := cfgString(cfg, "url")
	if target == "" {
		target = "https://events.pagerduty.com/v2/enqueue"
	}
	sev := ev.Severity
	if sev == "" {
		sev = "warning"
	}
	payload := map[string]any{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("netdiag-%s-%d-%s", ev.Kind, ev.SiteID, ev.HostIP),
		"payload": map[string]any{
			"summary": ev.Title, "source": "netdiag", "severity": sev,
			"custom_details": ev.Data,
		},
	}
	return postJSON(ctx, m, target, payload, map[string]string{})
}

// sendKafka uses the Confluent REST Proxy, which speaks plain HTTP and avoids
// pulling a Kafka client (and its C dependencies) into the binary.
func (m *Manager) sendKafka(ctx context.Context, cfg map[string]any, ev Event) error {
	base := strings.TrimRight(cfgString(cfg, "url"), "/")
	topic := cfgString(cfg, "topic")
	if base == "" || topic == "" {
		return fmt.Errorf("kafka requiere url y topic")
	}
	payload := []map[string]any{{"key": ev.Kind, "value": ev}}
	headers := map[string]string{}
	if token := cfgString(cfg, "token"); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return postJSON(ctx, m, base+"/topics/"+url.PathEscape(topic), payload, headers)
}

func eventJSON(ev Event) ([]byte, error) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	return json.Marshal(ev)
}
