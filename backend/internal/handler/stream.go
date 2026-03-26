package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"anomaly_detection_system/backend/internal/perf"
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
	client := c.ClientIP()

	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	sub := bc.SubscribeMJPEG()
	defer bc.UnsubscribeMJPEG(sub)

	writeFrame := func(frame []byte) bool {
		header := fmt.Sprintf("--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
		if _, err := c.Writer.WriteString(header); err != nil {
			return false
		}
		if _, err := c.Writer.Write(frame); err != nil {
			return false
		}
		if _, err := c.Writer.WriteString("\r\n"); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	windowStart := time.Now()
	windowFrames := 0
	windowBytes := 0

	recordSend := func(frameSize int) {
		if !perf.Enabled() {
			return
		}
		windowFrames++
		windowBytes += frameSize
		now := time.Now()
		if elapsed := now.Sub(windowStart); elapsed >= time.Second {
			fps := float64(windowFrames) / elapsed.Seconds()
			mbps := (float64(windowBytes) / 1024.0 / 1024.0) / elapsed.Seconds()
			perf.Logf("MJPEG stream perf: client=%s fps=%.1f bandwidth=%.2fMB/s", client, fps, mbps)
			windowStart = now
			windowFrames = 0
			windowBytes = 0
		}
	}

	if latest := bc.GetLatestFrame(); len(latest) > 0 {
		if !writeFrame(latest) {
			return
		}
		recordSend(len(latest))
	}

	for {
		select {
		case frame, ok := <-sub:
			if !ok {
				return
			}
			if !writeFrame(frame) {
				return
			}
			recordSend(len(frame))
		case <-c.Request.Context().Done():
			return
		case <-time.After(30 * time.Second):
			if perf.Enabled() {
				perf.Logf("MJPEG client timeout: client=%s", client)
			} else {
				log.Println("MJPEG client timeout")
			}
			return
		}
	}
}
