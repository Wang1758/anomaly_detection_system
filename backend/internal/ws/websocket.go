package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/pipeline"
)

// BroadcastMessage WebSocket 广播消息
type BroadcastMessage struct {
	Type      string      `json:"type"`      // 消息类型: "frame", "alert", "status"
	Timestamp int64       `json:"timestamp"` // 时间戳
	Data      interface{} `json:"data"`      // 消息数据
}

// FrameMessage 帧数据消息
type FrameMessage struct {
	FrameID       int64            `json:"frame_id"`
	ImageData     string           `json:"image_data"` // Base64 编码的 JPEG
	Width         int              `json:"width"`
	Height        int              `json:"height"`
	InferenceTime int64            `json:"inference_time"`
	Detections    []*DetectionData `json:"detections"`
}

// DetectionData 检测框数据
type DetectionData struct {
	ID          int32   `json:"id"`
	X1          float32 `json:"x1"`
	Y1          float32 `json:"y1"`
	X2          float32 `json:"x2"`
	Y2          float32 `json:"y2"`
	ClassName   string  `json:"class_name"`
	ClassID     int32   `json:"class_id"`
	Confidence  float32 `json:"confidence"`
	Entropy     float32 `json:"entropy"`
	IsUncertain bool    `json:"is_uncertain"`
}

// AlertMessage 报警消息
type AlertMessage struct {
	ID         int32   `json:"id"`
	FrameID    int64   `json:"frame_id"`
	Timestamp  int64   `json:"timestamp"`
	ImageData  string  `json:"image_data"` // 裁剪后的截图 (Base64)
	X1         float32 `json:"x1"`
	Y1         float32 `json:"y1"`
	X2         float32 `json:"x2"`
	Y2         float32 `json:"y2"`
	ClassName  string  `json:"class_name"`
	Confidence float32 `json:"confidence"`
	Entropy    float32 `json:"entropy"`
}

// Client WebSocket 客户端
type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	ctx    context.Context
	cancel context.CancelFunc
}

// Hub WebSocket 连接中心
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewHub 创建 WebSocket Hub
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run 运行 Hub
func (h *Hub) Run() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] 新客户端连接，当前连接数: %d", len(h.clients))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] 客户端断开，当前连接数: %d", len(h.clients))
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 客户端发送缓冲区满，关闭连接
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Stop 停止 Hub
func (h *Hub) Stop() {
	h.cancel()
	h.mu.Lock()
	for client := range h.clients {
		client.cancel()
		close(client.send)
	}
	h.clients = make(map[*Client]bool)
	h.mu.Unlock()
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg *BroadcastMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WebSocket] 消息序列化失败: %v", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		log.Println("[WebSocket] 广播通道已满")
	}
}

// ClientCount 获取客户端数量
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WebSocketServer WebSocket 服务器
type WebSocketServer struct {
	hub        *Hub
	config     *config.Config
	resultChan chan *pipeline.DetectionResult
	alertChan  chan *AlertMessage
}

// NewWebSocketServer 创建 WebSocket 服务器
func NewWebSocketServer(cfg *config.Config, resultChan chan *pipeline.DetectionResult, alertChan chan *AlertMessage) *WebSocketServer {
	return &WebSocketServer{
		hub:        NewHub(),
		config:     cfg,
		resultChan: resultChan,
		alertChan:  alertChan,
	}
}

// Start 启动 WebSocket 服务器
func (ws *WebSocketServer) Start() {
	// 启动 Hub
	go ws.hub.Run()

	// 启动结果广播协程
	go ws.broadcastLoop()

	// 启动报警广播协程
	go ws.alertLoop()

	log.Printf("[WebSocket] 服务已启动")
}

// Stop 停止服务器
func (ws *WebSocketServer) Stop() {
	ws.hub.Stop()
	log.Printf("[WebSocket] 服务已停止")
}

// broadcastLoop 广播检测结果
func (ws *WebSocketServer) broadcastLoop() {
	for result := range ws.resultChan {
		if result == nil {
			continue
		}

		// 构建帧消息
		frameMsg := &FrameMessage{
			FrameID:       result.FrameID,
			ImageData:     base64.StdEncoding.EncodeToString(result.Frame.Data),
			Width:         result.Frame.Width,
			Height:        result.Frame.Height,
			InferenceTime: result.InferenceTime,
			Detections:    make([]*DetectionData, 0, len(result.Detections)),
		}

		for _, det := range result.Detections {
			frameMsg.Detections = append(frameMsg.Detections, &DetectionData{
				ID:          det.ID,
				X1:          det.X1,
				Y1:          det.Y1,
				X2:          det.X2,
				Y2:          det.Y2,
				ClassName:   det.ClassName,
				ClassID:     det.ClassID,
				Confidence:  det.Confidence,
				Entropy:     det.Entropy,
				IsUncertain: det.IsUncertain,
			})
		}

		// 广播
		ws.hub.Broadcast(&BroadcastMessage{
			Type:      "frame",
			Timestamp: time.Now().UnixMilli(),
			Data:      frameMsg,
		})
	}
}

// alertLoop 广播报警消息
func (ws *WebSocketServer) alertLoop() {
	for alert := range ws.alertChan {
		if alert == nil {
			continue
		}

		ws.hub.Broadcast(&BroadcastMessage{
			Type:      "alert",
			Timestamp: time.Now().UnixMilli(),
			Data:      alert,
		})
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (ws *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // 允许跨域
	})
	if err != nil {
		log.Printf("[WebSocket] 连接失败: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    ws.hub,
		ctx:    ctx,
		cancel: cancel,
	}

	ws.hub.register <- client

	// 启动写协程
	go client.writePump()

	// 在主 goroutine 中保持连接活跃
	// 通过读取客户端消息来保持连接（浏览器会自动发送心跳）
	defer func() {
		ws.hub.unregister <- client
	}()

	// 持续读取客户端消息，保持连接活跃
	// 浏览器 WebSocket 会自动发送 Ping/Pong 帧
	for {
		// 设置较长的读取超时（60秒）
		readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
		_, _, err := conn.Read(readCtx)
		readCancel()

		if err != nil {
			// 检查是否是超时（超时不是错误，继续读取）
			if readCtx.Err() == context.DeadlineExceeded {
				continue
			}
			// 检查是否是连接关闭
			if websocket.CloseStatus(err) != -1 || ctx.Err() != nil {
				return
			}
			// 其他错误也继续读取
			continue
		}
		// 收到消息（可能是心跳或其他），继续读取
	}
}

// writePump 写消息到 WebSocket
func (c *Client) writePump() {
	defer func() {
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.send:
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
			err := c.conn.Write(ctx, websocket.MessageText, message)
			cancel()

			if err != nil {
				return
			}
		}
	}
}

// GetHub 获取 Hub 实例
func (ws *WebSocketServer) GetHub() *Hub {
	return ws.hub
}
