package pipeline

import (
	"context"
	"log"
	"sync"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/filter"
	"anomaly_detection_system/backend/internal/grpcclient"
)

// Pipeline orchestrates Producer -> OrderedPool -> Broadcaster.
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

	pool := NewOrderedPool(snap.Workers, snap.Workers*2)
	processFn := MakeProcessFunc(p.grpcClient)
	pool.Start(ctx, processFn)

	producer := NewProducer(snap.SourceType, snap.SourceAddr, snap.FPS)

	// Producer goroutine
	go func() {
		if err := producer.Run(ctx, pool); err != nil {
			log.Printf("Producer error: %v", err)
		}
		pool.Close()
		pool.CloseResults()
	}()

	// Consumer goroutine
	go func() {
		for r := range pool.OrderedCh {
			broadcaster.HandleResult(r)
		}
		log.Println("Pipeline consumer stopped")
	}()

	p.running = true
	log.Println("Pipeline started")
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
