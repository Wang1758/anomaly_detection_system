package pipeline

import (
	"container/heap"
	"context"
	"log"
	"sync"
	"time"

	pb "anomaly_detection_system/backend/internal/grpcclient/pb"
)

const defaultGapTimeout = 1 * time.Second

// Task is a unit of work carrying a frame and its sequence number.
type Task struct {
	SeqNo      int64
	ImageBytes []byte
}

// OrderedResult pairs a sequence number with its detection response.
type OrderedResult struct {
	SeqNo  int64
	Result *pb.DetectResponse
	Err    error
}

// resultHeap is a min-heap ordered by SeqNo for reordering.
type resultHeap []*OrderedResult

func (h resultHeap) Len() int            { return len(h) }
func (h resultHeap) Less(i, j int) bool  { return h[i].SeqNo < h[j].SeqNo }
func (h resultHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *resultHeap) Push(x interface{}) { *h = append(*h, x.(*OrderedResult)) }
func (h *resultHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// ProcessFunc is called by workers to process a task.
type ProcessFunc func(ctx context.Context, task *Task) *OrderedResult

// OrderedPool runs N workers concurrently and reorders results by SeqNo.
type OrderedPool struct {
	workers   int
	inputCh   chan *Task
	resultCh  chan *OrderedResult
	OrderedCh chan *OrderedResult

	gapTimeout      time.Duration
	workerWg        sync.WaitGroup
	inputCloseOnce  sync.Once
	resultCloseOnce sync.Once
}

func NewOrderedPool(workers, bufSize int) *OrderedPool {
	return &OrderedPool{
		workers:    workers,
		inputCh:    make(chan *Task, bufSize),
		resultCh:   make(chan *OrderedResult, bufSize),
		OrderedCh:  make(chan *OrderedResult, bufSize),
		gapTimeout: defaultGapTimeout,
	}
}

// Submit enqueues a task. Returns false if context is done.
func (p *OrderedPool) Submit(ctx context.Context, task *Task) bool {
	select {
	case p.inputCh <- task:
		return true
	case <-ctx.Done():
		return false
	}
}

// Start launches workers and the reorder goroutine.
func (p *OrderedPool) Start(ctx context.Context, fn ProcessFunc) {
	// Launch workers
	for i := 0; i < p.workers; i++ {
		p.workerWg.Add(1)
		go func() {
			defer p.workerWg.Done()
			for {
				select {
				case task, ok := <-p.inputCh:
					if !ok {
						return
					}
					result := fn(ctx, task)
					select {
					case p.resultCh <- result:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		p.workerWg.Wait()
		p.resultCloseOnce.Do(func() {
			close(p.resultCh)
		})
	}()

	// Reorder goroutine: buffer out-of-order results and emit in sequence
	go func() {
		defer close(p.OrderedCh)
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

		resetGapTimer := func() {
			if p.gapTimeout <= 0 {
				return
			}
			if gapTimer == nil {
				gapTimer = time.NewTimer(p.gapTimeout)
				gapTimerCh = gapTimer.C
				return
			}
			if !gapTimer.Stop() {
				select {
				case <-gapTimer.C:
				default:
				}
			}
			gapTimer.Reset(p.gapTimeout)
			gapTimerCh = gapTimer.C
		}

		emitReady := func() bool {
			for h.Len() > 0 && h[0].SeqNo < nextSeq {
				late := heap.Pop(&h).(*OrderedResult)
				log.Printf("OrderedPool dropping late result: seq=%d expected=%d", late.SeqNo, nextSeq)
			}

			for h.Len() > 0 && h[0].SeqNo == nextSeq {
				item := heap.Pop(&h).(*OrderedResult)
				select {
				case p.OrderedCh <- item:
				case <-ctx.Done():
					return false
				}
				nextSeq++
			}

			if h.Len() > 0 && h[0].SeqNo > nextSeq {
				resetGapTimer()
			} else {
				stopGapTimer()
			}

			return true
		}

		defer stopGapTimer()

		for {
			select {
			case r, ok := <-p.resultCh:
				if !ok {
					// Flush remaining buffered results in order
					for h.Len() > 0 {
						item := heap.Pop(&h).(*OrderedResult)
						p.OrderedCh <- item
					}
					return
				}
				heap.Push(&h, r)
				if !emitReady() {
					return
				}
			case <-gapTimerCh:
				if h.Len() == 0 || h[0].SeqNo <= nextSeq {
					stopGapTimer()
					continue
				}

				log.Printf("OrderedPool timeout waiting for seq=%d, skipping to seq=%d", nextSeq, h[0].SeqNo)
				nextSeq = h[0].SeqNo
				if !emitReady() {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Close signals no more input.
func (p *OrderedPool) Close() {
	p.inputCloseOnce.Do(func() {
		close(p.inputCh)
	})
	log.Println("OrderedPool input closed, draining...")
}

// CloseResults signals no more results (called after all workers exit).
func (p *OrderedPool) CloseResults() {
	p.workerWg.Wait()
	p.resultCloseOnce.Do(func() {
		close(p.resultCh)
	})
}
