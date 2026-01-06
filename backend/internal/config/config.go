package config

import (
	"sync"
)

// Config 全局配置
type Config struct {
	mu sync.RWMutex

	// 服务器配置
	Server ServerConfig `yaml:"server"`

	// 视频源配置
	Video VideoConfig `yaml:"video"`

	// AI 服务配置
	AI AIConfig `yaml:"ai"`

	// 过滤器配置
	Filter FilterConfig `yaml:"filter"`

	// 训练配置
	Training TrainingConfig `yaml:"training"`

	// 数据库配置
	Database DatabaseConfig `yaml:"database"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	HTTPPort      int    `yaml:"http_port"`      // HTTP 端口
	WebSocketPort int    `yaml:"websocket_port"` // WebSocket 端口
	GRPCAddress   string `yaml:"grpc_address"`   // Python AI 服务地址
}

// VideoConfig 视频源配置
type VideoConfig struct {
	SourceType string `yaml:"source_type" json:"source_type"` // "rtsp" 或 "local"
	RTSPUrl    string `yaml:"rtsp_url" json:"rtsp_url"`       // RTSP 地址
	LocalPath  string `yaml:"local_path" json:"local_path"`   // 本地文件路径
	FPS        int    `yaml:"fps" json:"fps"`                 // 采集帧率 (30 或 60)
}

// AIConfig AI 服务参数配置
type AIConfig struct {
	ConfidenceThreshold float32 `yaml:"confidence_threshold" json:"confidence_threshold"` // 置信度阈值
	EntropyThreshold    float32 `yaml:"entropy_threshold" json:"entropy_threshold"`       // 熵值阈值
	NMSIoUThreshold     float32 `yaml:"nms_iou_threshold" json:"nms_iou_threshold"`       // NMS IoU 阈值
	InputSize           int     `yaml:"input_size" json:"input_size"`                     // 输入图像尺寸
}

// FilterConfig 过滤器配置
type FilterConfig struct {
	SpatialIoUThreshold float32 `yaml:"spatial_iou_threshold" json:"spatial_iou_threshold"` // 空间抑制 IoU 阈值
	TimeWindowSeconds   int     `yaml:"time_window_seconds" json:"time_window_seconds"`     // 时间窗口（秒）
	EnableAlertPush     bool    `yaml:"enable_alert_push" json:"enable_alert_push"`         // 启用报警推送
	AutoSaveSample      bool    `yaml:"auto_save_sample" json:"auto_save_sample"`           // 自动保存样本
}

// TrainingConfig 训练配置
type TrainingConfig struct {
	TriggerThreshold   int    `yaml:"trigger_threshold" json:"trigger_threshold"`       // 触发训练的样本数阈值
	TrainingScriptPath string `yaml:"training_script_path" json:"training_script_path"` // 训练脚本路径
	ModelOutputPath    string `yaml:"model_output_path" json:"model_output_path"`       // 模型输出路径
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Path string `yaml:"path"` // SQLite 数据库文件路径
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort:      8080,
			WebSocketPort: 8081,
			GRPCAddress:   "localhost:50051",
		},
		Video: VideoConfig{
			SourceType: "rtsp",
			RTSPUrl:    "",
			LocalPath:  "",
			FPS:        30,
		},
		AI: AIConfig{
			ConfidenceThreshold: 0.5,
			EntropyThreshold:    0.5,
			NMSIoUThreshold:     0.8,
			InputSize:           640,
		},
		Filter: FilterConfig{
			SpatialIoUThreshold: 0.5,
			TimeWindowSeconds:   60,
			EnableAlertPush:     true,
			AutoSaveSample:      true,
		},
		Training: TrainingConfig{
			TriggerThreshold:   100,
			TrainingScriptPath: "../ai_service/train.py",
			ModelOutputPath:    "../data/models/",
		},
		Database: DatabaseConfig{
			Path: "/app/data/detection.db",
		},
	}
}

// UpdateVideo 更新视频配置
func (c *Config) UpdateVideo(cfg VideoConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Video = cfg
}

// UpdateAI 更新 AI 配置
func (c *Config) UpdateAI(cfg AIConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AI = cfg
}

// UpdateFilter 更新过滤器配置
func (c *Config) UpdateFilter(cfg FilterConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Filter = cfg
}

// UpdateTraining 更新训练配置
func (c *Config) UpdateTraining(cfg TrainingConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Training = cfg
}

// GetVideo 获取视频配置
func (c *Config) GetVideo() VideoConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Video
}

// GetAI 获取 AI 配置
func (c *Config) GetAI() AIConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AI
}

// GetFilter 获取过滤器配置
func (c *Config) GetFilter() FilterConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Filter
}

// GetTraining 获取训练配置
func (c *Config) GetTraining() TrainingConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Training
}
