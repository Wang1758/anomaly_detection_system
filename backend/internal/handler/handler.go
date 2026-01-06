package handler

import (
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/model"
	"anomaly_detection_system/backend/internal/pipeline"
	pb "anomaly_detection_system/backend/pb"
)

// Handler HTTP 请求处理器
type Handler struct {
	config       *config.Config
	grpcClient   *pipeline.GRPCClient
	videoCapture *pipeline.VideoCapture
}

// NewHandler 创建处理器
func NewHandler(cfg *config.Config, grpcClient *pipeline.GRPCClient, videoCapture *pipeline.VideoCapture) *Handler {
	return &Handler{
		config:       cfg,
		grpcClient:   grpcClient,
		videoCapture: videoCapture,
	}
}

// ======================== 视频配置 API ========================

// VideoConfigRequest 视频配置请求
type VideoConfigRequest struct {
	SourceType string `json:"source_type"` // "rtsp" 或 "local"
	RTSPUrl    string `json:"rtsp_url"`    // RTSP 地址
	LocalPath  string `json:"local_path"`  // 本地文件路径
	FPS        int    `json:"fps"`         // 帧率 (30 或 60)
}

// UpdateVideoConfig 更新视频配置
func (h *Handler) UpdateVideoConfig(c *gin.Context) {
	var req VideoConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler] 视频配置请求解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	log.Printf("[Handler] 收到视频配置请求: %+v", req)

	// 验证参数
	if req.SourceType != "rtsp" && req.SourceType != "local" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_type 必须是 'rtsp' 或 'local'"})
		return
	}

	// 放宽 FPS 验证，允许 1-120 之间的值
	if req.FPS < 1 || req.FPS > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fps 必须在 1 到 120 之间"})
		return
	}

	// 更新配置
	h.config.UpdateVideo(config.VideoConfig{
		SourceType: req.SourceType,
		RTSPUrl:    req.RTSPUrl,
		LocalPath:  req.LocalPath,
		FPS:        req.FPS,
	})

	// 重启视频采集
	if h.videoCapture != nil {
		if err := h.videoCapture.Restart(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "视频源切换失败: " + err.Error()})
			return
		}
	}

	log.Printf("[Handler] 视频配置已更新: type=%s, fps=%d", req.SourceType, req.FPS)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "视频配置已更新",
		"config":  h.config.GetVideo(),
	})
}

// GetVideoConfig 获取视频配置
func (h *Handler) GetVideoConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.config.GetVideo())
}

// ======================== AI 配置 API ========================

// AIConfigRequest AI 配置请求
type AIConfigRequest struct {
	ConfidenceThreshold *float32 `json:"confidence_threshold,omitempty"`
	EntropyThreshold    *float32 `json:"entropy_threshold,omitempty"`
	NMSIoUThreshold     *float32 `json:"nms_iou_threshold,omitempty"`
	InputSize           *int     `json:"input_size,omitempty"`
}

