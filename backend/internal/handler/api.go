package handler

import (
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/grpcclient"
	"anomaly_detection_system/backend/internal/models"
	"anomaly_detection_system/backend/internal/pipeline"
	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	cfg        *config.Config
	pipe       *pipeline.Pipeline
	grpcClient *grpcclient.Client
}

func NewAPIHandler(cfg *config.Config, pipe *pipeline.Pipeline, grpcClient *grpcclient.Client) *APIHandler {
	return &APIHandler{cfg: cfg, pipe: pipe, grpcClient: grpcClient}
}

// --- Config endpoints ---

func (h *APIHandler) GetConfig(c *gin.Context) {
	snap := h.cfg.Read()
	c.JSON(http.StatusOK, snap)
}

func (h *APIHandler) UpdateConfig(c *gin.Context) {
	var req config.Config
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfg.Update(func(cfg *config.Config) {
		if req.NMSThreshold > 0 {
			cfg.NMSThreshold = req.NMSThreshold
		}
		if req.ConfidenceThreshold > 0 {
			cfg.ConfidenceThreshold = req.ConfidenceThreshold
		}
		if req.EntropyThreshold > 0 {
			cfg.EntropyThreshold = req.EntropyThreshold
		}
		if req.W1 > 0 {
			cfg.W1 = req.W1
		}
		if req.W2 > 0 {
			cfg.W2 = req.W2
		}
		if req.FPS > 0 {
			cfg.FPS = req.FPS
		}
		if req.FilterTimeWindow > 0 {
			cfg.FilterTimeWindow = req.FilterTimeWindow
		}
		if req.FilterIoU > 0 {
			cfg.FilterIoU = req.FilterIoU
		}
		if req.SourceType != "" {
			cfg.SourceType = req.SourceType
		}
		if req.SourceAddr != "" {
			cfg.SourceAddr = req.SourceAddr
		}
	})

	// Sync AI params to Python service
	snap := h.cfg.Read()
	_, err := h.grpcClient.UpdateParams(c.Request.Context(),
		snap.NMSThreshold, snap.ConfidenceThreshold, snap.EntropyThreshold,
		snap.W1, snap.W2)
	if err != nil {
		log.Printf("Failed to sync params to AI service: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// --- Sample endpoints ---

func (h *APIHandler) ListSamples(c *gin.Context) {
	status := c.Query("status")
	var samples []models.Sample
	query := db.DB
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query.Order("created_at DESC").Find(&samples)
	c.JSON(http.StatusOK, samples)
}

func (h *APIHandler) LabelSample(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Label bool `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := db.DB.Model(&models.Sample{}).Where("id = ?", id).Updates(map[string]interface{}{
		"label":  req.Label,
		"status": "labeled",
		"source": "human",
	})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "labeled"})
}

func (h *APIHandler) RelabelSample(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Label bool `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.DB.Model(&models.Sample{}).Where("id = ?", id).Update("label", req.Label)
	c.JSON(http.StatusOK, gin.H{"message": "relabeled"})
}

func (h *APIHandler) AIJudge(c *gin.Context) {
	// TODO: integrate with multimodal LLM for auto-labeling
	var pending []models.Sample
	db.DB.Where("status = ?", "pending").Find(&pending)

	for i := range pending {
		label := true // placeholder
		pending[i].Label = &label
		pending[i].Status = "labeled"
		pending[i].Source = "ai_agent"
		db.DB.Save(&pending[i])
	}

	c.JSON(http.StatusOK, gin.H{"message": "ai judged", "count": len(pending)})
}

// --- Training endpoints ---

func (h *APIHandler) TriggerTraining(c *gin.Context) {
	var count int64
	db.DB.Model(&models.Sample{}).Where("status = ?", "labeled").Count(&count)

	// TODO: trigger actual training script
	run := models.TrainingRun{
		SampleCount: int(count),
		Accuracy:    0.0,
		ModelPath:   filepath.Join(h.cfg.Read().DataDir, "models", "latest.pt"),
	}
	db.DB.Create(&run)

	db.DB.Model(&models.Sample{}).Where("status = ?", "labeled").Update("status", "trained")

	c.JSON(http.StatusOK, gin.H{"message": "training triggered", "sample_count": count, "run_id": run.ID})
}

func (h *APIHandler) TrainingHistory(c *gin.Context) {
	var runs []models.TrainingRun
	db.DB.Order("created_at DESC").Find(&runs)
	c.JSON(http.StatusOK, runs)
}

// --- Pipeline control ---

func (h *APIHandler) StartPipeline(c *gin.Context) {
	if err := h.pipe.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pipeline started"})
}

func (h *APIHandler) StopPipeline(c *gin.Context) {
	h.pipe.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "pipeline stopped"})
}

// --- Image serving ---

func (h *APIHandler) ServeImage(c *gin.Context) {
	filename := c.Param("filename")
	imgPath := filepath.Join(h.cfg.Read().DataDir, "images", filename)
	c.File(imgPath)
}
