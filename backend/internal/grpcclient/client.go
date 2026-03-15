package grpcclient

import (
	"context"
	"log"
	"time"

	pb "anomaly_detection_system/backend/internal/grpcclient/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.DetectionServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(50*1024*1024),
			grpc.MaxCallSendMsgSize(50*1024*1024),
		),
	)
	if err != nil {
		return nil, err
	}
	log.Printf("gRPC client connected to %s", addr)
	return &Client{conn: conn, client: pb.NewDetectionServiceClient(conn)}, nil
}

func (c *Client) Detect(ctx context.Context, imageBytes []byte, frameID int64) (*pb.DetectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.client.Detect(ctx, &pb.DetectRequest{
		Image:   imageBytes,
		FrameId: frameID,
	})
}

func (c *Client) ReloadModel(ctx context.Context, modelPath string) (*pb.ReloadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.client.ReloadModel(ctx, &pb.ReloadRequest{ModelPath: modelPath})
}

func (c *Client) UpdateParams(ctx context.Context, nms, conf, entropy, w1, w2 float32) (*pb.ParamsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.UpdateParams(ctx, &pb.ParamsRequest{
		NmsThreshold:        nms,
		ConfidenceThreshold: conf,
		EntropyThreshold:    entropy,
		W1:                  w1,
		W2:                  w2,
	})
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
