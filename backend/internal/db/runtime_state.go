package db

import (
	"errors"

	"anomaly_detection_system/backend/internal/models"
	"gorm.io/gorm"
)

const runIDCounterKey = "pipeline_run_id_counter"

// NextPipelineRunID increments and returns a persistent pipeline run id.
// Value is stored in DB so service restarts continue from the previous counter.
func NextPipelineRunID() (int64, error) {
	var next int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var state models.RuntimeState
		err := tx.Where("key = ?", runIDCounterKey).First(&state).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				state = models.RuntimeState{Key: runIDCounterKey, IntValue: 1}
				if createErr := tx.Create(&state).Error; createErr != nil {
					return createErr
				}
				next = 1
				return nil
			}
			return err
		}

		next = state.IntValue + 1
		if updateErr := tx.Model(&models.RuntimeState{}).
			Where("key = ?", runIDCounterKey).
			Update("int_value", next).Error; updateErr != nil {
			return updateErr
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return next, nil
}
