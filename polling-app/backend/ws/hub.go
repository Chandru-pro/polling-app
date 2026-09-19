package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Hub keeps track of which websocket clients are watching which poll, and
// forwards Redis pub/sub messages for that poll straight to them. Redis is
// the actual source of truth for "what changed, right now" — the hub never
// computes counts itself, it just relays what Redis publishes.
type Hub struct {
	redis      *redis.Client
	mu         sync.RWMutex
	clients    map[string]map[*websocket.Conn]bool // pollId -> set of conns
	subscribed map[string]bool                     // pollId -> already has a redis subscriber running
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		redis:      rdb,
		clients:    make(map[string]map[*websocket.Conn]bool),
		subscribed: make(map[string]bool),
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func PollChannel(pollID string) string {
	return "poll:" + pollID + ":updates"
}

func (h *Hub) register(pollID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[pollID] == nil {
		h.clients[pollID] = make(map[*websocket.Conn]bool)
	}
	h.clients[pollID][conn] = true
}

func (h *Hub) unregister(pollID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[pollID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.clients, pollID)
		}
	}
	conn.Close()
}

func (h *Hub) broadcast(pollID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients[pollID] {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("ws write error: %v", err)
		}
	}
}

// SubscribeAndForward starts exactly one Redis subscriber for a poll
// channel, the first time anyone watches it, and pipes every published
// update to every websocket client currently registered for that poll.
// Guarded so repeated viewers don't each open their own Redis subscription.
func (h *Hub) SubscribeAndForward(ctx context.Context, pollID string) {
	h.mu.Lock()
	if h.subscribed[pollID] {
		h.mu.Unlock()
		return
	}
	h.subscribed[pollID] = true
	h.mu.Unlock()

	sub := h.redis.Subscribe(ctx, PollChannel(pollID))
	ch := sub.Channel()
	go func() {
		defer sub.Close()
		defer func() {
			h.mu.Lock()
			delete(h.subscribed, pollID)
			h.mu.Unlock()
		}()
		for msg := range ch {
			h.broadcast(pollID, []byte(msg.Payload))
		}
	}()
}

// ServeWS upgrades the connection and registers it against the poll's room.
// It also sends one initial snapshot so the client isn't blank until the
// next vote comes in.
func (h *Hub) ServeWS(c *gin.Context, pollID string, initialSnapshot []byte) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	h.register(pollID, conn)
	if initialSnapshot != nil {
		_ = conn.WriteMessage(websocket.TextMessage, initialSnapshot)
	}

	// Read pump just drains/detects disconnects; clients don't send us anything meaningful.
	go func() {
		defer h.unregister(pollID, conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func MustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return []byte("{}")
	}
	return b
}
