package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/filter"
	"anomaly_detection_system/backend/internal/models"
	"anomaly_detection_system/backend/internal/perf"

	"gorm.io/gorm/clause"
)

const (
	mjpegSubscriberBufferSize = 16
	aiInputWidth              = 640.0
	aiInputHeight             = 640.0
)

// Broadcaster consumes ordered results and fans out to:
// - MJPEG stream (original high-res frames, no boxes drawn)
// - Detection channel (real-time bbox overlay data via WebSocket)
// - Alert channel (uncertain detection alerts via WebSocket)
type Broadcaster struct {
	mu          sync.RWMutex
	runID       int64
	latestFrame []byte
	outputFPS   float64
	targetFPS   int
	lastEmitAt  time.Time
	fpsWindowAt time.Time
	fpsFrames   int
	frameBytes  int64
	fanoutDrops int64
	alertQueued int64

	// Subscribers for MJPEG streaming
	subMu       sync.RWMutex
	subscribers map[chan []byte]struct{}

	// Alert channel for uncertain detection WebSocket push
	AlertCh     chan *models.AlertEvent
	alertWorkCh chan alertWork

	// Detection channel for real-time bbox overlay WebSocket push
	DetectionCh chan *models.DetectionFrame

	// Live channel for synchronized frame+detections push via WebSocket binary
	LiveCh chan *models.LiveFrame

	filter  *filter.SpatiotemporalFilter
	dataDir string
}

type alertWork struct {
	runID      int64
	frameID    int64
	origJPEG   []byte
	detections []models.DetectionMeta
	timestamp  string
}

func NewBroadcaster(f *filter.SpatiotemporalFilter, dataDir string, targetFPS int, runID int64) *Broadcaster {
	b := &Broadcaster{
		subscribers: make(map[chan []byte]struct{}),
		AlertCh:     make(chan *models.AlertEvent, 64),
		alertWorkCh: make(chan alertWork, 128),
		DetectionCh: make(chan *models.DetectionFrame, 256),
		LiveCh:      make(chan *models.LiveFrame, 64),
		filter:      f,
		dataDir:     dataDir,
		targetFPS:   targetFPS,
		runID:       runID,
	}
	go b.alertWorker()
	return b
}

// ResetForNewRun clears stale frame and updates run-scoped settings.
func (b *Broadcaster) ResetForNewRun(f *filter.SpatiotemporalFilter, dataDir string, targetFPS int, runID int64) {
	b.mu.Lock()
	b.latestFrame = nil
	b.outputFPS = 0
	b.targetFPS = targetFPS
	b.runID = runID
	b.lastEmitAt = time.Time{}
	b.fpsWindowAt = time.Time{}
	b.fpsFrames = 0
	b.filter = f
	b.dataDir = dataDir
	b.mu.Unlock()
}

// SubscribeMJPEG returns a channel that receives JPEG frames.
func (b *Broadcaster) SubscribeMJPEG() chan []byte {
	ch := make(chan []byte, mjpegSubscriberBufferSize)
	b.subMu.Lock()
	b.subscribers[ch] = struct{}{}
	b.subMu.Unlock()
	return ch
}

// UnsubscribeMJPEG removes a subscriber.
func (b *Broadcaster) UnsubscribeMJPEG(ch chan []byte) {
	b.subMu.Lock()
	delete(b.subscribers, ch)
	b.subMu.Unlock()
	close(ch)
}

// GetLatestFrame returns the most recent JPEG frame.
func (b *Broadcaster) GetLatestFrame() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.latestFrame
}

func (b *Broadcaster) GetOutputFPS() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.outputFPS
}

