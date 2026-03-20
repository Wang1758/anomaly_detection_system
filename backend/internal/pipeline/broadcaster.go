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

// Broadcaster consumes ordered results and fans out to video stream + alert channel.
type Broadcaster struct {
	// Latest MJPEG frame for streaming
	mu          sync.RWMutex
	latestFrame []byte

	// Subscribers for MJPEG streaming
	subMu       sync.RWMutex
	subscribers map[chan []byte]struct{}

	// Alert channel for WebSocket push
	AlertCh chan *models.AlertEvent

	filter  *filter.SpatiotemporalFilter
	dataDir string
}

func NewBroadcaster(f *filter.SpatiotemporalFilter, dataDir string) *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan []byte]struct{}),
		AlertCh:     make(chan *models.AlertEvent, 64),
		filter:      f,
		dataDir:     dataDir,
	}
}

// ResetForNewRun clears stale frame and updates run-scoped settings.
func (b *Broadcaster) ResetForNewRun(f *filter.SpatiotemporalFilter, dataDir string) {
	b.mu.Lock()
	b.latestFrame = nil
	b.filter = f
	b.dataDir = dataDir
	b.mu.Unlock()
}

// SubscribeMJPEG returns a channel that receives JPEG frames.
func (b *Broadcaster) SubscribeMJPEG() chan []byte {
	ch := make(chan []byte, 2)
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

	imgDir := filepath.Join(b.dataDir, "images")
	os.MkdirAll(imgDir, 0755)

	// Save original (clean) image for training data
	origName := fmt.Sprintf("frame_%d.jpg", r.Result.FrameId)
	origPath := filepath.Join(imgDir, origName)
	if len(r.Result.OriginalImage) > 0 {
		if err := os.WriteFile(origPath, r.Result.OriginalImage, 0644); err != nil {
			log.Printf("Failed to save original image: %v", err)
			return
		}
	}

	// Save visualized image (with annotated boxes) for alert display
	visName := fmt.Sprintf("vis_frame_%d.jpg", r.Result.FrameId)
	visPath := filepath.Join(imgDir, visName)
	if len(visFrame) > 0 {
		if err := os.WriteFile(visPath, visFrame, 0644); err != nil {
			log.Printf("Failed to save visualized image: %v", err)
		}
	}

	// Save to database (basename only; full file lives under dataDir/images)
	sample := models.Sample{
		FrameID:   r.Result.FrameId,
		ImagePath: origName,
		Status:    "pending",
	}
	if err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "frame_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"image_path", "status", "updated_at"}),
	}).Create(&sample).Error; err != nil {
		log.Printf("Failed to save sample: %v", err)
	}

	// Push alert event with the annotated visualization for the frontend
	event := &models.AlertEvent{
		Type:       "alert",
		FrameID:    r.Result.FrameId,
		ImageURL:   fmt.Sprintf("/api/images/%s", visName),
		Detections: uncertainDets,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	select {
	case b.AlertCh <- event:
	default:
		log.Println("Alert channel full, dropping event")
	}
}
