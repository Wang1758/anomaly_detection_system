package pipeline

import (
	"context"
	"log"
	"sync"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/filter"
	"anomaly_detection_system/backend/internal/grpcclient"
)

// Pipeline orchestrates Producer -> BatchProcessor -> Broadcaster.
//
// The batch pipeline mirrors 乌骨鸡 project's architecture where frames are
// collected and processed in batches on the GPU, yielding 3-5x throughput
// improvement over sequential single-frame gRPC calls.
type Pipeline struct {
	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	broadcaster *Broadcaster
	grpcClient  *grpcclient.Client
	cfg         *config.Config
}

func New(cfg *config.Config, grpcClient *grpcclient.Client) *Pipeline {
	return &Pipeline{
		cfg:        cfg,
		grpcClient: grpcClient,
	}
}

func (p *Pipeline) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Pipeline) GetBroadcaster() *Broadcaster {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.broadcaster
}

func (p *Pipeline) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	snap := p.cfg.Read()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	f := filter.NewSpatiotemporalFilter(snap.FilterTimeWindow, snap.FilterIoU)
	broadcaster := NewBroadcaster(f, snap.DataDir)
	p.broadcaster = broadcaster

	producer := NewProducer(snap.SourceType, snap.SourceAddr, snap.FPS)

	frameCh := make(chan *Task, snap.BatchSize*2)
	resultCh := make(chan *OrderedResult, snap.BatchSize*2)
	batchProc := NewBatchProcessor(p.grpcClient, snap.BatchSize, snap.BatchTimeout)

	// Producer goroutine: reads frames and sends to frameCh
	go func() {
		if err := producer.RunCh(ctx, frameCh); err != nil {
			log.Printf("Producer error: %v", err)
		}
		close(frameCh)
	}()

	// BatchProcessor goroutine: collects frames into batches and calls gRPC BatchDetect
	go func() {
		batchProc.Run(ctx, frameCh, resultCh)
		close(resultCh)
	}()

	// Consumer goroutine: feeds ordered results to the Broadcaster
	go func() {
		for r := range resultCh {
			broadcaster.HandleResult(r)
		}
		log.Println("Pipeline consumer stopped")
	}()

	p.running = true
	log.Printf("Pipeline started (batch_size=%d, batch_timeout=%dms)",
		snap.BatchSize, snap.BatchTimeout)
	return nil
}

func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	p.cancel()
	p.running = false
	log.Println("Pipeline stopped")
}
