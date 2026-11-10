// Package realtime fans server events out to WebSocket clients (GDD §23: selective live features only).
//
// Topology: the worker PUBLISHes envelopes to a Redis channel; every API instance SUBSCRIBEs and broadcasts to
// its locally connected clients. Clients never send state over this socket; the only client frame is a topic
// subscription ({"op":"sub","topic":"hall:3"} / {"op":"unsub"}): one topic per socket, validated by the hub.
// Topic envelopes travel on Redis channel TopicChannelPrefix+topic and reach only the sockets that follow it
// (Tử Tiêu Cung điện, decisions 2026-11-10); everything else is still broadcast to every socket.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
)

// Channel is the Redis pub/sub channel for broadcast envelopes.
const Channel = "realtime:broadcast"

// TopicChannelPrefix + topic is the Redis channel of a topic (API instances PSUBSCRIBE TopicChannelPrefix + "*").
const TopicChannelPrefix = "realtime:topic:"

// Client frame limits: a socket may (re)subscribe this many times per minute before it is closed.
const maxFramesPerMinute = 30

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
	// TopicOK validates a subscription (nil = no topics accepted). Topics are public rooms (hall chat and seats are
	// readable by any player), so a subscription needs no authentication — only a well-formed, bounded name.
	TopicOK func(topic string) bool

	mu      sync.Mutex
	clients map[*client]struct{}
	topics  map[string]map[*client]struct{}
}

type client struct {
	send  chan []byte
	topic string // guarded by Hub.mu
}

// NewHub creates a hub.
func NewHub(log *slog.Logger, origins []string, maxClients int) *Hub {
	return &Hub{Log: log, OriginPatterns: origins, MaxClients: maxClients, clients: map[*client]struct{}{},
		topics: map[string]map[*client]struct{}{}}
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
		h.offer(c, msg)
	}
}

// BroadcastTopic queues msg for the clients following topic only (same drop rule).
func (h *Hub) BroadcastTopic(topic string, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.topics[topic] {
		h.offer(c, msg)
	}
}

// PushTopic marshals an envelope and queues it for this instance's followers of topic (no Redis: used for values
// every instance computes itself, e.g. the hall listener count).
func (h *Hub) PushTopic(topic, eventType, id string, data any) error {
	b, err := json.Marshal(Envelope{V: 1, Type: eventType, ID: id, TS: time.Now().UTC(), Data: data})
	if err != nil {
		return err
	}
	h.BroadcastTopic(topic, b)
	return nil
}

// LocalTopics lists the topics with prefix that at least one local client follows.
func (h *Hub) LocalTopics(prefix string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.topics))
	for t, cs := range h.topics {
		if len(cs) > 0 && strings.HasPrefix(t, prefix) {
			out = append(out, t)
		}
	}
	return out
}

// offer: caller holds mu.
func (h *Hub) offer(c *client, msg []byte) {
	select {
	case c.send <- msg:
	default:
		h.drop(c)
	}
}

// drop removes a client from every index and closes its queue (once). Caller holds mu.
func (h *Hub) drop(c *client) {
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	h.follow(c, "")
	close(c.send)
}

// follow moves c to topic ("" = none). Caller holds mu.
func (h *Hub) follow(c *client, topic string) {
	if c.topic != "" {
		if set := h.topics[c.topic]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.topics, c.topic)
			}
		}
	}
	c.topic = topic
	if topic != "" {
		set := h.topics[topic]
		if set == nil {
			set = map[*client]struct{}{}
			h.topics[topic] = set
		}
		set[c] = struct{}{}
	}
}

// frame is the only client → server message.
type frame struct {
	Op    string `json:"op"`
	Topic string `json:"topic"`
}

// handleFrame applies a subscription frame; malformed frames and unknown topics are ignored.
func (h *Hub) handleFrame(c *client, b []byte) {
	var f frame
	if json.Unmarshal(b, &f) != nil {
		return
	}
	switch f.Op {
	case "sub":
		if h.TopicOK == nil || !h.TopicOK(f.Topic) {
			return
		}
	case "unsub":
		f.Topic = ""
	default:
		return
	}
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		h.follow(c, f.Topic)
	}
	h.mu.Unlock()
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
	conn.SetReadLimit(512) // clients only send tiny subscription frames
	c := &client{send: make(chan []byte, 16)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.drop(c)
		h.mu.Unlock()
	}()

	// Read loop: subscription frames (rate-bounded), and ctx cancelled when the peer goes away.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer cancel()
		window, n := time.Now(), 0
		for {
			typ, b, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if now := time.Now(); now.Sub(window) > time.Minute {
				window, n = now, 0
			}
			if n++; n > maxFramesPerMinute {
				conn.Close(websocket.StatusPolicyViolation, "too many frames")
				return
			}
			if typ == websocket.MessageText {
				h.handleFrame(c, b)
			}
		}
	}()
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

// Subscribe relays Redis pub/sub messages to the hub until ctx ends (reconnects are handled by go-redis):
// broadcasts to every socket, topic messages to that topic's sockets.
func (h *Hub) Subscribe(ctx context.Context, rdb *redis.Client) {
	sub := rdb.Subscribe(ctx, Channel)
	defer sub.Close()
	if err := sub.PSubscribe(ctx, TopicChannelPrefix+"*"); err != nil && h.Log != nil {
		h.Log.Warn("realtime: psubscribe", "err", err)
	}
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			if topic, ok := strings.CutPrefix(m.Channel, TopicChannelPrefix); ok {
				h.BroadcastTopic(topic, []byte(m.Payload))
			} else {
				h.Broadcast([]byte(m.Payload))
			}
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

// PublishTopic sends an envelope to the sockets following topic on every API instance ("" = Publish to all).
func PublishTopic(ctx context.Context, rdb *redis.Client, topic string, e Envelope) error {
	if topic == "" {
		return Publish(ctx, rdb, e)
	}
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
	return rdb.Publish(ctx, TopicChannelPrefix+topic, b).Err()
}
