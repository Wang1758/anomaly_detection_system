package pipeline

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"anomaly_detection_system/backend/internal/grpcclient"
)

// MakeProcessFunc creates the ProcessFunc that calls single-frame gRPC Detect.
// Kept for backward compatibility; the batch pipeline uses BatchProcessor instead.
func MakeProcessFunc(client *grpcclient.Client) ProcessFunc {
	return func(ctx context.Context, task *Task) *OrderedResult {
		resp, err := client.Detect(ctx, task.ImageBytes, task.SeqNo)
		if err != nil {
			log.Printf("gRPC detect error for frame %d: %v", task.SeqNo, err)
			return &OrderedResult{SeqNo: task.SeqNo, Err: err}
		}
		return &OrderedResult{SeqNo: task.SeqNo, Result: resp}
	}
}

// BatchProcessor collects frames into batches and dispatches them to a pool of
// concurrent workers for parallel BatchDetect gRPC calls. Results are reordered
// by SeqNo before being emitted, mirroring 乌骨鸡 project's ordered-concurrently
// worker pool pattern for higher GPU throughput.
type BatchProcessor struct {
	client       *grpcclient.Client
	batchSize    int
	batchTimeout time.Duration
	workers      int
}

func NewBatchProcessor(client *grpcclient.Client, batchSize int, batchTimeoutMs int, workers int) *BatchProcessor {
	if workers < 1 {
		workers = 1
	}
	return &BatchProcessor{
		client:       client,
		batchSize:    batchSize,
		batchTimeout: time.Duration(batchTimeoutMs) * time.Millisecond,
		workers:      workers,
	}
}

// batchWork is a unit of concurrent work containing a batch of frames.
type batchWork struct {
	tasks []*Task
}

// Run reads tasks from inputCh, batches them, dispatches to a pool of concurrent
// workers, reorders results by SeqNo, and emits to outputCh.
func (bp *BatchProcessor) Run(ctx context.Context, inputCh <-chan *Task, outputCh chan<- *OrderedResult) {
	batchCh := make(chan batchWork, bp.workers*2)
	unorderedCh := make(chan []*OrderedResult, bp.workers*2)

	// Launch worker pool: each worker picks a batch, calls BatchDetect, sends results
	var workerWg sync.WaitGroup
	for i := 0; i < bp.workers; i++ {
		workerWg.Add(1)
		go func(id int) {
			defer workerWg.Done()
			for {
				select {
				case work, ok := <-batchCh:
					if !ok {
						return
					}
					results := bp.processBatch(ctx, work)
					select {
					case unorderedCh <- results:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	// Close unorderedCh when all workers exit
	go func() {
		workerWg.Wait()
		close(unorderedCh)
	}()

	// Reorder goroutine: buffer out-of-order batch results, emit in SeqNo order
	var reorderWg sync.WaitGroup
	reorderWg.Add(1)
	go bp.reorderResults(ctx, unorderedCh, outputCh, &reorderWg)

	log.Printf("BatchProcessor: started %d concurrent workers (batch_size=%d)", bp.workers, bp.batchSize)

	// Batch collection loop (same logic as before)
	var batch []*Task
	timer := time.NewTimer(bp.batchTimeout)
	timer.Stop()
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		work := batchWork{tasks: batch}
		select {
		case batchCh <- work:
		case <-ctx.Done():
		}
		batch = make([]*Task, 0, bp.batchSize)
	}

	for {
		select {
		case task, ok := <-inputCh:
			if !ok {
				flush()
				close(batchCh)
				reorderWg.Wait()
				return
			}
			batch = append(batch, task)
			if len(batch) == 1 {
				timer.Reset(bp.batchTimeout)
			}
			if len(batch) >= bp.batchSize {
				timer.Stop()
				flush()
			}
		case <-timer.C:
			flush()
		case <-ctx.Done():
			close(batchCh)
			reorderWg.Wait()
			return
		}
	}
}

// processBatch calls BatchDetect for a single batch and returns per-frame results.
func (bp *BatchProcessor) processBatch(ctx context.Context, work batchWork) []*OrderedResult {
	images := make([][]byte, len(work.tasks))
	frameIDs := make([]int64, len(work.tasks))
	for i, t := range work.tasks {
		images[i] = t.ImageBytes
		frameIDs[i] = t.SeqNo
	}

	results := make([]*OrderedResult, len(work.tasks))
	resp, err := bp.client.BatchDetect(ctx, images, frameIDs)
	if err != nil {
		log.Printf("BatchDetect error for %d frames: %v", len(work.tasks), err)
		for i, t := range work.tasks {
			results[i] = &OrderedResult{SeqNo: t.SeqNo, Err: err}
		}
		return results
	}

	apiResults := resp.GetResults()
	for i, t := range work.tasks {
		if i < len(apiResults) {
			results[i] = &OrderedResult{SeqNo: t.SeqNo, Result: apiResults[i]}
		} else {
			results[i] = &OrderedResult{SeqNo: t.SeqNo, Err: fmt.Errorf("batch result missing index %d", i)}
		}
	}
	return results
}

// reorderResults collects unordered batch results and emits them in SeqNo order
// using a min-heap, with a gap timeout to skip missing frames.
func (bp *BatchProcessor) reorderResults(ctx context.Context, unorderedCh <-chan []*OrderedResult, outputCh chan<- *OrderedResult, wg *sync.WaitGroup) {
	defer wg.Done()

	var h resultHeap
	heap.Init(&h)
	nextSeq := int64(0)

	var gapTimer *time.Timer
	var gapTimerCh <-chan time.Time

	stopGapTimer := func() {
		if gapTimer == nil {
			gapTimerCh = nil
			return
		}
		if !gapTimer.Stop() {
			select {
			case <-gapTimer.C:
			default:
			}
		}
		gapTimerCh = nil
	}

	startGapTimer := func() {
		if gapTimer == nil {
			gapTimer = time.NewTimer(defaultGapTimeout)
			gapTimerCh = gapTimer.C
			return
		}
		if gapTimerCh == nil {
			gapTimer.Reset(defaultGapTimeout)
			gapTimerCh = gapTimer.C
		}
	}

	emitReady := func() bool {
		for h.Len() > 0 && h[0].SeqNo < nextSeq {
			late := heap.Pop(&h).(*OrderedResult)
			log.Printf("BatchProcessor reorder: dropping late result seq=%d expected=%d", late.SeqNo, nextSeq)
		}
		for h.Len() > 0 && h[0].SeqNo == nextSeq {
			item := heap.Pop(&h).(*OrderedResult)
			select {
			case outputCh <- item:
			case <-ctx.Done():
				return false
			}
			nextSeq++
		}
		if h.Len() > 0 && h[0].SeqNo > nextSeq {
			startGapTimer()
		} else {
			stopGapTimer()
		}
		return true
	}

	defer stopGapTimer()

	for {
		select {
		case results, ok := <-unorderedCh:
			if !ok {
				for h.Len() > 0 {
					item := heap.Pop(&h).(*OrderedResult)
					outputCh <- item
				}
				return
			}
			for _, r := range results {
				heap.Push(&h, r)
			}
			if !emitReady() {
				return
			}
		case <-gapTimerCh:
			if h.Len() == 0 || h[0].SeqNo <= nextSeq {
				stopGapTimer()
				continue
			}
			log.Printf("BatchProcessor reorder: gap timeout, skipping seq=%d to seq=%d", nextSeq, h[0].SeqNo)
			nextSeq = h[0].SeqNo
			if !emitReady() {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
