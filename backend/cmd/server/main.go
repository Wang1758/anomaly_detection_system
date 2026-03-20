package main

import (
	"log"
	"net/http"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/grpcclient"
	"anomaly_detection_system/backend/internal/handler"
	"anomaly_detection_system/backend/internal/pipeline"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Get()

	// Init database
	db.Init(cfg.DataDir)

	// Init gRPC client
	grpcClient, err := grpcclient.New(cfg.AIServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to AI service: %v", err)
	}
	defer grpcClient.Close()

	// Init pipeline
	pipe := pipeline.New(cfg, grpcClient)

	// Init LLM judger (uses env vars: LLM_API_KEY, LLM_BASE_URL, LLM_MODEL)
	llmJudger := handler.NewLLMJudger(cfg.LLMApiKey, cfg.LLMBaseURL, cfg.LLMModel)
	if llmJudger.Available() {
		log.Printf("LLM AI Judge enabled (model=%s)", cfg.LLMModel)
	} else {
		log.Println("LLM API key not set — AI Judge will fall back to YOLO re-detection")
	}

	// Init handlers
	apiHandler := handler.NewAPIHandler(cfg, pipe, grpcClient, llmJudger)
	streamHandler := handler.NewStreamHandler(pipe)
	wsHub := handler.NewWSHub(pipe)

	// Setup Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// REST API
	api := r.Group("/api")
	{
		api.GET("/config", apiHandler.GetConfig)
		api.PUT("/config", apiHandler.UpdateConfig)

		api.GET("/samples", apiHandler.ListSamples)
		api.POST("/samples/:id/label", apiHandler.LabelSample)
		api.PATCH("/samples/:id/relabel", apiHandler.RelabelSample)
		api.POST("/samples/ai-judge", apiHandler.AIJudge)

		api.POST("/training/trigger", apiHandler.TriggerTraining)
		api.GET("/training/history", apiHandler.TrainingHistory)

		api.POST("/pipeline/start", apiHandler.StartPipeline)
		api.POST("/pipeline/stop", apiHandler.StopPipeline)
		api.GET("/pipeline/status", apiHandler.PipelineStatus)

		api.GET("/images/:filename", apiHandler.ServeImage)
	}

	// Root hint to avoid 404 when opening backend port in browser
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":         "anomaly-detection-backend",
			"message":         "Backend is running. Open frontend at http://localhost:3000",
			"frontend_url":    "http://localhost:3000",
			"api_base":        "/api",
			"stream_endpoint": "/api/stream/mjpeg",
			"ws_endpoint":     "/ws/events",
		})
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// MJPEG stream
	r.GET("/api/stream/mjpeg", streamHandler.ServeMJPEG)

	// WebSocket
	r.GET("/ws/events", gin.WrapF(wsHub.HandleWS))

	log.Printf("Server starting on %s", cfg.ServerPort)
	if err := r.Run(cfg.ServerPort); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
