package handler

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/grpcclient"
	pb "anomaly_detection_system/backend/internal/grpcclient/pb"
	"anomaly_detection_system/backend/internal/models"
	"anomaly_detection_system/backend/internal/pipeline"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	cfg        *config.Config
	pipe       *pipeline.Pipeline
	grpcClient *grpcclient.Client
	llmJudger  *LLMJudger
}

func NewAPIHandler(cfg *config.Config, pipe *pipeline.Pipeline, grpcClient *grpcclient.Client, llm *LLMJudger) *APIHandler {
	return &APIHandler{cfg: cfg, pipe: pipe, grpcClient: grpcClient, llmJudger: llm}
}

// --- Config endpoints ---

func (h *APIHandler) GetConfig(c *gin.Context) {
	snap := h.cfg.Read()
	c.JSON(http.StatusOK, snap)
}

func (h *APIHandler) UpdateConfig(c *gin.Context) {
	var req configUpdateBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FPS != nil && *req.FPS <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fps must be > 0"})
		return
	}
	if req.Workers != nil && *req.Workers <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workers must be > 0"})
		return
	}
	if req.BatchSize != nil && *req.BatchSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "batch_size must be > 0"})
		return
	}
	if req.BatchTimeout != nil && *req.BatchTimeout < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "batch_timeout_ms must be >= 0"})
		return
	}

	h.cfg.Update(func(cfg *config.Config) {
		mergeConfigUpdate(cfg, &req)
	})

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
	if err := query.Order("created_at DESC").Find(&samples).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "sample not found"})
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

	result := db.DB.Model(&models.Sample{}).Where("id = ?", id).Updates(map[string]interface{}{
		"label":  req.Label,
		"source": "human",
	})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "sample not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "relabeled"})
}

