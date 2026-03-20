package handler

import "anomaly_detection_system/backend/internal/config"

// configUpdateBody is the JSON body for PUT /api/config.
// Pointer fields are optional; nil means "do not change".
type configUpdateBody struct {
	NMSThreshold        *float32 `json:"nms_threshold"`
	ConfidenceThreshold *float32 `json:"confidence_threshold"`
	EntropyThreshold    *float32 `json:"entropy_threshold"`
	W1                  *float32 `json:"w1"`
	W2                  *float32 `json:"w2"`
	FPS                 *int     `json:"fps"`
	Workers             *int     `json:"workers"`
	BatchSize           *int     `json:"batch_size"`
	BatchTimeout        *int     `json:"batch_timeout_ms"`
	FilterTimeWindow    *float64 `json:"filter_time_window"`
	FilterIoU           *float64 `json:"filter_iou"`
	SourceType          *string  `json:"source_type"`
	SourceAddr          *string  `json:"source_addr"`
}

func mergeConfigUpdate(dst *config.Config, req *configUpdateBody) {
	if req == nil {
		return
	}
	if req.NMSThreshold != nil {
		dst.NMSThreshold = *req.NMSThreshold
	}
	if req.ConfidenceThreshold != nil {
		dst.ConfidenceThreshold = *req.ConfidenceThreshold
	}
	if req.EntropyThreshold != nil {
		dst.EntropyThreshold = *req.EntropyThreshold
	}
	if req.W1 != nil {
		dst.W1 = *req.W1
	}
	if req.W2 != nil {
		dst.W2 = *req.W2
	}
	if req.FPS != nil {
		dst.FPS = *req.FPS
	}
	if req.Workers != nil {
		dst.Workers = *req.Workers
	}
	if req.BatchSize != nil {
		dst.BatchSize = *req.BatchSize
	}
	if req.BatchTimeout != nil {
		dst.BatchTimeout = *req.BatchTimeout
	}
	if req.FilterTimeWindow != nil {
		dst.FilterTimeWindow = *req.FilterTimeWindow
	}
	if req.FilterIoU != nil {
		dst.FilterIoU = *req.FilterIoU
	}
	if req.SourceType != nil {
		dst.SourceType = *req.SourceType
	}
	if req.SourceAddr != nil {
		dst.SourceAddr = *req.SourceAddr
	}
}