// UpdateAIConfig 更新 AI 配置
func (h *Handler) UpdateAIConfig(c *gin.Context) {
	var req AIConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建 gRPC 请求
	grpcReq := &pb.UpdateParamsRequest{}

	aiConfig := h.config.GetAI()

	if req.ConfidenceThreshold != nil {
		if *req.ConfidenceThreshold < 0 || *req.ConfidenceThreshold > 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "confidence_threshold 必须在 0.0 到 1.0 之间"})
			return
		}
		grpcReq.ConfidenceThreshold = req.ConfidenceThreshold
		aiConfig.ConfidenceThreshold = *req.ConfidenceThreshold
	}

	if req.EntropyThreshold != nil {
		if *req.EntropyThreshold < 0 || *req.EntropyThreshold > 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entropy_threshold 必须在 0.0 到 1.0 之间"})
			return
		}
		grpcReq.EntropyThreshold = req.EntropyThreshold
		aiConfig.EntropyThreshold = *req.EntropyThreshold
	}

	if req.NMSIoUThreshold != nil {
		if *req.NMSIoUThreshold < 0.5 || *req.NMSIoUThreshold > 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nms_iou_threshold 必须在 0.5 到 1.0 之间"})
			return
		}
		grpcReq.NmsIouThreshold = req.NMSIoUThreshold
		aiConfig.NMSIoUThreshold = *req.NMSIoUThreshold
	}

	if req.InputSize != nil {
		if *req.InputSize != 320 && *req.InputSize != 640 && *req.InputSize != 1280 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "input_size 必须是 320, 640 或 1280"})
			return
		}
		inputSize := int32(*req.InputSize)
		grpcReq.InputSize = &inputSize
		aiConfig.InputSize = *req.InputSize
	}

	// 更新本地配置
	h.config.UpdateAI(aiConfig)

	// 转发到 Python AI 服务
	aiServiceMessage := ""
	if h.grpcClient != nil {
		resp, err := h.grpcClient.UpdateAIParams(grpcReq)
		if err != nil {
			// AI 服务不可用，仅更新本地配置，不报错
			log.Printf("[Handler] AI 服务不可用，仅更新本地配置: %v", err)
			aiServiceMessage = "（AI 服务未运行，参数将在服务启动后生效）"
		} else if resp != nil && !resp.Success {
			// AI 服务返回失败，记录日志但不报错
			log.Printf("[Handler] AI 服务更新失败: %s", resp.Message)
			aiServiceMessage = "（" + resp.Message + "）"
		}
	}

	log.Printf("[Handler] AI 配置已更新")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "AI 配置已更新" + aiServiceMessage,
		"config":  h.config.GetAI(),
	})
}

// GetAIConfig 获取 AI 配置
func (h *Handler) GetAIConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.config.GetAI())
}

// ======================== 过滤器配置 API ========================

// FilterConfigRequest 过滤器配置请求
type FilterConfigRequest struct {
	SpatialIoUThreshold *float32 `json:"spatial_iou_threshold,omitempty"`
	TimeWindowSeconds   *int     `json:"time_window_seconds,omitempty"`
	EnableAlertPush     *bool    `json:"enable_alert_push,omitempty"`
	AutoSaveSample      *bool    `json:"auto_save_sample,omitempty"`
}

// UpdateFilterConfig 更新过滤器配置
func (h *Handler) UpdateFilterConfig(c *gin.Context) {
	var req FilterConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filterConfig := h.config.GetFilter()

	if req.SpatialIoUThreshold != nil {
		if *req.SpatialIoUThreshold < 0 || *req.SpatialIoUThreshold > 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "spatial_iou_threshold 必须在 0.0 到 1.0 之间"})
			return
		}
		filterConfig.SpatialIoUThreshold = *req.SpatialIoUThreshold
	}

	if req.TimeWindowSeconds != nil {
		if *req.TimeWindowSeconds < 1 || *req.TimeWindowSeconds > 120 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "time_window_seconds 必须在 1 到 120 之间"})
			return
		}
		filterConfig.TimeWindowSeconds = *req.TimeWindowSeconds
	}

	if req.EnableAlertPush != nil {
		filterConfig.EnableAlertPush = *req.EnableAlertPush
	}

	if req.AutoSaveSample != nil {
		filterConfig.AutoSaveSample = *req.AutoSaveSample
	}

	h.config.UpdateFilter(filterConfig)

	log.Printf("[Handler] 过滤器配置已更新")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "过滤器配置已更新",
		"config":  h.config.GetFilter(),
	})
}

// GetFilterConfig 获取过滤器配置
func (h *Handler) GetFilterConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.config.GetFilter())
}

// ======================== 训练配置 API ========================

// TrainingConfigRequest 训练配置请求
type TrainingConfigRequest struct {
	TriggerThreshold *int `json:"trigger_threshold,omitempty"`
}

// UpdateTrainingConfig 更新训练配置
func (h *Handler) UpdateTrainingConfig(c *gin.Context) {
	var req TrainingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trainingConfig := h.config.GetTraining()

	if req.TriggerThreshold != nil {
		if *req.TriggerThreshold < 50 || *req.TriggerThreshold > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "trigger_threshold 必须在 50 到 500 之间"})
			return
		}
		trainingConfig.TriggerThreshold = *req.TriggerThreshold
	}

	h.config.UpdateTraining(trainingConfig)

	log.Printf("[Handler] 训练配置已更新: threshold=%d", trainingConfig.TriggerThreshold)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "训练配置已更新",
		"config":  h.config.GetTraining(),
	})
}