// PublishFrame pushes original JPEG frames to MJPEG subscribers at target FPS.
// This path is intentionally decoupled from AI inference so stream smoothness
// reflects capture throughput, not detection throughput.
func (b *Broadcaster) PublishFrame(frame []byte) {
	if len(frame) == 0 {
		return
	}

	now := time.Now()
	b.mu.Lock()
	b.latestFrame = frame
	b.lastEmitAt = now
	b.fpsFrames++
	b.frameBytes += int64(len(frame))
	if b.fpsWindowAt.IsZero() {
		b.fpsWindowAt = now
	}
	if elapsed := now.Sub(b.fpsWindowAt); elapsed >= time.Second {
		b.outputFPS = float64(b.fpsFrames) / elapsed.Seconds()
		if perf.Enabled() {
			mbps := (float64(b.frameBytes) / 1024.0 / 1024.0) / elapsed.Seconds()
			subscribers := b.subscriberCount()
			perf.Logf("Broadcaster perf: output_fps=%.1f bytes=%.2fMB/s subscribers=%d alert_q=%d detection_q=%d fanout_drops=%d",
				b.outputFPS, mbps, subscribers, len(b.alertWorkCh), len(b.DetectionCh), b.fanoutDrops)
		}
		b.fpsFrames = 0
		b.fpsWindowAt = now
		b.frameBytes = 0
		b.fanoutDrops = 0
	}
	b.mu.Unlock()

	b.subMu.RLock()
	for ch := range b.subscribers {
		select {
		case ch <- frame:
		default:
			if perf.Enabled() {
				b.mu.Lock()
				b.fanoutDrops++
				b.mu.Unlock()
			}
		}
	}
	b.subMu.RUnlock()
}

// mapDetections converts detection coordinates from AI input space (640x640)
// to the original high-res frame coordinate system.
func mapDetections(dets []*models.DetectionMeta, origWidth, origHeight int) {
	scaleX := float64(origWidth) / aiInputWidth
	scaleY := float64(origHeight) / aiInputHeight
	for _, d := range dets {
		d.X1 = float32(float64(d.X1) * scaleX)
		d.Y1 = float32(float64(d.Y1) * scaleY)
		d.X2 = float32(float64(d.X2) * scaleX)
		d.Y2 = float32(float64(d.Y2) * scaleY)
	}
}

// HandleResult processes a single ordered result from the pool.
func (b *Broadcaster) HandleResult(r *OrderedResult) {
	if r.Err != nil || r.Result == nil {
		return
	}

	origFrame := r.OrigJPEG
	b.mu.RLock()
	runID := b.runID
	b.mu.RUnlock()
	if len(origFrame) > 0 {
		b.PublishFrame(origFrame)
	}

	// Convert proto detections to model detections and map coordinates to orig resolution
	allDets := make([]models.DetectionMeta, 0, len(r.Result.Detections))
	detPtrs := make([]*models.DetectionMeta, 0, len(r.Result.Detections))
	for _, d := range r.Result.Detections {
		det := models.DetectionMeta{
			X1: d.X1, Y1: d.Y1, X2: d.X2, Y2: d.Y2,
			Confidence:   d.Confidence,
			ClassID:      d.ClassId,
			ClassName:    d.ClassName,
			IsUncertain:  d.IsUncertain,
			Entropy:      d.Entropy,
			AnomalyScore: d.AnomalyScore,
		}
		allDets = append(allDets, det)
	}
	for i := range allDets {
		detPtrs = append(detPtrs, &allDets[i])
	}
	mapDetections(detPtrs, r.OrigWidth, r.OrigHeight)

	// Push detection overlay data via WebSocket channel
	detFrame := &models.DetectionFrame{
		Type:        "detections",
		FrameID:     r.Result.FrameId,
		FrameWidth:  r.OrigWidth,
		FrameHeight: r.OrigHeight,
		Detections:  allDets,
	}
	select {
	case b.DetectionCh <- detFrame:
	default:
		// Drop if consumer is slow; real-time overlay is best-effort
	}

	// Push synchronized frame+detections packet for single-channel live rendering
	if len(origFrame) > 0 {
		liveFrame := &models.LiveFrame{
			Type:        "live_frame",
			FrameID:     r.Result.FrameId,
			FrameWidth:  r.OrigWidth,
			FrameHeight: r.OrigHeight,
			Detections:  allDets,
			JPEG:        append([]byte(nil), origFrame...),
		}
		select {
		case b.LiveCh <- liveFrame:
		default:
			// Drop if consumer is slow; keep live stream low-latency.
		}
	}

	// Process uncertain detections for alert persistence
	if !r.Result.HasUncertain {
		return
	}

	var uncertainDets []models.DetectionMeta
	for _, d := range allDets {
		if d.IsUncertain {
			if b.filter.ShouldAlert(d) {
				uncertainDets = append(uncertainDets, d)
			}
		}
	}

	if len(uncertainDets) == 0 {
		return
	}
	if len(origFrame) == 0 {
		return
	}

	work := alertWork{
		runID:      runID,
		frameID:    r.Result.FrameId,
		origJPEG:   append([]byte(nil), origFrame...),
		detections: uncertainDets,
		timestamp:  time.Now().Format(time.RFC3339),
	}

	select {
	case b.alertWorkCh <- work:
		if perf.Enabled() {
			b.mu.Lock()
			b.alertQueued++
			b.mu.Unlock()
		}
	default:
		log.Printf("Alert work queue full, dropping alert persistence for frame=%d", r.Result.FrameId)
	}
}

