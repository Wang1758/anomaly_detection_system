package pipeline

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/filter"
	"anomaly_detection_system/backend/internal/models"

	"gorm.io/gorm/clause"
)

const mjpegSubscriberBufferSize = 16

// Broadcaster consumes ordered results and fans out to video stream + alert channel.
type Broadcaster struct {
	// Latest MJPEG frame for streaming
	mu          sync.RWMutex
	latestFrame []byte
	outputFPS   float64
	fpsWindowAt time.Time
	fpsFrames   int

	// Subscribers for MJPEG streaming
	subMu       sync.RWMutex
	subscribers map[chan []byte]struct{}

	// Alert channel for WebSocket push
	AlertCh     chan *models.AlertEvent
	alertWorkCh chan alertWork

	filter  *filter.SpatiotemporalFilter
	dataDir string
}

type alertWork struct {
	frameID    int64
	visFrame   []byte
	original   []byte
	detections []models.DetectionMeta
	timestamp  string
}

func NewBroadcaster(f *filter.SpatiotemporalFilter, dataDir string) *Broadcaster {
	b := &Broadcaster{
		subscribers: make(map[chan []byte]struct{}),
		AlertCh:     make(chan *models.AlertEvent, 64),
		alertWorkCh: make(chan alertWork, 128),
		filter:      f,
		dataDir:     dataDir,
	}
	go b.alertWorker()
	return b
}

// ResetForNewRun clears stale frame and updates run-scoped settings.
func (b *Broadcaster) ResetForNewRun(f *filter.SpatiotemporalFilter, dataDir string) {
	b.mu.Lock()
	b.latestFrame = nil
	b.outputFPS = 0
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

// HandleResult processes a single ordered result from the pool.
func (b *Broadcaster) HandleResult(r *OrderedResult) {
	if r.Err != nil || r.Result == nil {
		return
	}

	visFrame := r.Result.VisualizedImage
	if len(visFrame) == 0 {
		return
	}

	// Update latest frame and broadcast to MJPEG subscribers
	b.mu.Lock()
	b.latestFrame = visFrame
	now := time.Now()
	if b.fpsWindowAt.IsZero() {
		b.fpsWindowAt = now
	}
	b.fpsFrames++
	if elapsed := now.Sub(b.fpsWindowAt); elapsed >= time.Second {
		b.outputFPS = float64(b.fpsFrames) / elapsed.Seconds()
		b.fpsFrames = 0
		b.fpsWindowAt = now
	}
	b.mu.Unlock()

	b.subMu.RLock()
	for ch := range b.subscribers {
		select {
		case ch <- visFrame:
		default:
			// Drop frame if subscriber is slow
		}
	}
	b.subMu.RUnlock()

	// Process uncertain detections
	if !r.Result.HasUncertain {
		return
	}

	var uncertainDets []models.DetectionMeta
	for _, d := range r.Result.Detections {
		if d.IsUncertain {
			det := models.DetectionMeta{
				X1: d.X1, Y1: d.Y1, X2: d.X2, Y2: d.Y2,
				Confidence:   d.Confidence,
				ClassID:      d.ClassId,
				ClassName:    d.ClassName,
				IsUncertain:  d.IsUncertain,
				Entropy:      d.Entropy,
				AnomalyScore: d.AnomalyScore,
			}
			if b.filter.ShouldAlert(det) {
				uncertainDets = append(uncertainDets, det)
			}
		}
	}

	if len(uncertainDets) == 0 {
		return
	}

	work := alertWork{
		frameID:    r.Result.FrameId,
		visFrame:   append([]byte(nil), visFrame...),
		original:   append([]byte(nil), r.Result.OriginalImage...),
		detections: uncertainDets,
		timestamp:  time.Now().Format(time.RFC3339),
	}

	select {
	case b.alertWorkCh <- work:
	default:
		log.Printf("Alert work queue full, dropping alert persistence for frame=%d", r.Result.FrameId)
	}
}

func (b *Broadcaster) alertWorker() {
	for work := range b.alertWorkCh {
		b.persistAlert(work)
	}
}

func (b *Broadcaster) persistAlert(work alertWork) {
	imgDir := filepath.Join(b.dataDir, "images")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		log.Printf("Failed to create image dir: %v", err)
		return
	}

	origName := fmt.Sprintf("frame_%d.jpg", work.frameID)
	origPath := filepath.Join(imgDir, origName)
	if len(work.original) > 0 {
		if err := os.WriteFile(origPath, work.original, 0644); err != nil {
			log.Printf("Failed to save original image: %v", err)
			return
		}
	}

	visName := fmt.Sprintf("vis_frame_%d.jpg", work.frameID)
	visPath := filepath.Join(imgDir, visName)
	if len(work.visFrame) > 0 {
		if err := os.WriteFile(visPath, work.visFrame, 0644); err != nil {
			log.Printf("Failed to save visualized image: %v", err)
		}
	}

	sample := models.Sample{
		FrameID:             work.frameID,
		ImagePath:           origName,
		VisualizedImagePath: visName,
		UncertainCount:      len(work.detections),
		Status:              "pending",
	}
	if err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "frame_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"image_path", "visualized_image_path", "uncertain_count", "status", "updated_at"}),
	}).Create(&sample).Error; err != nil {
		log.Printf("Failed to save sample: %v", err)
	}

	event := &models.AlertEvent{
		Type:       "alert",
		FrameID:    work.frameID,
		ImageURL:   fmt.Sprintf("/api/images/%s", visName),
		Detections: work.detections,
		Timestamp:  work.timestamp,
	}

	select {
	case b.AlertCh <- event:
	default:
		log.Println("Alert channel full, dropping event")
	}
}
