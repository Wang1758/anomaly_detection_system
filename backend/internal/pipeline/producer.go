package pipeline

import (
	"context"
	"fmt"
	"log"
	"time"

	"gocv.io/x/gocv"
)

// Producer reads frames from RTSP or local video via GoCV.
type Producer struct {
	sourceType string
	sourceAddr string
	fps        int
}

func NewProducer(sourceType, sourceAddr string, fps int) *Producer {
	return &Producer{
		sourceType: sourceType,
		sourceAddr: sourceAddr,
		fps:        fps,
	}
}

func (p *Producer) Run(ctx context.Context, pool *OrderedPool) error {
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

	var seqNo int64
	ticker := time.NewTicker(time.Second / time.Duration(p.fps))
	defer ticker.Stop()

	log.Printf("Producer started (gocv): type=%s addr=%s fps=%d", p.sourceType, p.sourceAddr, p.fps)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ok := cap.Read(&mat); !ok || mat.Empty() {
				log.Println("Producer: end of stream")
				return nil
			}
			buf, err := gocv.IMEncode(gocv.JPEGFileExt, mat)
			if err != nil {
				log.Printf("Producer: encode error: %v", err)
				continue
			}
			encoded := append([]byte(nil), buf.GetBytes()...)
			task := &Task{SeqNo: seqNo, ImageBytes: encoded}
			buf.Close()
			if !pool.Submit(ctx, task) {
				return nil
			}
			seqNo++
		}
	}
}
