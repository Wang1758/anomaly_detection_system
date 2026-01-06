package model

import (
	"time"

	"gorm.io/gorm"
)

// Sample 样本数据模型
type Sample struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 图片信息
	ImagePath   string `gorm:"not null" json:"image_path"` // 图片存储路径
	ImageWidth  int    `json:"image_width"`                // 图片宽度
	ImageHeight int    `json:"image_height"`               // 图片高度

	// 检测框信息
	BBoxX1 float32 `json:"bbox_x1"` // 边界框左上角 X
	BBoxY1 float32 `json:"bbox_y1"` // 边界框左上角 Y
	BBoxX2 float32 `json:"bbox_x2"` // 边界框右下角 X
	BBoxY2 float32 `json:"bbox_y2"` // 边界框右下角 Y

	// AI 检测结果
	ClassName   string  `json:"class_name"`   // 类别名称
	ClassID     int     `json:"class_id"`     // 类别 ID
	Confidence  float32 `json:"confidence"`   // 置信度
	Entropy     float32 `json:"entropy"`      // 熵值
	IsUncertain bool    `json:"is_uncertain"` // 是否为不确定目标

	// 人工标注结果
	LabelStatus string     `gorm:"default:'pending'" json:"label_status"` // pending/normal/abnormal
	LabeledBy   string     `json:"labeled_by"`                            // 标注者
	LabeledAt   *time.Time `json:"labeled_at"`                            // 标注时间

	// 训练状态
	UsedForTraining bool `gorm:"default:false" json:"used_for_training"` // 是否已用于训练
}

// TableName 指定表名
func (Sample) TableName() string {
	return "samples"
}

// SystemConfig 系统配置存储
type SystemConfig struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}

// TrainingLog 训练日志
type TrainingLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	// 训练信息
	SampleCount int        `json:"sample_count"`                    // 样本数量
	StartTime   time.Time  `json:"start_time"`                      // 开始时间
	EndTime     *time.Time `json:"end_time"`                        // 结束时间
	Status      string     `gorm:"default:'running'" json:"status"` // running/completed/failed

	// 模型信息
	OldModelPath string `json:"old_model_path"` // 旧模型路径
	NewModelPath string `json:"new_model_path"` // 新模型路径

	// 训练结果
	ErrorMessage string `json:"error_message"` // 错误信息
}

// TableName 指定表名
func (TrainingLog) TableName() string {
	return "training_logs"
}