func (b *Broadcaster) subscriberCount() int {
	b.subMu.RLock()
	defer b.subMu.RUnlock()
	return len(b.subscribers)
}

func (b *Broadcaster) alertWorker() {
	for work := range b.alertWorkCh {
		b.persistAlert(work)
	}
}

func (b *Broadcaster) persistAlert(work alertWork) {
	start := time.Now()
	imgDir := filepath.Join(b.dataDir, "images")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		log.Printf("Failed to create image dir: %v", err)
		return
	}

	// Save the original high-res frame (no longer saving visualized image)
	origName := fmt.Sprintf("frame_%d_%d.jpg", work.frameID, time.Now().UnixNano())
	origPath := filepath.Join(imgDir, origName)
	if len(work.origJPEG) > 0 {
		if err := os.WriteFile(origPath, work.origJPEG, 0644); err != nil {
			log.Printf("Failed to save original image: %v", err)
			return
		}
	}

	sample := models.Sample{
		RunID:          work.runID,
		FrameID:        work.frameID,
		ImagePath:      origName,
		UncertainCount: len(work.detections),
		DetectionsJSON: encodeDetections(work.detections),
		Status:         "pending",
	}
	if err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "run_id"}, {Name: "frame_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"image_path", "uncertain_count", "detections_json", "status", "updated_at"}),
	}).Create(&sample).Error; err != nil {
		log.Printf("Failed to save sample: %v", err)
	}
	if sample.ID == 0 {
		_ = db.DB.Where("run_id = ? AND frame_id = ?", work.runID, work.frameID).First(&sample).Error
	}
	if perf.Enabled() {
		if latency := time.Since(start); latency > 80*time.Millisecond {
			perf.Logf("Alert persist slow: frame=%d dets=%d latency=%s", work.frameID, len(work.detections), latency)
		}
	}

	event := &models.AlertEvent{
		Type:       "alert",
		SampleID:   sample.ID,
		RunID:      work.runID,
		FrameID:    work.frameID,
		ImageURL:   fmt.Sprintf("/api/images/%s", origName),
		Detections: work.detections,
		Timestamp:  work.timestamp,
	}

	select {
	case b.AlertCh <- event:
	default:
		log.Println("Alert channel full, dropping event")
	}
}

func encodeDetections(dets []models.DetectionMeta) string {
	if len(dets) == 0 {
		return "[]"
	}
	b, err := json.Marshal(dets)
	if err != nil {
		return "[]"
	}
	return string(b)
}
