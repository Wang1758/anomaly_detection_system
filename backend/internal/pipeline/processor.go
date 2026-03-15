package pipeline

import (
	"context"
	"log"

	"anomaly_detection_system/backend/internal/grpcclient"
)

// Processor creates the ProcessFunc that calls gRPC for each frame.
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
