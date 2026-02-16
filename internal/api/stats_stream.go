package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// StatsStreamEvent represents a live gateway event streamed to dashboard clients.
type StatsStreamEvent struct {
	Timestamp time.Time `json:"timestamp"`
	ClientID  string    `json:"client_id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Allowed   bool      `json:"allowed"`
	Limit     int64     `json:"limit,omitempty"`
	Remaining int64     `json:"remaining,omitempty"`
	Status    int       `json:"status"`
}

// StatsStreamBroker fan-outs live stats events to active subscribers.
type StatsStreamBroker struct {
	mu          sync.RWMutex
	subscribers map[int]chan StatsStreamEvent
	nextID      int
	bufferSize  int
}

// NewStatsStreamBroker creates a new in-memory event broker.
func NewStatsStreamBroker(bufferSize int) *StatsStreamBroker {
	if bufferSize <= 0 {
		bufferSize = 64
	}

	return &StatsStreamBroker{
		subscribers: make(map[int]chan StatsStreamEvent),
		bufferSize:  bufferSize,
	}
}

// Publish broadcasts an event to all subscribers in a non-blocking way.
func (b *StatsStreamBroker) Publish(event StatsStreamEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Drop when subscriber buffer is full to avoid blocking producers.
		}
	}
}

// Subscribe registers a subscriber channel and returns an unsubscribe function.
func (b *StatsStreamBroker) Subscribe() (<-chan StatsStreamEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	ch := make(chan StatsStreamEvent, b.bufferSize)
	b.subscribers[id] = ch

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if existing, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(existing)
		}
	}

	return ch, unsubscribe
}

// StatsStreamHandler serves live events over WebSocket.
type StatsStreamHandler struct {
	broker   *StatsStreamBroker
	upgrader websocket.Upgrader
}

// NewStatsStreamHandler creates a WebSocket stream handler.
func NewStatsStreamHandler(broker *StatsStreamBroker) *StatsStreamHandler {
	if broker == nil {
		broker = NewStatsStreamBroker(64)
	}

	return &StatsStreamHandler{
		broker: broker,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}
}

// ServeHTTP upgrades requests to WebSocket and streams live events.
func (h *StatsStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	events, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if writeErr := conn.WriteJSON(event); writeErr != nil {
				return
			}
		case <-pingTicker.C:
			if pingErr := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); pingErr != nil {
				return
			}
		case <-readDone:
			return
		case <-r.Context().Done():
			return
		}
	}
}
