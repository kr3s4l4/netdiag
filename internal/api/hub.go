package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is one message pushed to connected browsers.
type Event struct {
	Type string    `json:"type"`
	Data any       `json:"data,omitempty"`
	TS   time.Time `json:"ts"`
}

// Hub fans events out to WebSocket clients. Slow consumers are dropped instead
// of blocking the scanner.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	log     *slog.Logger
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{clients: make(map[*wsClient]struct{}), log: log}
}

// Broadcast sends an event to every connected client.
func (h *Hub) Broadcast(eventType string, data any) {
	if h == nil {
		return
	}
	payload, err := json.Marshal(Event{Type: eventType, Data: data, TS: time.Now()})
	if err != nil {
		h.log.Warn("no se pudo serializar evento", "tipo", eventType, "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
			// Backpressure: drop the client rather than stall the producer.
			h.drop(c)
		}
	}
}

// Clients returns the number of connected browsers.
func (h *Hub) Clients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.once.Do(func() { close(c.done) })
	_ = c.conn.Close()
}

func (h *Hub) drop(c *wsClient) {
	go h.remove(c)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// Same-origin only: compare the Origin host with the request host.
		host := r.Host
		trimmed := strings.TrimPrefix(strings.TrimPrefix(origin, "http://"), "https://")
		return strings.EqualFold(trimmed, host)
	},
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote a response.
		return
	}
	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
	}
	s.hub.add(client)
	welcome, _ := json.Marshal(Event{
		Type: "hello",
		Data: map[string]any{"client": s.hub.Clients(), "version": s.version},
		TS:   time.Now(),
	})
	client.send <- welcome

	go s.writePump(client)
	go s.readPump(client)
}

func (s *Server) readPump(c *wsClient) {
	defer s.hub.remove(c)
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		// The client may send {"type":"ping"} to keep the link warm.
		if strings.Contains(string(msg), `"ping"`) {
			select {
			case c.send <- []byte(`{"type":"pong","ts":"` + time.Now().UTC().Format(time.RFC3339) + `"}`):
			default:
			}
		}
	}
}

func (s *Server) writePump(c *wsClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer s.hub.remove(c)
	for {
		select {
		case <-c.done:
			return
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
