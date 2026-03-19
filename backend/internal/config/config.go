package config

import "sync"

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
}

var (
	global *Config
	once   sync.Once
)

func Get() *Config {
	once.Do(func() {
		global = Default()
	})
	return global
}

func Default() *Config {
	return &Config{
		AIServiceAddr:       "localhost:50051",
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
		DataDir:             "./data",
	}
}

func (c *Config) Read() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c
}

func (c *Config) Update(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}
