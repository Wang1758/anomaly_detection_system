package pipeline

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"anomaly_detection_system/backend/internal/config"
	pb "anomaly_detection_system/backend/pb"
)

// DetectionResult 检测结果
type DetectionResult struct {
	FrameID       int64        // 帧序号
	Frame         *Frame       // 原始帧数据
	Detections    []*Detection // 检测结果列表
	InferenceTime int64        // 推理耗时(毫秒)
	Timestamp     time.Time    // 处理时间
}

// Detection 单个检测结果
type Detection struct {
	ID          int32   // 检测框ID
	X1, Y1      float32 // 左上角坐标
	X2, Y2      float32 // 右下角坐标
	ClassName   string  // 类别名称
	ClassID     int32   // 类别ID
	Confidence  float32 // 置信度
	Entropy     float32 // 熵值
	IsUncertain bool    // 是否为不确定目标
}

// GRPCClient gRPC 通信客户端
type GRPCClient struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// 配置
	config *config.Config

	// gRPC 连接
	conn   *grpc.ClientConn
	client pb.DetectionServiceClient

	// 通道
	frameChan  chan *Frame
	resultChan chan *DetectionResult

	// 有序处理
	workerCount int
	pending     sync.Map // frameID -> chan *DetectionResult

	// 统计
	totalSent     int64
	totalReceived int64
	errors        int64
}

// NewGRPCClient 创建 gRPC 客户端
func NewGRPCClient(cfg *config.Config, frameChan chan *Frame, resultChan chan *DetectionResult) *GRPCClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &GRPCClient{
		config:      cfg,
		ctx:         ctx,
		cancel:      cancel,
		frameChan:   frameChan,
		resultChan:  resultChan,
		workerCount: 4, // 并发工作协程数
	}
}

// Start 启动 gRPC 客户端
func (gc *GRPCClient) Start() error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	address := gc.config.Server.GRPCAddress
	log.Printf("[GRPCClient] 正在连接 AI 服务: %s", address)

	// 建立连接
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
		),
	)
	if err != nil {
		return err
	}

	gc.conn = conn
	gc.client = pb.NewDetectionServiceClient(conn)

	log.Printf("[GRPCClient] 已连接到 AI 服务")

	// 启动有序处理协程池
	go gc.orderingLoop()
	for i := 0; i < gc.workerCount; i++ {
		go gc.workerLoop(i)
	}

	return nil
}

// Stop 停止 gRPC 客户端
func (gc *GRPCClient) Stop() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.cancel()

	if gc.conn != nil {
		gc.conn.Close()
		gc.conn = nil
	}

	log.Printf("[GRPCClient] 已断开连接，发送 %d，接收 %d，错误 %d",
		gc.totalSent, gc.totalReceived, gc.errors)
}

// workerLoop 工作协程
func (gc *GRPCClient) workerLoop(id int) {
	log.Printf("[GRPCClient] Worker %d 启动", id)

	for {
		select {
		case <-gc.ctx.Done():
			log.Printf("[GRPCClient] Worker %d 停止", id)
			return
		case frame := <-gc.frameChan:
			if frame == nil {
				continue
			}

			// 执行检测
			result := gc.detect(frame)
			if result != nil {
				// 发送到结果通道
				select {
				case gc.resultChan <- result:
					gc.totalReceived++
				default:
					log.Println("[GRPCClient] 结果通道已满，丢弃结果")
				}
			}
		}
	}
}

// orderingLoop 有序输出协程（确保结果按帧序号排序）
func (gc *GRPCClient) orderingLoop() {
	// 简化实现：由于 workerLoop 直接输出到 resultChan，
	// 这里可以添加更复杂的排序逻辑
	// 当前实现中，我们依赖下游处理器处理乱序
}

// detect 执行单帧检测
func (gc *GRPCClient) detect(frame *Frame) *DetectionResult {
	gc.mu.RLock()
	client := gc.client
	gc.mu.RUnlock()

	if client == nil {
		return nil
	}

	// 构建请求
	req := &pb.DetectRequest{
		ImageData:   frame.Data,
		FrameId:     frame.ID,
		ImageFormat: "jpeg",
	}

	// 调用 AI 服务（带超时和重试）
	var resp *pb.DetectResponse
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(gc.ctx, 5*time.Second)
		resp, err = client.Detect(ctx, req)
		cancel()

		if err == nil && resp.Error == "" {
			break
		}

		if i < maxRetries-1 {
			log.Printf("[GRPCClient] 检测请求失败 (重试 %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(100 * time.Millisecond)
		}
	}

	gc.totalSent++

	if err != nil {
		gc.errors++
		log.Printf("[GRPCClient] 检测请求最终失败: %v", err)
		return nil
	}

	if resp.Error != "" {
		gc.errors++
		log.Printf("[GRPCClient] AI 服务返回错误: %s", resp.Error)
		return nil
	}

	// 解析结果
	result := &DetectionResult{
		FrameID:       resp.FrameId,
		Frame:         frame,
		InferenceTime: resp.InferenceTimeMs,
		Timestamp:     time.Now(),
		Detections:    make([]*Detection, 0, len(resp.Results)),
	}

	for _, r := range resp.Results {
		detection := &Detection{
			ID:          r.Id,
			X1:          r.Bbox.X1,
			Y1:          r.Bbox.Y1,
			X2:          r.Bbox.X2,
			Y2:          r.Bbox.Y2,
			ClassName:   r.ClassName,
			ClassID:     r.ClassId,
			Confidence:  r.Confidence,
			Entropy:     r.Entropy,
			IsUncertain: r.IsUncertain,
		}
		result.Detections = append(result.Detections, detection)
	}

	return result
}

// UpdateAIParams 更新 AI 服务参数
func (gc *GRPCClient) UpdateAIParams(params *pb.UpdateParamsRequest) (*pb.UpdateParamsResponse, error) {
	gc.mu.RLock()
	client := gc.client
	gc.mu.RUnlock()

	if client == nil {
		// AI 服务未连接，跳过更新
		log.Println("[GRPCClient] AI 服务未连接，跳过参数更新")
		return &pb.UpdateParamsResponse{
			Success: true,
			Message: "AI 服务未连接，仅更新本地配置",
		}, nil
	}

	// 使用独立的 context，避免父 context 取消导致调用失败
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.UpdateParams(ctx, params)
}

// ReloadModel 重载模型
func (gc *GRPCClient) ReloadModel(modelPath string) (*pb.ReloadModelResponse, error) {
	gc.mu.RLock()
	client := gc.client
	gc.mu.RUnlock()

	if client == nil {
		// AI 服务未连接，跳过重载
		log.Println("[GRPCClient] AI 服务未连接，跳过模型重载")
		return &pb.ReloadModelResponse{
			Success: false,
			Message: "AI 服务未连接",
		}, nil
	}

	// 使用独立的 context，避免父 context 取消导致调用失败
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // 模型加载可能较慢
	defer cancel()

	return client.ReloadModel(ctx, &pb.ReloadModelRequest{
		ModelPath: modelPath,
	})
}

// GetStats 获取统计信息
func (gc *GRPCClient) GetStats() map[string]interface{} {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	return map[string]interface{}{
		"total_sent":     gc.totalSent,
		"total_received": gc.totalReceived,
		"errors":         gc.errors,
		"connected":      gc.conn != nil,
	}
}
