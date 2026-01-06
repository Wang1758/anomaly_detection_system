package model

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	// 自动迁移表结构
	err = DB.AutoMigrate(&Sample{}, &SystemConfig{}, &TrainingLog{})
	if err != nil {
		return err
	}

	log.Println("数据库初始化成功:", dbPath)
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}

// CreateSample 创建样本记录
func CreateSample(sample *Sample) error {
	return DB.Create(sample).Error
}

// GetPendingSamples 获取待标注样本
func GetPendingSamples(limit int) ([]Sample, error) {
	var samples []Sample
	err := DB.Where("label_status = ?", "pending").
		Order("created_at DESC").
		Limit(limit).
		Find(&samples).Error
	return samples, err
}

// GetLabeledSamplesCount 获取已标注样本数量
func GetLabeledSamplesCount() (int64, error) {
	var count int64
	err := DB.Model(&Sample{}).
		Where("label_status IN (?, ?)", "normal", "abnormal").
		Where("used_for_training = ?", false).
		Count(&count).Error
	return count, err
}

// GetUntrainedSamples 获取未训练的已标注样本
func GetUntrainedSamples() ([]Sample, error) {
	var samples []Sample
	err := DB.Where("label_status IN (?, ?)", "normal", "abnormal").
		Where("used_for_training = ?", false).
		Find(&samples).Error
	return samples, err
}

// MarkSamplesAsTrained 标记样本为已训练
func MarkSamplesAsTrained(sampleIDs []uint) error {
	return DB.Model(&Sample{}).
		Where("id IN ?", sampleIDs).
		Update("used_for_training", true).Error
}

// UpdateSampleLabel 更新样本标注
func UpdateSampleLabel(id uint, status string, labeledBy string) error {
	return DB.Model(&Sample{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"label_status": status,
			"labeled_by":   labeledBy,
			"labeled_at":   gorm.Expr("datetime('now')"),
		}).Error
}

// CreateTrainingLog 创建训练日志
func CreateTrainingLog(log *TrainingLog) error {
	return DB.Create(log).Error
}

// UpdateTrainingLog 更新训练日志
func UpdateTrainingLog(id uint, updates map[string]interface{}) error {
	return DB.Model(&TrainingLog{}).Where("id = ?", id).Updates(updates).Error
}

// GetLatestTrainingLog 获取最新训练日志
func GetLatestTrainingLog() (*TrainingLog, error) {
	var trainingLog TrainingLog
	err := DB.Order("created_at DESC").First(&trainingLog).Error
	if err != nil {
		return nil, err
	}
	return &trainingLog, nil
}
