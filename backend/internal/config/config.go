package config

import (
	"os"
	"strings"
	"sync"
)

type Config struct {
	mu sync.RWMutex

	// AI service
	AIServiceAddr       string  `json:"ai_service_addr"`
	NMSThreshold        float32 `json:"nms_threshold"`
	ConfidenceThreshold float32 `json:"confidence_threshold"`
	EntropyThreshold    float32 `json:"entropy_threshold"`
	W1                  float32 `json:"w1"`
	W2                  float32 `json:"w2"`

	// Pipeline
	SourceType   string `json:"source_type"` // "rtsp" or "local"
	SourceAddr   string `json:"source_addr"`
	FPS          int    `json:"fps"`
	Workers      int    `json:"workers"`
	BatchSize    int    `json:"batch_size"`
	BatchTimeout int    `json:"batch_timeout_ms"` // milliseconds

	// Spatiotemporal filter
	FilterTimeWindow float64 `json:"filter_time_window"` // seconds
	FilterIoU        float64 `json:"filter_iou"`

	// Server
	ServerPort string `json:"server_port"`
	DataDir    string `json:"data_dir"`

	// LLM (multimodal) for AI Judge — loaded from env only, never exposed via API
	LLMApiKey  string `json:"-"`
	LLMBaseURL string `json:"-"`
	LLMModel   string `json:"-"`
}

var (
	global *Config
	once   sync.Once
)

func Get() *Config {
	once.Do(func() {
		global = Default()
		applyEnvOverrides(global)
	})
	return global
}

// applyEnvOverrides loads AI_SERVICE_ADDR, SERVER_PORT, DATA_DIR once at startup.
func applyEnvOverrides(c *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v := strings.TrimSpace(os.Getenv("AI_SERVICE_ADDR")); v != "" {
		c.AIServiceAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("SERVER_PORT")); v != "" {
		c.ServerPort = normalizeListenAddr(v)
	}
	if v := strings.TrimSpace(os.Getenv("DATA_DIR")); v != "" {
		c.DataDir = v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_API_KEY")); v != "" {
		c.LLMApiKey = v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_BASE_URL")); v != "" {
		c.LLMBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_MODEL")); v != "" {
		c.LLMModel = v
	}
}

func normalizeListenAddr(port string) string {
	p := strings.TrimSpace(port)
	if p == "" {
		return ":8080"
	}
	if strings.HasPrefix(p, ":") {
		return p
	}
	return ":" + p
}

func Default() *Config {
	return &Config{
		AIServiceAddr:       "192.168.3.23:50051", // python服务地址
		NMSThreshold:        0.8,
		ConfidenceThreshold: 0.25,
		EntropyThreshold:    0.5,
		W1:                  0.6,
		W2:                  0.4,
		SourceType:          "local",
		SourceAddr:          "",
		FPS:                 30,
		Workers:             4,
		BatchSize:           8,
		BatchTimeout:        200,
		FilterTimeWindow:    60.0,
		FilterIoU:           0.5,
		ServerPort:          ":8080",
		DataDir:             "../data",
		LLMBaseURL:          "https://api.openai.com/v1",
		LLMModel:            "gpt-4o",
	}
}

func (c *Config) Read() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &Config{
		AIServiceAddr:       c.AIServiceAddr,
		NMSThreshold:        c.NMSThreshold,
		ConfidenceThreshold: c.ConfidenceThreshold,
		EntropyThreshold:    c.EntropyThreshold,
		W1:                  c.W1,
		W2:                  c.W2,
		SourceType:          c.SourceType,
		SourceAddr:          c.SourceAddr,
		FPS:                 c.FPS,
		Workers:             c.Workers,
		BatchSize:           c.BatchSize,
		BatchTimeout:        c.BatchTimeout,
		FilterTimeWindow:    c.FilterTimeWindow,
		FilterIoU:           c.FilterIoU,
		ServerPort:          c.ServerPort,
		DataDir:             c.DataDir,
		LLMApiKey:           c.LLMApiKey,
		LLMBaseURL:          c.LLMBaseURL,
		LLMModel:            c.LLMModel,
	}
}

func (c *Config) Update(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}