// GetTrainingConfig 获取训练配置
func (h *Handler) GetTrainingConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.config.GetTraining())
}

// ======================== 样本反馈 API ========================

// FeedbackRequest 样本反馈请求
type FeedbackRequest struct {
	SampleID    uint   `json:"sample_id" binding:"required"`
	LabelStatus string `json:"label_status" binding:"required"` // "normal" 或 "abnormal"
	LabeledBy   string `json:"labeled_by"`
}

// SubmitFeedback 提交样本标注反馈
func (h *Handler) SubmitFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.LabelStatus != "normal" && req.LabelStatus != "abnormal" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label_status 必须是 'normal' 或 'abnormal'"})
		return
	}

	// 更新数据库
	err := model.UpdateSampleLabel(req.SampleID, req.LabelStatus, req.LabeledBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新样本失败: " + err.Error()})
		return
	}

	log.Printf("[Handler] 样本标注已更新: id=%d, status=%s", req.SampleID, req.LabelStatus)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "样本标注已更新",
	})
}

// ======================== 训练 API ========================

// GetTrainingStatus 获取训练状态
func (h *Handler) GetTrainingStatus(c *gin.Context) {
	// 获取已标注样本数量
	count, err := model.GetLabeledSamplesCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取最新训练日志
	trainingLog, _ := model.GetLatestTrainingLog()

	trainingConfig := h.config.GetTraining()

	c.JSON(http.StatusOK, gin.H{
		"labeled_samples_count": count,
		"trigger_threshold":     trainingConfig.TriggerThreshold,
		"can_train":             count >= int64(trainingConfig.TriggerThreshold),
		"latest_training":       trainingLog,
	})
}

// TriggerTraining 手动触发训练
func (h *Handler) TriggerTraining(c *gin.Context) {
	trainingConfig := h.config.GetTraining()

	// 创建训练日志
	trainingLog := &model.TrainingLog{
		StartTime: time.Now(),
		Status:    "running",
	}
	err := model.CreateTrainingLog(trainingLog)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建训练日志失败: " + err.Error()})
		return
	}

	// 异步执行训练
	go func() {
		log.Printf("[Handler] 开始执行训练脚本: %s", trainingConfig.TrainingScriptPath)

		cmd := exec.Command("python", trainingConfig.TrainingScriptPath)
		output, err := cmd.CombinedOutput()

		now := time.Now()
		if err != nil {
			log.Printf("[Handler] 训练失败: %v, 输出: %s", err, string(output))
			model.UpdateTrainingLog(trainingLog.ID, map[string]interface{}{
				"status":        "failed",
				"end_time":      now,
				"error_message": err.Error(),
			})
			return
		}

		log.Printf("[Handler] 训练完成，输出: %s", string(output))
		model.UpdateTrainingLog(trainingLog.ID, map[string]interface{}{
			"status":   "completed",
			"end_time": now,
		})

		// 触发模型重载
		if h.grpcClient != nil {
			resp, err := h.grpcClient.ReloadModel("")
			if err != nil {
				log.Printf("[Handler] 模型重载失败: %v", err)
			} else if resp != nil {
				log.Printf("[Handler] 模型重载成功: %s", resp.Message)
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "训练已启动",
		"training_id": trainingLog.ID,
	})
}

// ======================== 系统状态 API ========================

// GetSystemStatus 获取系统状态
func (h *Handler) GetSystemStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"video":    h.config.GetVideo(),
		"ai":       h.config.GetAI(),
		"filter":   h.config.GetFilter(),
		"training": h.config.GetTraining(),
		"time":     time.Now().Format(time.RFC3339),
	})
}

// GetAllConfig 获取所有配置
func (h *Handler) GetAllConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"video":    h.config.GetVideo(),
		"ai":       h.config.GetAI(),
		"filter":   h.config.GetFilter(),
		"training": h.config.GetTraining(),
	})
}

// ======================== 样本列表 API ========================

// GetPendingSamples 获取待处理样本
func (h *Handler) GetPendingSamples(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	samples, err := model.GetPendingSamples(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"samples": samples,
		"count":   len(samples),
	})
}
