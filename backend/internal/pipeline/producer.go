package pipeline

import (
	"context"
	"fmt"
	"image"
	"log"
	"time"

	"anomaly_detection_system/backend/internal/perf"

	"gocv.io/x/gocv"
)

const (
	modelInputWidth    = 640
	modelInputHeight   = 640
	modelInputChannels = 3
	origJPEGQuality    = 85
)

func ValidateSource(sourceType, sourceAddr string) error {
	if sourceAddr == "" {
		return fmt.Errorf("source address is empty")
	}

	var cap *gocv.VideoCapture
	var err error

	switch sourceType {
	case "rtsp", "local":
		cap, err = gocv.OpenVideoCapture(sourceAddr)
	default:
		return fmt.Errorf("unknown source type: %s", sourceType)
	}
	if err != nil {
		return fmt.Errorf("failed to open video source: %w", err)
	}
	defer cap.Close()

	mat := gocv.NewMat()
	defer mat.Close()
	if ok := cap.Read(&mat); !ok || mat.Empty() {
		return fmt.Errorf("video source is not readable: %s", sourceAddr)
	}

	return nil
}

// Producer reads frames from RTSP or local video via GoCV.
type Producer struct {
	sourceType string
	sourceAddr string
	fps        int
	frameSink  func([]byte)
}

func NewProducer(sourceType, sourceAddr string, fps int) *Producer {
	return &Producer{
		sourceType: sourceType,
		sourceAddr: sourceAddr,
		fps:        fps,
	}
}

func (p *Producer) SetFrameSink(frameSink func([]byte)) {
	p.frameSink = frameSink
}

func (p *Producer) Run(ctx context.Context, pool *OrderedPool) error {
	return p.run(ctx, func(task *Task) bool {
		return pool.Submit(ctx, task)
	})
}

// RunCh sends frames to a plain channel (used by BatchProcessor pipeline).
func (p *Producer) RunCh(ctx context.Context, ch chan<- *Task) error {
	var dropped int64
	var acceptedSeq int64
	lastDropLog := time.Now()
	windowStart := time.Now()
	windowEnqueued := int64(0)
	windowDropped := int64(0)
	return p.run(ctx, func(task *Task) bool {
		if ctx.Err() != nil {
			return false
		}

		now := time.Now()
		if perf.Enabled() {
			if elapsed := now.Sub(windowStart); elapsed >= time.Second {
				inflight := len(ch)
				capCh := cap(ch)
				acceptRate := float64(windowEnqueued) / elapsed.Seconds()
				perf.Logf("Producer enqueue perf: accepted=%d dropped=%d accept_rate=%.1ffps queue=%d/%d accepted_seq=%d",
					windowEnqueued, windowDropped, acceptRate, inflight, capCh, acceptedSeq)
				windowStart = now
				windowEnqueued = 0
				windowDropped = 0
			}
		}

		enqueuedTask := &Task{
			SeqNo:      acceptedSeq,
			ImageBytes: task.ImageBytes,
			OrigJPEG:   task.OrigJPEG,
			OrigWidth:  task.OrigWidth,
			OrigHeight: task.OrigHeight,
		}
		select {
		case ch <- enqueuedTask:
			acceptedSeq++
			windowEnqueued++
			return true
		default:
			dropped++
			windowDropped++
			if time.Since(lastDropLog) >= time.Second {
				log.Printf("Producer drop frames under pressure: dropped=%d accepted_seq=%d", dropped, acceptedSeq)
				lastDropLog = time.Now()
			}
			return true
		case <-ctx.Done():
			return false
		}
	})
}

func (p *Producer) run(ctx context.Context, emit func(*Task) bool) error {
	var cap *gocv.VideoCapture
	var err error

	switch p.sourceType {
	case "rtsp", "local":
		cap, err = gocv.OpenVideoCapture(p.sourceAddr)
	default:
		return fmt.Errorf("unknown source type: %s", p.sourceType)
	}
	if err != nil {
		return fmt.Errorf("failed to open video source: %w", err)
	}
	defer cap.Close()

	mat := gocv.NewMat()
	defer mat.Close()
	resized := gocv.NewMat()
	defer resized.Close()
	rgb := gocv.NewMat()
	defer rgb.Close()

	var seqNo int64
	ticker := time.NewTicker(time.Second / time.Duration(p.fps))
	defer ticker.Stop()
	windowStart := time.Now()
	windowRead := int64(0)
	windowPrepared := int64(0)
	windowReadCost := time.Duration(0)
	windowPrepCost := time.Duration(0)

	log.Printf("Producer started (gocv): type=%s addr=%s fps=%d", p.sourceType, p.sourceAddr, p.fps)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			readStart := time.Now()
			if ok := cap.Read(&mat); !ok || mat.Empty() {
				log.Println("Producer: end of stream")
				return nil
			}
			windowRead++
			windowReadCost += time.Since(readStart)

			prepStart := time.Now()

			origWidth := mat.Cols()
			origHeight := mat.Rows()

			// Encode the original high-res frame as JPEG for streaming
			origBuf, encErr := gocv.IMEncodeWithParams(gocv.JPEGFileExt, mat, []int{gocv.IMWriteJpegQuality, origJPEGQuality})
			if encErr != nil {
				log.Printf("Producer: JPEG encode error: %v", encErr)
				continue
			}
			origJPEG := append([]byte(nil), origBuf.GetBytes()...)
			origBuf.Close()
			if p.frameSink != nil {
				p.frameSink(origJPEG)
			}

			// Resize a copy to 640x640 for AI inference
			gocv.Resize(mat, &resized, image.Pt(modelInputWidth, modelInputHeight), 0, 0, gocv.InterpolationLinear)
			gocv.CvtColor(resized, &rgb, gocv.ColorBGRToRGB)
			raw := rgb.ToBytes()
			if len(raw) != modelInputWidth*modelInputHeight*modelInputChannels {
				log.Printf("Producer: unexpected raw frame bytes=%d expect=%d", len(raw), modelInputWidth*modelInputHeight*modelInputChannels)
				continue
			}
			windowPrepared++
			windowPrepCost += time.Since(prepStart)

			task := &Task{
				SeqNo:      seqNo,
				ImageBytes: append([]byte(nil), raw...),
				OrigJPEG:   origJPEG,
				OrigWidth:  origWidth,
				OrigHeight: origHeight,
			}
			if !emit(task) {
				return nil
			}
			seqNo++

			now := time.Now()
			if perf.Enabled() {
				if elapsed := now.Sub(windowStart); elapsed >= time.Second {
					avgReadMs := 0.0
					avgPrepMs := 0.0
					if windowRead > 0 {
						avgReadMs = float64(windowReadCost.Milliseconds()) / float64(windowRead)
					}
					if windowPrepared > 0 {
						avgPrepMs = float64(windowPrepCost.Milliseconds()) / float64(windowPrepared)
					}
					produceRate := float64(windowPrepared) / elapsed.Seconds()
					perf.Logf("Producer capture perf: read=%d prepared=%d rate=%.1ffps avg_read=%.2fms avg_preprocess=%.2fms seq=%d origRes=%dx%d",
						windowRead, windowPrepared, produceRate, avgReadMs, avgPrepMs, seqNo, origWidth, origHeight)
					windowStart = now
					windowRead = 0
					windowPrepared = 0
					windowReadCost = 0
					windowPrepCost = 0
				}
			}
		}
	}
}
