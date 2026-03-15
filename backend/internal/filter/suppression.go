package filter

import (
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/models"
)

type alertEntry struct {
	detection models.DetectionMeta
	timestamp time.Time
}

// SpatiotemporalFilter suppresses duplicate alerts for the same object
// within a time window based on IoU overlap.
type SpatiotemporalFilter struct {
	mu           sync.Mutex
	activeAlerts []alertEntry
	timeWindow   time.Duration
	iouThreshold float64
}

func NewSpatiotemporalFilter(timeWindowSec float64, iouThreshold float64) *SpatiotemporalFilter {
	return &SpatiotemporalFilter{
		timeWindow:   time.Duration(timeWindowSec * float64(time.Second)),
		iouThreshold: iouThreshold,
	}
}

// ShouldAlert returns true if this detection is novel enough to trigger an alert.
func (f *SpatiotemporalFilter) ShouldAlert(det models.DetectionMeta) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	// Prune expired entries
	alive := f.activeAlerts[:0]
	for _, a := range f.activeAlerts {
		if now.Sub(a.timestamp) < f.timeWindow {
			alive = append(alive, a)
		}
	}
	f.activeAlerts = alive

	// Check spatial overlap with active alerts
	for _, a := range f.activeAlerts {
		if computeIoU(det, a.detection) > f.iouThreshold {
			return false // suppressed
		}
	}

	f.activeAlerts = append(f.activeAlerts, alertEntry{
		detection: det,
		timestamp: now,
	})
	return true
}

func (f *SpatiotemporalFilter) UpdateParams(timeWindowSec, iouThreshold float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.timeWindow = time.Duration(timeWindowSec * float64(time.Second))
	f.iouThreshold = iouThreshold
}

func computeIoU(a, b models.DetectionMeta) float64 {
	xa := max64(float64(a.X1), float64(b.X1))
	ya := max64(float64(a.Y1), float64(b.Y1))
	xb := min64(float64(a.X2), float64(b.X2))
	yb := min64(float64(a.Y2), float64(b.Y2))

	inter := max64(0, xb-xa) * max64(0, yb-ya)
	areaA := float64(a.X2-a.X1) * float64(a.Y2-a.Y1)
	areaB := float64(b.X2-b.X1) * float64(b.Y2-b.Y1)
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
