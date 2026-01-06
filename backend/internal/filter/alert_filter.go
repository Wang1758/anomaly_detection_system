package filter

import (
	"log"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/pipeline"
	"anomaly_detection_system/backend/internal/ws"
)

// ActiveAlert 活跃报警记录
type ActiveAlert struct {
	ID        int32     // 检测框ID
	CenterX   float32   // 中心点 X
	CenterY   float32   // 中心点 Y
	X1, Y1    float32   // 边界框左上角
	X2, Y2    float32   // 边界框右下角
	Timestamp time.Time // 时间戳
}

// AlertFilter 报警过滤器（空间及时间抑制）
type AlertFilter struct {
	mu           sync.RWMutex
	config       *config.Config
	activeAlerts []*ActiveAlert // 活跃报警列表
}

// NewAlertFilter 创建报警过滤器
func NewAlertFilter(cfg *config.Config) *AlertFilter {
	filter := &AlertFilter{
		config:       cfg,
		activeAlerts: make([]*ActiveAlert, 0),
	}

	// 启动清理协程
	go filter.cleanupLoop()

	return filter
}

// cleanupLoop 定期清理过期报警
func (f *AlertFilter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		f.cleanup()
	}
}

// cleanup 清理过期的报警记录
func (f *AlertFilter) cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	filterConfig := f.config.GetFilter()
	expireTime := time.Now().Add(-time.Duration(filterConfig.TimeWindowSeconds) * time.Second)

	newList := make([]*ActiveAlert, 0, len(f.activeAlerts))
	for _, alert := range f.activeAlerts {
		if alert.Timestamp.After(expireTime) {
			newList = append(newList, alert)
		}
	}

	if len(newList) != len(f.activeAlerts) {
		log.Printf("[AlertFilter] 清理过期报警: %d -> %d", len(f.activeAlerts), len(newList))
	}

	f.activeAlerts = newList
}

// ShouldAlert 检查是否应该发送报警
// 返回 true 表示应该发送报警，false 表示应该抑制
func (f *AlertFilter) ShouldAlert(detection *pipeline.Detection) bool {
	filterConfig := f.config.GetFilter()

	// 检查是否启用报警推送
	if !filterConfig.EnableAlertPush {
		return false
	}

	// 只处理不确定目标
	if !detection.IsUncertain {
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// 计算当前检测框的中心点
	centerX := (detection.X1 + detection.X2) / 2
	centerY := (detection.Y1 + detection.Y2) / 2

	// 清理过期记录
	expireTime := time.Now().Add(-time.Duration(filterConfig.TimeWindowSeconds) * time.Second)
	validAlerts := make([]*ActiveAlert, 0, len(f.activeAlerts))
	for _, alert := range f.activeAlerts {
		if alert.Timestamp.After(expireTime) {
			validAlerts = append(validAlerts, alert)
		}
	}
	f.activeAlerts = validAlerts

	// 检查空间重叠
	for _, alert := range f.activeAlerts {
		iou := f.calculateIoU(
			detection.X1, detection.Y1, detection.X2, detection.Y2,
			alert.X1, alert.Y1, alert.X2, alert.Y2,
		)

		if iou > filterConfig.SpatialIoUThreshold {
			log.Printf("[AlertFilter] 抑制报警: IoU=%.3f > %.3f (阈值)", iou, filterConfig.SpatialIoUThreshold)
			return false
		}
	}

	// 添加到活跃报警列表
	f.activeAlerts = append(f.activeAlerts, &ActiveAlert{
		ID:        detection.ID,
		CenterX:   centerX,
		CenterY:   centerY,
		X1:        detection.X1,
		Y1:        detection.Y1,
		X2:        detection.X2,
		Y2:        detection.Y2,
		Timestamp: time.Now(),
	})

	log.Printf("[AlertFilter] 新增报警: ID=%d, 位置=(%.1f, %.1f), 活跃报警数=%d",
		detection.ID, centerX, centerY, len(f.activeAlerts))

	return true
}

// calculateIoU 计算两个框的 IoU
func (f *AlertFilter) calculateIoU(x1a, y1a, x2a, y2a, x1b, y1b, x2b, y2b float32) float32 {
	// 计算交集
	x1 := max32(x1a, x1b)
	y1 := max32(y1a, y1b)
	x2 := min32(x2a, x2b)
	y2 := min32(y2a, y2b)

	if x2 <= x1 || y2 <= y1 {
		return 0.0
	}

	intersection := (x2 - x1) * (y2 - y1)
	areaA := (x2a - x1a) * (y2a - y1a)
	areaB := (x2b - x1b) * (y2b - y1b)
	union := areaA + areaB - intersection

	if union <= 0 {
		return 0.0
	}

	return intersection / union
}

// ProcessDetections 处理检测结果，返回需要报警的检测
func (f *AlertFilter) ProcessDetections(result *pipeline.DetectionResult) []*ws.AlertMessage {
	alerts := make([]*ws.AlertMessage, 0)

	for _, detection := range result.Detections {
		if f.ShouldAlert(detection) {
			// 创建报警消息
			alert := &ws.AlertMessage{
				ID:         detection.ID,
				FrameID:    result.FrameID,
				Timestamp:  time.Now().UnixMilli(),
				X1:         detection.X1,
				Y1:         detection.Y1,
				X2:         detection.X2,
				Y2:         detection.Y2,
				ClassName:  detection.ClassName,
				Confidence: detection.Confidence,
				Entropy:    detection.Entropy,
			}

			// TODO: 裁剪图像并编码为 Base64
			// 这里需要从 result.Frame.Data 中裁剪出扩展后的区域

			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// GetActiveAlertsCount 获取活跃报警数量
func (f *AlertFilter) GetActiveAlertsCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.activeAlerts)
}

// GetStats 获取统计信息
func (f *AlertFilter) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	filterConfig := f.config.GetFilter()

	return map[string]interface{}{
		"active_alerts_count":   len(f.activeAlerts),
		"spatial_iou_threshold": filterConfig.SpatialIoUThreshold,
		"time_window_seconds":   filterConfig.TimeWindowSeconds,
		"enable_alert_push":     filterConfig.EnableAlertPush,
	}
}

// 辅助函数
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
