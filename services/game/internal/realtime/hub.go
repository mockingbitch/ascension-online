// Package realtime fans server events out to WebSocket clients (GDD §23: selective live features only).
//
// Topology: the worker PUBLISHes envelopes to a Redis channel; every API instance SUBSCRIBEs and broadcasts to
// its locally connected clients. Clients only receive — they never send state over this socket.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
)

// Channel is the Redis pub/sub channel for broadcast envelopes.
const Channel = "realtime:broadcast"

// Envelope is the wire format (packages/contracts/ws-messages.md).
type Envelope struct {
	V    int       `json:"v"`
	Type string    `json:"type"`
	ID   string    `json:"id"`
	TS   time.Time `json:"ts"`
	Data any       `json:"data"`
}

// Hub tracks local connections.
type Hub struct {
	Log            *slog.Logger
	OriginPatterns []string // e.g. "localhost:5173", "game.example.com"
	MaxClients     int

	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	send chan []byte
}

// NewHub creates a hub.
func NewHub(log *slog.Logger, origins []string, maxClients int) *Hub {
	return &Hub{Log: log, OriginPatterns: origins, MaxClients: maxClients, clients: map[*client]struct{}{}}
}

// Count returns connected clients.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Broadcast queues msg for every client; slow clients (full buffer) are dropped, never waited on.
func (h *Hub) Broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			delete(h.clients, c)
			close(c.send)
		}
	}
}

// ServeHTTP upgrades and serves one read-only subscriber.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Count() >= h.MaxClients {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	// http.Server Read/WriteTimeout deadlines survive the hijack; clear them for this long-lived
	// connection (liveness is handled by our ping loop and per-write timeouts instead).
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return // Accept already wrote the error response
	}
	conn.SetReadLimit(512) // clients don't send data; only control frames are expected
	c := &client{send: make(chan []byte, 16)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if _, ok := h.clients[c]; ok {
			delete(h.clients, c)
			close(c.send)
		}
		h.mu.Unlock()
	}()

	ctx := conn.CloseRead(r.Context()) // discard client frames, cancel ctx when the peer goes away
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case msg, ok := <-c.send:
			if !ok {
				conn.Close(websocket.StatusPolicyViolation, "too slow")
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Subscribe relays Redis pub/sub messages to the hub until ctx ends (reconnects are handled by go-redis).
func (h *Hub) Subscribe(ctx context.Context, rdb *redis.Client) {
	sub := rdb.Subscribe(ctx, Channel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			h.Broadcast([]byte(m.Payload))
		}
	}
}

// Publish sends an envelope to every API instance.
func Publish(ctx context.Context, rdb *redis.Client, e Envelope) error {
	if e.V == 0 {
		e.V = 1
	}
	if e.Type == "" {
		return errors.New("realtime: envelope type required")
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return rdb.Publish(ctx, Channel, b).Err()
}
