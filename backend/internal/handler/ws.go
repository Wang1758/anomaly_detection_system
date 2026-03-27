package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/pipeline"

	"nhooyr.io/websocket"
)

// WSHub manages WebSocket connections and broadcasts both alert events
// and real-time detection overlay data to all connected clients.
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
	go hub.alertBroadcastLoop()
	go hub.detectionBroadcastLoop()
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
	n := len(h.clients)
	h.mu.Unlock()

	log.Printf("WebSocket client connected, total=%d", n)

	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, conn)
	rem := len(h.clients)
	h.mu.Unlock()
	cancel()
	conn.Close(websocket.StatusNormalClosure, "")
	log.Printf("WebSocket client disconnected, total=%d", rem)
}

func (h *WSHub) alertBroadcastLoop() {
	for {
		bc := h.pipe.GetBroadcaster()
		if bc == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for event := range bc.AlertCh {
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("Failed to marshal alert: %v", err)
				continue
			}
			h.broadcastRaw(data)
		}
	}
}

func (h *WSHub) detectionBroadcastLoop() {
	for {
		bc := h.pipe.GetBroadcaster()
		if bc == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for frame := range bc.DetectionCh {
			data, err := json.Marshal(frame)
			if err != nil {
				continue
			}
			h.broadcastRaw(data)
		}
	}
}

func (h *WSHub) broadcastRaw(data []byte) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	if len(conns) == 0 {
		return
	}

	var stale []*websocket.Conn
	for _, conn := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		werr := conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if werr != nil {
			log.Printf("WebSocket write error: %v", werr)
			stale = append(stale, conn)
		}
	}

	if len(stale) == 0 {
		return
	}
	h.mu.Lock()
	for _, conn := range stale {
		if cancel, ok := h.clients[conn]; ok {
			delete(h.clients, conn)
			cancel()
			_ = conn.Close(websocket.StatusGoingAway, "write failed")
		}
	}
	h.mu.Unlock()
}
