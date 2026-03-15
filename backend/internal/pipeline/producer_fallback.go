//go:build !gocv

package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Producer reads frames from image files or generates synthetic frames
// when GoCV is not available.
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
	frames, err := p.loadFrames()
	if err != nil {
		return err
	}

	var seqNo int64
	ticker := time.NewTicker(time.Second / time.Duration(p.fps))
	defer ticker.Stop()

	log.Printf("Producer started (fallback): type=%s addr=%s fps=%d frames=%d",
		p.sourceType, p.sourceAddr, p.fps, len(frames))

	idx := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if len(frames) == 0 {
				frames = [][]byte{generateSyntheticFrame(640, 480)}
			}
			frame := frames[idx%len(frames)]
			task := &Task{SeqNo: seqNo, ImageBytes: frame}
			if !pool.Submit(ctx, task) {
				return nil
			}
			seqNo++
			idx++
		}
	}
}

func (p *Producer) loadFrames() ([][]byte, error) {
	if p.sourceAddr == "" {
		log.Println("No source address, using synthetic frames")
		return [][]byte{generateSyntheticFrame(640, 480)}, nil
	}

	info, err := os.Stat(p.sourceAddr)
	if err != nil {
		return nil, fmt.Errorf("source not found: %w", err)
	}

	if !info.IsDir() {
		data, err := os.ReadFile(p.sourceAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		return [][]byte{data}, nil
	}

	// Read all JPEG files from directory
	entries, err := os.ReadDir(p.sourceAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			files = append(files, filepath.Join(p.sourceAddr, e.Name()))
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		log.Println("No image files found, using synthetic frames")
		return [][]byte{generateSyntheticFrame(640, 480)}, nil
	}

	var frames [][]byte
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("Skipping %s: %v", f, err)
			continue
		}
		frames = append(frames, data)
	}
	return frames, nil
}

func generateSyntheticFrame(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{R: 30, G: 30, B: 40, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	// Draw some random rectangles to simulate objects
	for i := 0; i < 3; i++ {
		rx := rand.Intn(w - 60)
		ry := rand.Intn(h - 60)
		rw := 30 + rand.Intn(40)
		rh := 30 + rand.Intn(40)
		c := color.RGBA{R: uint8(rand.Intn(100)), G: uint8(rand.Intn(100)), B: uint8(rand.Intn(100)), A: 255}
		for y := ry; y < ry+rh && y < h; y++ {
			for x := rx; x < rx+rw && x < w; x++ {
				img.Set(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}
