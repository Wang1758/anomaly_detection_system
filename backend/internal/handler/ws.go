package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/models"
	"anomaly_detection_system/backend/internal/pipeline"
	"nhooyr.io/websocket"
)

// WSHub manages WebSocket connections and broadcasts alert events.
type WSHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]context.CancelFunc
	pipe    *pipeline.Pipeline
}

func NewWSHub(pipe *pipeline.Pipeline) *WSHub {
	hub := &WSHub{
		clients: make(map[*websocket.Conn]context.CancelFunc),
		pipe:    pipe,
	}
	go hub.broadcastLoop()
	return hub
}

func (h *WSHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	h.mu.Lock()
	h.clients[conn] = cancel
	h.mu.Unlock()

	log.Printf("WebSocket client connected, total=%d", len(h.clients))

	// Keep connection alive, read messages (for future bidirectional use)
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	cancel()
	conn.Close(websocket.StatusNormalClosure, "")
	log.Printf("WebSocket client disconnected, total=%d", len(h.clients))
}

func (h *WSHub) broadcastLoop() {
	for {
		bc := h.pipe.GetBroadcaster()
		if bc == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for event := range bc.AlertCh {
			h.broadcast(event)
		}
	}
}

func (h *WSHub) broadcast(event *models.AlertEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal alert: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			log.Printf("WebSocket write error: %v", err)
		}
		cancel()
	}
}
