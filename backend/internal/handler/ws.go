package handler

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/models"
	"anomaly_detection_system/backend/internal/pipeline"

	"nhooyr.io/websocket"
)

// WSHub manages WebSocket connections and broadcasts both alert events
// and real-time detection overlay data to all connected clients.
type WSHub struct {
	eventMu      sync.RWMutex
	eventClients map[*websocket.Conn]context.CancelFunc
	liveMu       sync.RWMutex
	liveClients  map[*websocket.Conn]context.CancelFunc
	pipe         *pipeline.Pipeline
}

func NewWSHub(pipe *pipeline.Pipeline) *WSHub {
	hub := &WSHub{
		eventClients: make(map[*websocket.Conn]context.CancelFunc),
		liveClients:  make(map[*websocket.Conn]context.CancelFunc),
		pipe:         pipe,
	}
	go hub.alertBroadcastLoop()
	go hub.liveBroadcastLoop()
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
	h.eventMu.Lock()
	h.eventClients[conn] = cancel
	n := len(h.eventClients)
	h.eventMu.Unlock()

	log.Printf("WebSocket event client connected, total=%d", n)

	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	h.eventMu.Lock()
	delete(h.eventClients, conn)
	rem := len(h.eventClients)
	h.eventMu.Unlock()
	cancel()
	conn.Close(websocket.StatusNormalClosure, "")
	log.Printf("WebSocket event client disconnected, total=%d", rem)
}

func (h *WSHub) HandleLiveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("Live WebSocket accept error: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	h.liveMu.Lock()
	h.liveClients[conn] = cancel
	n := len(h.liveClients)
	h.liveMu.Unlock()

	log.Printf("WebSocket live client connected, total=%d", n)

	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	h.liveMu.Lock()
	delete(h.liveClients, conn)
	rem := len(h.liveClients)
	h.liveMu.Unlock()
	cancel()
	conn.Close(websocket.StatusNormalClosure, "")
	log.Printf("WebSocket live client disconnected, total=%d", rem)
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
			h.broadcastEventText(data)
		}
	}
}

func (h *WSHub) liveBroadcastLoop() {
	for {
		bc := h.pipe.GetBroadcaster()
		if bc == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for frame := range bc.LiveCh {
			data, err := encodeLiveFrameBinary(frame)
			if err != nil {
				log.Printf("Failed to encode live frame: %v", err)
				continue
			}
			h.broadcastLiveBinary(data)
		}
	}
}

func (h *WSHub) broadcastEventText(data []byte) {
	h.eventMu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.eventClients))
	for conn := range h.eventClients {
		conns = append(conns, conn)
	}
	h.eventMu.RUnlock()

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
	h.eventMu.Lock()
	for _, conn := range stale {
		if cancel, ok := h.eventClients[conn]; ok {
			delete(h.eventClients, conn)
			cancel()
			_ = conn.Close(websocket.StatusGoingAway, "write failed")
		}
	}
	h.eventMu.Unlock()
}

func (h *WSHub) broadcastLiveBinary(data []byte) {
	h.liveMu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.liveClients))
	for conn := range h.liveClients {
		conns = append(conns, conn)
	}
	h.liveMu.RUnlock()

	if len(conns) == 0 {
		return
	}

	var stale []*websocket.Conn
	for _, conn := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		werr := conn.Write(ctx, websocket.MessageBinary, data)
		cancel()
		if werr != nil {
			log.Printf("WebSocket live write error: %v", werr)
			stale = append(stale, conn)
		}
	}

	if len(stale) == 0 {
		return
	}

	h.liveMu.Lock()
	for _, conn := range stale {
		if cancel, ok := h.liveClients[conn]; ok {
			delete(h.liveClients, conn)
			cancel()
			_ = conn.Close(websocket.StatusGoingAway, "live write failed")
		}
	}
	h.liveMu.Unlock()
}

func encodeLiveFrameBinary(frame *models.LiveFrame) ([]byte, error) {
	meta := struct {
		Type        string                 `json:"type"`
		FrameID     int64                  `json:"frame_id"`
		FrameWidth  int                    `json:"frame_width"`
		FrameHeight int                    `json:"frame_height"`
		Detections  []models.DetectionMeta `json:"detections"`
	}{
		Type:        frame.Type,
		FrameID:     frame.FrameID,
		FrameWidth:  frame.FrameWidth,
		FrameHeight: frame.FrameHeight,
		Detections:  frame.Detections,
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}

	packet := make([]byte, 4+len(metaBytes)+len(frame.JPEG))
	binary.BigEndian.PutUint32(packet[:4], uint32(len(metaBytes)))
	copy(packet[4:4+len(metaBytes)], metaBytes)
	copy(packet[4+len(metaBytes):], frame.JPEG)
	return packet, nil
}
