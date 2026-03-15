package models

import "time"

type Sample struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	FrameID   int64  `gorm:"uniqueIndex" json:"frame_id"`
	ImagePath string `json:"image_path"`
	Status    string `gorm:"default:pending" json:"status"` // pending | labeled | trained
	Label     *bool  `json:"label"`                         // true=positive, false=negative, nil=unlabeled
	Source    string `json:"source"`                        // "human" | "ai_agent"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TrainingRun struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SampleCount int       `json:"sample_count"`
	Accuracy    float64   `json:"accuracy"`
	ModelPath   string    `json:"model_path"`
	CreatedAt   time.Time `json:"created_at"`
}

type DetectionMeta struct {
	X1          float32 `json:"x1"`
	Y1          float32 `json:"y1"`
	X2          float32 `json:"x2"`
	Y2          float32 `json:"y2"`
	Confidence  float32 `json:"confidence"`
	ClassID     int32   `json:"class_id"`
	ClassName   string  `json:"class_name"`
	IsUncertain bool    `json:"is_uncertain"`
	Entropy     float32 `json:"entropy"`
	AnomalyScore float32 `json:"anomaly_score"`
}

type AlertEvent struct {
	Type       string          `json:"type"`
	FrameID    int64           `json:"frame_id"`
	ImageURL   string          `json:"image_url"`
	Detections []DetectionMeta `json:"detections"`
	Timestamp  string          `json:"timestamp"`
}
