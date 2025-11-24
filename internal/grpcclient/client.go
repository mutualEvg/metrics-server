package grpcclient

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/mutualEvg/metrics-server/internal/models"
	
)

// MetricsClient wraps the gRPC client for sending metrics
type MetricsClient struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
	realIP string
}

// NewMetricsClient creates a new gRPC metrics client
func NewMetricsClient(address string) (*MetricsClient, error) {
	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := pb.NewMetricsClient(conn)

	// Get outbound IP for x-real-ip metadata
	realIP := getOutboundIP()
	log.Printf("gRPC client initialized with IP: %s", realIP)

	return &MetricsClient{
		conn:   conn,
		client: client,
		realIP: realIP,
	}, nil
}

// Close closes the gRPC connection
func (c *MetricsClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendMetrics sends a batch of metrics to the gRPC server
func (c *MetricsClient) SendMetrics(ctx context.Context, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	// Convert internal metrics to protobuf metrics
	pbMetrics := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		pbMetric := &pb.Metric{
			Id: m.ID,
		}

		if m.MType == "gauge" && m.Value != nil {
			pbMetric.Type = pb.Metric_GAUGE
			pbMetric.Value = *m.Value
		} else if m.MType == "counter" && m.Delta != nil {
			pbMetric.Type = pb.Metric_COUNTER
			pbMetric.Delta = *m.Delta
		} else {
			log.Printf("Skipping metric %s with invalid type or value", m.ID)
			continue
		}

		pbMetrics = append(pbMetrics, pbMetric)
	}

	// Create request
	req := &pb.UpdateMetricsRequest{
		Metrics: pbMetrics,
	}

	// Add x-real-ip to metadata
	md := metadata.New(map[string]string{
		"x-real-ip": c.realIP,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Send request with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.client.UpdateMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send metrics via gRPC: %w", err)
	}

	log.Printf("Successfully sent %d metrics via gRPC", len(pbMetrics))
	return nil
}

// getOutboundIP gets the preferred outbound IP address of this machine
func getOutboundIP() string {
	// Try to get the outbound IP by connecting to a public DNS server
	// This doesn't actually send any data, just establishes which interface would be used
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1" // Fallback to localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
