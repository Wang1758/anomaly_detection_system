package main

import (
	"log"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/grpcclient"
	"anomaly_detection_system/backend/internal/handler"
	"anomaly_detection_system/backend/internal/pipeline"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Get()

	// Local overrides
	aiServiceAddr := "localhost:50051" // ai 服务grpc地址
	serverPort := ":8080"              // 当前go服务监听地址
	dataDir := "../data"               // 数据库文件存放位置

	cfg.AIServiceAddr = aiServiceAddr
	cfg.ServerPort = serverPort
	cfg.DataDir = dataDir

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

	// Init handlers
	apiHandler := handler.NewAPIHandler(cfg, pipe, grpcClient)
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

		api.GET("/images/:filename", apiHandler.ServeImage)
	}

	// MJPEG stream
	r.GET("/api/stream/mjpeg", streamHandler.ServeMJPEG)

	// WebSocket
	r.GET("/ws/events", gin.WrapF(wsHub.HandleWS))

	log.Printf("Server starting on %s", cfg.ServerPort)
	if err := r.Run(cfg.ServerPort); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
