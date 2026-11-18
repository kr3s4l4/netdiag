package integrations

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
)

func testEvent() Event {
	return Event{
		Kind: "scan.finished", SiteID: 1, Severity: "info",
		Title: "Escaneo finalizado", Message: "10 hosts", Timestamp: time.Now(),
	}
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	cfg := config.Integrations{Enabled: true, Timeout: config.Duration(2 * time.Second), MaxPerMinute: 60, QueueSize: 16, AllowPrivateTargets: true}
	m := New(nil, cfg, nil)
	t.Cleanup(m.Stop)
	return m
}

func TestElasticsearchBulkNDJSON(t *testing.T) {
	var gotBody string
	var gotPath string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	m := testManager(t)
	cfg := map[string]any{"url": srv.URL, "index": "netdiag-test", "api_key": "key-123"}
	if err := m.sendElasticsearch(context.Background(), cfg, testEvent()); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/_bulk" || gotContentType != "application/x-ndjson" {
		t.Fatalf("petición inesperada: %s %s", gotPath, gotContentType)
	}
	lines := strings.Split(strings.TrimSpace(gotBody), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperaba 2 líneas NDJSON, hay %d: %q", len(lines), gotBody)
	}
	if !strings.Contains(lines[0], `"netdiag-test"`) {
		t.Fatalf("índice ausente en la acción bulk: %s", lines[0])
	}
	var doc Event
	if err := json.Unmarshal([]byte(lines[1]), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "scan.finished" {
		t.Fatalf("evento incorrecto: %+v", doc)
	}
}

func TestWebhookSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-NetDiag-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	m := testManager(t)
	if err := m.sendWebhook(context.Background(), map[string]any{"url": srv.URL, "secret": "topsecret"}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotSig, "sha256=") || len(gotSig) != 71 {
		t.Fatalf("firma inesperada: %q", gotSig)
	}
}

func TestChatProvidersAndTelegram(t *testing.T) {
	type call struct {
		path string
		body string
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.URL.Path, string(b)})
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	m := testManager(t)
	for _, provider := range []string{"slack", "discord", "msteams"} {
		if err := m.sendChat(context.Background(), map[string]any{"url": srv.URL + "/hook"}, testEvent(), provider); err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
	}
	if err := m.sendTelegram(context.Background(), map[string]any{
		"api_url": srv.URL, "token": "123:ABC", "chat_id": "-1001",
	}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("esperaba 4 llamadas, hay %d", len(calls))
	}
	if !strings.Contains(calls[0].body, "text") || !strings.Contains(calls[1].body, "content") ||
		!strings.Contains(calls[2].body, "MessageCard") {
		t.Fatalf("payloads de chat incorrectos: %+v", calls)
	}
	if !strings.Contains(calls[3].path, "/bot123:ABC/sendMessage") {
		t.Fatalf("ruta de Telegram incorrecta: %s", calls[3].path)
	}
}

func TestPagerDutyAndKafka(t *testing.T) {
	var pdBody, kafkaBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "pagerduty") {
			pdBody = string(b)
		} else {
			kafkaBody = string(b)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	m := testManager(t)
	if err := m.sendPagerDuty(context.Background(), map[string]any{
		"url": srv.URL + "/pagerduty", "routing_key": "rk-1",
	}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if err := m.sendKafka(context.Background(), map[string]any{
		"url": srv.URL, "topic": "netdiag.events", "token": "tok",
	}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pdBody, `"event_action":"trigger"`) || !strings.Contains(pdBody, "rk-1") {
		t.Fatalf("payload de PagerDuty incorrecto: %s", pdBody)
	}
	if !strings.HasPrefix(kafkaBody, "[") || !strings.Contains(kafkaBody, `"key":"scan.finished"`) {
		t.Fatalf("payload de Kafka incorrecto: %s", kafkaBody)
	}
}

func TestSSRFGuardBlocksPrivateTargets(t *testing.T) {
	cfg := config.Integrations{Enabled: true, Timeout: config.Duration(2 * time.Second), MaxPerMinute: 60, QueueSize: 16, AllowPrivateTargets: false}
	m := New(nil, cfg, nil)
	t.Cleanup(m.Stop)
	err := m.sendWebhook(context.Background(), map[string]any{"url": "http://127.0.0.1:9999/hook"}, testEvent())
	if err == nil {
		t.Fatal("el guard SSRF debe bloquear 127.0.0.1 sin autorización explícita")
	}
}
