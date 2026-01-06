package pipeline

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/config"
)

// Frame 视频帧数据
type Frame struct {
	ID        int64     // 帧序号
	Data      []byte    // JPEG 编码后的图像数据
	Timestamp time.Time // 采集时间
	Width     int       // 图像宽度
	Height    int       // 图像高度
}

// VideoCapture 视频采集器（基于 ffmpeg）
type VideoCapture struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// 配置
	config *config.Config

	// ffmpeg 进程
	cmd    *exec.Cmd
	stdout io.ReadCloser
	isOpen bool

	// 视频信息
	width  int
	height int

	// 帧输出通道
	frameChan chan *Frame

	// 统计
	frameID   int64
	totalRead int64
	errors    int64
}

// NewVideoCapture 创建视频采集器
func NewVideoCapture(cfg *config.Config, frameChan chan *Frame) *VideoCapture {
	ctx, cancel := context.WithCancel(context.Background())
	return &VideoCapture{
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		frameChan: frameChan,
		width:     1280, // 默认宽度
		height:    720,  // 默认高度
	}
}

// Start 启动视频采集
func (vc *VideoCapture) Start() error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.isOpen {
		return nil
	}

	videoConfig := vc.config.GetVideo()

	// 确定视频源
	var source string
	if videoConfig.SourceType == "rtsp" {
		source = videoConfig.RTSPUrl
		log.Printf("[VideoCapture] 正在连接 RTSP 流: %s", source)
	} else {
		source = videoConfig.LocalPath
		log.Printf("[VideoCapture] 正在打开本地视频: %s", source)
	}

	// 构建 ffmpeg 命令
	// 输出原始 RGB 数据到 stdout
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}

	// RTSP 特殊参数
	if videoConfig.SourceType == "rtsp" {
		args = append(args,
			"-rtsp_transport", "tcp",
			"-stimeout", "5000000", // 5秒超时
		)
	}

	// 循环播放本地视频
	if videoConfig.SourceType == "local" {
		args = append(args, "-stream_loop", "-1")
	}

	args = append(args,
		"-i", source,
		"-f", "image2pipe",
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d", videoConfig.FPS, vc.width, vc.height),
		"-vcodec", "mjpeg",
		"-q:v", "5", // JPEG 质量
		"-",
	)

	vc.cmd = exec.CommandContext(vc.ctx, "ffmpeg", args...)

	var err error
	vc.stdout, err = vc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}

	// 启动 ffmpeg
	if err := vc.cmd.Start(); err != nil {
		return fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	vc.isOpen = true
	log.Printf("[VideoCapture] ffmpeg 已启动，帧率: %d FPS, 分辨率: %dx%d",
		videoConfig.FPS, vc.width, vc.height)

	// 启动采集协程
	go vc.captureLoop()

	return nil
}

// Stop 停止视频采集
func (vc *VideoCapture) Stop() {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if !vc.isOpen {
		return
	}

	vc.cancel()

	if vc.cmd != nil && vc.cmd.Process != nil {
		vc.cmd.Process.Kill()
		vc.cmd.Wait()
		vc.cmd = nil
	}

	if vc.stdout != nil {
		vc.stdout.Close()
		vc.stdout = nil
	}

	vc.isOpen = false
	log.Printf("[VideoCapture] 视频采集已停止，共读取 %d 帧，错误 %d 次", vc.totalRead, vc.errors)
}

// Restart 重启视频采集（用于切换视频源）
func (vc *VideoCapture) Restart() error {
	vc.Stop()

	// 重新创建 context
	vc.ctx, vc.cancel = context.WithCancel(context.Background())

	return vc.Start()
}

