package pipeline

import (
	"context"
	"log"
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

// BatchProcessor collects frames into batches and sends them to the AI service
// via BatchDetect gRPC. This mirrors the 乌骨鸡 project's batch inference
// pattern (model.predict(source=image_list)) for significantly higher GPU
// throughput: batch of 8 is 3-5x faster than 8 sequential calls.
type BatchProcessor struct {
	client       *grpcclient.Client
	batchSize    int
	batchTimeout time.Duration
}

func NewBatchProcessor(client *grpcclient.Client, batchSize int, batchTimeoutMs int) *BatchProcessor {
	return &BatchProcessor{
		client:       client,
		batchSize:    batchSize,
		batchTimeout: time.Duration(batchTimeoutMs) * time.Millisecond,
	}
}

// Run reads tasks from inputCh, batches them, calls BatchDetect, and sends
// ordered results to outputCh. Results are naturally ordered because each
// batch is processed sequentially and items within a batch preserve order.
func (bp *BatchProcessor) Run(ctx context.Context, inputCh <-chan *Task, outputCh chan<- *OrderedResult) {
	var batch []*Task
	timer := time.NewTimer(bp.batchTimeout)
	timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		images := make([][]byte, len(batch))
		frameIDs := make([]int64, len(batch))
		for i, t := range batch {
			images[i] = t.ImageBytes
			frameIDs[i] = t.SeqNo
		}

		resp, err := bp.client.BatchDetect(ctx, images, frameIDs)
		if err != nil {
			log.Printf("BatchDetect error for %d frames: %v", len(batch), err)
			for _, t := range batch {
				select {
				case outputCh <- &OrderedResult{SeqNo: t.SeqNo, Err: err}:
				case <-ctx.Done():
					return
				}
			}
		} else {
			results := resp.GetResults()
			for i, t := range batch {
				var r *OrderedResult
				if i < len(results) {
					r = &OrderedResult{SeqNo: t.SeqNo, Result: results[i]}
				} else {
					r = &OrderedResult{SeqNo: t.SeqNo, Err: nil}
				}
				select {
				case outputCh <- r:
				case <-ctx.Done():
					return
				}
			}
		}

		batch = batch[:0]
	}

	for {
		select {
		case task, ok := <-inputCh:
			if !ok {
				flush()
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
			return
		}
	}
}
