package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"anomaly_detection_system/backend/internal/pipeline"
	"github.com/gin-gonic/gin"
)

// StreamHandler serves MJPEG stream to clients.
type StreamHandler struct {
	pipe *pipeline.Pipeline
}

func NewStreamHandler(pipe *pipeline.Pipeline) *StreamHandler {
	return &StreamHandler{pipe: pipe}
}

func (h *StreamHandler) ServeMJPEG(c *gin.Context) {
	bc := h.pipe.GetBroadcaster()
	if bc == nil {
		c.String(http.StatusServiceUnavailable, "Pipeline not running")
		return
	}

	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	sub := bc.SubscribeMJPEG()
	defer bc.UnsubscribeMJPEG(sub)

	for {
		select {
		case frame, ok := <-sub:
			if !ok {
				return
			}
			header := fmt.Sprintf("--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			if _, err := c.Writer.WriteString(header); err != nil {
				return
			}
			if _, err := c.Writer.Write(frame); err != nil {
				return
			}
			if _, err := c.Writer.WriteString("\r\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		case <-time.After(30 * time.Second):
			log.Println("MJPEG client timeout")
			return
		}
	}
}
