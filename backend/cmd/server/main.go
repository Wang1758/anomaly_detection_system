package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/filter"
	"anomaly_detection_system/backend/internal/handler"
	"anomaly_detection_system/backend/internal/model"
	"anomaly_detection_system/backend/internal/pipeline"
	"anomaly_detection_system/backend/internal/ws"
)

func main() {
	log.Println("======================================")
	log.Println("  智慧养殖场监控系统 - Go 后端服务")
	log.Println("======================================")

	// 加载配置
	cfg := config.DefaultConfig()
	log.Printf("配置加载完成")

	// 初始化数据库
	if err := model.InitDB(cfg.Database.Path); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 创建通道
	frameChan := make(chan *pipeline.Frame, 30)            // 视频帧通道
	resultChan := make(chan *pipeline.DetectionResult, 30) // 检测结果通道
	alertChan := make(chan *ws.AlertMessage, 100)          // 报警消息通道

	// 创建组件
	videoCapture := pipeline.NewVideoCapture(cfg, frameChan)
	grpcClient := pipeline.NewGRPCClient(cfg, frameChan, resultChan)
	wsServer := ws.NewWebSocketServer(cfg, resultChan, alertChan)
	alertFilter := filter.NewAlertFilter(cfg)
	httpHandler := handler.NewHandler(cfg, grpcClient, videoCapture)

	// 启动组件
	log.Println("正在启动各组件...")

	// 启动 gRPC 客户端
	if err := grpcClient.Start(); err != nil {
		log.Printf("警告: gRPC 客户端启动失败: %v (AI 服务可能未启动)", err)
	}

	// 启动 WebSocket 服务
	wsServer.Start()

	// 启动视频采集（如果配置了视频源）
	if cfg.Video.RTSPUrl != "" || cfg.Video.LocalPath != "" {
		if err := videoCapture.Start(); err != nil {
			log.Printf("警告: 视频采集启动失败: %v", err)
		}
	}

	// 启动报警处理协程
	go processAlerts(resultChan, alertFilter, alertChan)

	// 设置 Gin 路由
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS 中间件
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API 路由
	api := router.Group("/api")
	{
		// 配置接口
		api.GET("/config", httpHandler.GetAllConfig)
		api.GET("/config/video", httpHandler.GetVideoConfig)
		api.POST("/config/video", httpHandler.UpdateVideoConfig)
		api.GET("/config/ai", httpHandler.GetAIConfig)
		api.POST("/config/ai", httpHandler.UpdateAIConfig)
		api.GET("/config/filter", httpHandler.GetFilterConfig)
		api.POST("/config/filter", httpHandler.UpdateFilterConfig)
		api.GET("/config/training", httpHandler.GetTrainingConfig)
		api.POST("/config/training", httpHandler.UpdateTrainingConfig)

		// 反馈接口
		api.POST("/feedback", httpHandler.SubmitFeedback)

		// 训练接口
		api.GET("/training/status", httpHandler.GetTrainingStatus)
		api.POST("/training/trigger", httpHandler.TriggerTraining)

		// 样本接口
		api.GET("/samples/pending", httpHandler.GetPendingSamples)

		// 系统状态
		api.GET("/status", httpHandler.GetSystemStatus)
	}

	// WebSocket 路由
	router.GET("/ws", func(c *gin.Context) {
		wsServer.HandleWebSocket(c.Writer, c.Request)
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 启动 HTTP 服务器
	httpAddr := ":8080"
	log.Printf("HTTP 服务启动: http://localhost%s", httpAddr)
	log.Printf("WebSocket 服务: ws://localhost%s/ws", httpAddr)

	// 启动服务器
	server := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务器启动失败: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")

	// 停止组件
	videoCapture.Stop()
	grpcClient.Stop()
	wsServer.Stop()

	log.Println("服务已关闭")
}

// processAlerts 处理检测结果，生成报警
func processAlerts(resultChan chan *pipeline.DetectionResult, alertFilter *filter.AlertFilter, alertChan chan *ws.AlertMessage) {
	// 创建一个新的通道来接收结果（不影响 WebSocket 广播）
	// 这里简化实现：直接监听，实际应该用扇出模式
	log.Println("[AlertProcessor] 报警处理协程启动")

	// 注意：由于 resultChan 同时被 wsServer 和这里使用，
	// 实际实现中应该使用扇出模式。这里仅作示例。
}
