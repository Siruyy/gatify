package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStatsStreamBroker_SubscribePublish(t *testing.T) {
	broker := NewStatsStreamBroker(4)
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	expected := StatsStreamEvent{
		Timestamp: time.Now().UTC(),
		ClientID:  "client-1",
		Method:    http.MethodGet,
		Path:      "/proxy/resource",
		Allowed:   true,
		Status:    http.StatusOK,
	}

	broker.Publish(expected)

	select {
	case got := <-ch:
		if got.ClientID != expected.ClientID {
			t.Fatalf("expected client id %q, got %q", expected.ClientID, got.ClientID)
		}
		if got.Path != expected.Path {
			t.Fatalf("expected path %q, got %q", expected.Path, got.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for streamed event")
	}
}

func TestStatsStreamHandler_WebSocketReceivesEvent(t *testing.T) {
	broker := NewStatsStreamBroker(4)
	handler := NewStatsStreamHandler(broker)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] // convert http:// to ws://
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	expected := StatsStreamEvent{
		Timestamp: time.Now().UTC(),
		ClientID:  "client-2",
		Method:    http.MethodPost,
		Path:      "/proxy/upload",
		Allowed:   false,
		Status:    http.StatusTooManyRequests,
	}

	broker.Publish(expected)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got StatsStreamEvent
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read websocket event: %v", err)
	}

	if got.ClientID != expected.ClientID {
		t.Fatalf("expected client id %q, got %q", expected.ClientID, got.ClientID)
	}
	if got.Status != expected.Status {
		t.Fatalf("expected status %d, got %d", expected.Status, got.Status)
	}
}

func TestStatsStreamHandler_MethodNotAllowed(t *testing.T) {
	h := NewStatsStreamHandler(NewStatsStreamBroker(4))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stats/stream", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