// captureLoop 视频采集循环
func (vc *VideoCapture) captureLoop() {
	reader := bufio.NewReader(vc.stdout)

	log.Println("[VideoCapture] 采集循环启动")

	for {
		select {
		case <-vc.ctx.Done():
			log.Println("[VideoCapture] 采集循环收到停止信号")
			return
		default:
			// 读取 JPEG 帧
			jpegData, err := vc.readJPEGFrame(reader)
			if err != nil {
				if err == io.EOF {
					log.Println("[VideoCapture] 视频流结束")
					return
				}
				vc.errors++
				// 短暂休眠避免错误循环
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if len(jpegData) == 0 {
				continue
			}

			vc.frameID++
			vc.totalRead++

			frame := &Frame{
				ID:        vc.frameID,
				Data:      jpegData,
				Timestamp: time.Now(),
				Width:     vc.width,
				Height:    vc.height,
			}

			// 非阻塞发送到通道
			select {
			case vc.frameChan <- frame:
			default:
				// 通道满了，丢弃帧
				if vc.frameID%100 == 0 {
					log.Println("[VideoCapture] 帧通道已满，丢弃帧")
				}
			}
		}
	}
}

// readJPEGFrame 从流中读取一个完整的 JPEG 帧
func (vc *VideoCapture) readJPEGFrame(reader *bufio.Reader) ([]byte, error) {
	// JPEG 起始标记: FF D8
	// JPEG 结束标记: FF D9

	var buf bytes.Buffer

	// 查找 JPEG 起始标记
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == 0xFF {
			next, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			if next == 0xD8 {
				// 找到起始标记
				buf.WriteByte(0xFF)
				buf.WriteByte(0xD8)
				break
			}
			// 不是起始标记，放回
			reader.UnreadByte()
		}
	}

	// 读取直到结束标记
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(b)

		if b == 0xFF {
			next, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			buf.WriteByte(next)

			if next == 0xD9 {
				// 找到结束标记
				return buf.Bytes(), nil
			}
		}

		// 防止读取过大的帧
		if buf.Len() > 10*1024*1024 { // 10MB 限制
			return nil, fmt.Errorf("帧数据过大")
		}
	}
}

// GetStats 获取统计信息
func (vc *VideoCapture) GetStats() map[string]interface{} {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	return map[string]interface{}{
		"frame_id":    vc.frameID,
		"total_read":  vc.totalRead,
		"errors":      vc.errors,
		"is_open":     vc.isOpen,
		"source_type": vc.config.GetVideo().SourceType,
		"width":       vc.width,
		"height":      vc.height,
	}
}

// IsOpen 检查视频源是否打开
func (vc *VideoCapture) IsOpen() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.isOpen
}

// SetResolution 设置分辨率
func (vc *VideoCapture) SetResolution(width, height int) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.width = width
	vc.height = height
}

// 以下是用于读取原始视频帧的辅助函数（备用方案）

// readRawFrame 读取原始 RGB 帧数据
func (vc *VideoCapture) readRawFrame(reader io.Reader) ([]byte, error) {
	frameSize := vc.width * vc.height * 3 // RGB24
	data := make([]byte, frameSize)

	n, err := io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}
	if n != frameSize {
		return nil, fmt.Errorf("读取帧数据不完整: %d/%d", n, frameSize)
	}

	return data, nil
}

// rgbToJPEG 将 RGB 数据转换为 JPEG
func (vc *VideoCapture) rgbToJPEG(rgbData []byte) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, vc.width, vc.height))

	// 将 RGB24 转换为 RGBA
	for y := 0; y < vc.height; y++ {
		for x := 0; x < vc.width; x++ {
			srcIdx := (y*vc.width + x) * 3
			dstIdx := (y*vc.width + x) * 4
			img.Pix[dstIdx] = rgbData[srcIdx]     // R
			img.Pix[dstIdx+1] = rgbData[srcIdx+1] // G
			img.Pix[dstIdx+2] = rgbData[srcIdx+2] // B
			img.Pix[dstIdx+3] = 255               // A
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 辅助函数：解析视频信息
func parseVideoInfo(output string) (width, height int, fps float64) {
	// 解析 ffprobe 输出
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "width=") {
			width, _ = strconv.Atoi(strings.TrimPrefix(line, "width="))
		} else if strings.HasPrefix(line, "height=") {
			height, _ = strconv.Atoi(strings.TrimPrefix(line, "height="))
		} else if strings.HasPrefix(line, "r_frame_rate=") {
			rate := strings.TrimPrefix(line, "r_frame_rate=")
			parts := strings.Split(rate, "/")
			if len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 {
					fps = num / den
				}
			}
		}
	}
	return
}

// 未使用的导入占位
var _ = binary.BigEndian