type aiJudgeItemResult struct {
	ID      uint   `json:"id"`
	FrameID int64  `json:"frame_id"`
	Label   bool   `json:"label,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Error   string `json:"error,omitempty"`
}

const maxLLMConcurrency = 5

// AIJudge uses a multimodal LLM to judge pending samples.
// Falls back to YOLO gRPC re-detection when no LLM API key is configured.
func (h *APIHandler) AIJudge(c *gin.Context) {
	var pending []models.Sample
	if err := db.DB.Where("status = ?", "pending").Find(&pending).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(pending) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no pending samples", "count": 0, "results": []aiJudgeItemResult{}})
		return
	}

	snap := h.cfg.Read()
	ctx := c.Request.Context()

	if h.llmJudger.Available() {
		results := h.aiJudgeViaLLM(ctx, pending, snap)
		c.JSON(http.StatusOK, gin.H{"message": "ai judged", "method": "llm", "count": len(pending), "results": results})
	} else {
		results := h.aiJudgeViaYOLO(ctx, pending, snap)
		c.JSON(http.StatusOK, gin.H{"message": "ai judged", "method": "yolo_fallback", "count": len(pending), "results": results})
	}
}

// aiJudgeViaLLM sends each pending sample to the multimodal LLM concurrently.
func (h *APIHandler) aiJudgeViaLLM(ctx context.Context, pending []models.Sample, snap *config.Config) []aiJudgeItemResult {
	type task struct {
		idx    int
		sample models.Sample
		data   []byte
	}

	var tasks []task
	results := make([]aiJudgeItemResult, len(pending))

	for i, s := range pending {
		path := resolveSampleImagePath(snap.DataDir, s)
		if path == "" {
			results[i] = aiJudgeItemResult{ID: s.ID, FrameID: s.FrameID, Error: "empty image path"}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			results[i] = aiJudgeItemResult{ID: s.ID, FrameID: s.FrameID, Error: err.Error()}
			continue
		}
		tasks = append(tasks, task{idx: i, sample: s, data: data})
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxLLMConcurrency)

	for _, t := range tasks {
		wg.Add(1)
		go func(t task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			jr, err := h.llmJudger.JudgeImage(ctx, t.data)
			if err != nil {
				log.Printf("LLM judge error for sample %d: %v", t.sample.ID, err)
				results[t.idx] = aiJudgeItemResult{ID: t.sample.ID, FrameID: t.sample.FrameID, Error: err.Error()}
				return
			}

			if err := db.DB.Model(&models.Sample{}).Where("id = ?", t.sample.ID).Updates(map[string]interface{}{
				"label":  jr.Label,
				"status": "labeled",
				"source": "ai_agent",
			}).Error; err != nil {
				results[t.idx] = aiJudgeItemResult{ID: t.sample.ID, FrameID: t.sample.FrameID, Error: err.Error()}
				return
			}
			results[t.idx] = aiJudgeItemResult{ID: t.sample.ID, FrameID: t.sample.FrameID, Label: jr.Label, Reason: jr.Reason}
		}(t)
	}

	wg.Wait()
	return results
}

// aiJudgeViaYOLO re-runs YOLO detection via gRPC BatchDetect as fallback.
func (h *APIHandler) aiJudgeViaYOLO(ctx context.Context, pending []models.Sample, snap *config.Config) []aiJudgeItemResult {
	batchSize := snap.BatchSize
	if batchSize <= 0 {
		batchSize = 8
	}

	var results []aiJudgeItemResult

	for start := 0; start < len(pending); start += batchSize {
		end := start + batchSize
		if end > len(pending) {
			end = len(pending)
		}

		type loaded struct {
			sample models.Sample
			bytes  []byte
		}
		var loads []loaded
		for _, s := range pending[start:end] {
			path := resolveSampleImagePath(snap.DataDir, s)
			if path == "" {
				results = append(results, aiJudgeItemResult{ID: s.ID, FrameID: s.FrameID, Error: "empty image path"})
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				results = append(results, aiJudgeItemResult{ID: s.ID, FrameID: s.FrameID, Error: err.Error()})
				continue
			}
			loads = append(loads, loaded{sample: s, bytes: data})
		}
		if len(loads) == 0 {
			continue
		}

		images := make([][]byte, len(loads))
		frameIDs := make([]int64, len(loads))
		for i, L := range loads {
			images[i] = L.bytes
			frameIDs[i] = L.sample.FrameID
		}

		resp, err := h.grpcClient.BatchDetect(ctx, images, frameIDs)
		if err != nil {
			for _, L := range loads {
				results = append(results, aiJudgeItemResult{ID: L.sample.ID, FrameID: L.sample.FrameID, Error: err.Error()})
			}
			continue
		}

		out := resp.GetResults()
		for i, L := range loads {
			var r *pb.DetectResponse
			if i < len(out) {
				r = out[i]
			}
			label, reason := inferLabelFromDetectResponse(r)

			if err := db.DB.Model(&models.Sample{}).Where("id = ?", L.sample.ID).Updates(map[string]interface{}{
				"label":  label,
				"status": "labeled",
				"source": "ai_agent",
			}).Error; err != nil {
				results = append(results, aiJudgeItemResult{ID: L.sample.ID, FrameID: L.sample.FrameID, Error: err.Error()})
				continue
			}
			results = append(results, aiJudgeItemResult{ID: L.sample.ID, FrameID: L.sample.FrameID, Label: label, Reason: reason})
		}
	}
	return results
}

// --- Training endpoints ---

func (h *APIHandler) TriggerTraining(c *gin.Context) {
	var count int64
	db.DB.Model(&models.Sample{}).Where("status = ?", "labeled").Count(&count)

	if count == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no labeled samples to train"})
		return
	}

	run := models.TrainingRun{
		SampleCount: int(count),
		Accuracy:    0.0,
		ModelPath:   filepath.Join(h.cfg.Read().DataDir, "models", "latest.pt"),
	}
	if err := db.DB.Create(&run).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.DB.Model(&models.Sample{}).Where("status = ?", "labeled").Update("status", "trained").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "training triggered", "sample_count": count, "run_id": run.ID})
}

func (h *APIHandler) TrainingHistory(c *gin.Context) {
	var runs []models.TrainingRun
	if err := db.DB.Order("created_at DESC").Find(&runs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

func (h *APIHandler) PipelineStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"running": h.pipe.IsRunning()})
}

// --- Image serving ---

func (h *APIHandler) ServeImage(c *gin.Context) {
	name, ok := safeImageFilename(c.Param("filename"))
	if !ok {
		c.Status(http.StatusBadRequest)
		return
	}
	imgDir := filepath.Join(h.cfg.Read().DataDir, "images")
	imgPath := filepath.Join(imgDir, name)
	rel, err := filepath.Rel(imgDir, imgPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		c.Status(http.StatusBadRequest)
		return
	}
	c.File(imgPath)
}
