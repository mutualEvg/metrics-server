package grpcserver

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/mutualEvg/metrics-server/internal/proto"
	"github.com/mutualEvg/metrics-server/storage"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T, trustedSubnet string) (*grpc.Server, *bufconn.Listener, storage.Storage) {
	lis := bufconn.Listen(bufSize)

	store := storage.NewMemStorage()

	var opts []grpc.ServerOption
	if trustedSubnet != "" {
		opts = append(opts, grpc.UnaryInterceptor(TrustedSubnetInterceptor(trustedSubnet)))
	}
	s := grpc.NewServer(opts...)

	metricsServer := NewMetricsServer(store)
	pb.RegisterMetricsServer(s, metricsServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	return s, lis, store
}

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestGRPCUpdateMetrics(t *testing.T) {
	s, lis, store := setupTestServer(t, "")
	defer s.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsClient(conn)

	// Test updating gauge metric
	gaugeValue := 42.5
	req := &pb.UpdateMetricsRequest{
		Metrics: []*pb.Metric{
			{
				Id:    "test_gauge",
				Type:  pb.Metric_GAUGE,
				Value: gaugeValue,
			},
		},
	}

	_, err = client.UpdateMetrics(ctx, req)
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Verify metric was stored
	value, exists := store.GetGauge("test_gauge")
	if !exists {
		t.Errorf("Gauge metric was not stored")
	}
	if value != gaugeValue {
		t.Errorf("Expected gauge value %f, got %f", gaugeValue, value)
	}

	// Test updating counter metric
	counterDelta := int64(10)
	req = &pb.UpdateMetricsRequest{
		Metrics: []*pb.Metric{
			{
				Id:    "test_counter",
				Type:  pb.Metric_COUNTER,
				Delta: counterDelta,
			},
		},
	}

	_, err = client.UpdateMetrics(ctx, req)
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Verify counter was stored
	delta, exists := store.GetCounter("test_counter")
	if !exists {
		t.Errorf("Counter metric was not stored")
	}
	if delta != counterDelta {
		t.Errorf("Expected counter delta %d, got %d", counterDelta, delta)
	}
}

func TestGRPCBatchUpdate(t *testing.T) {
	s, lis, store := setupTestServer(t, "")
	defer s.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsClient(conn)

	// Test batch update
	req := &pb.UpdateMetricsRequest{
		Metrics: []*pb.Metric{
			{
				Id:    "gauge1",
				Type:  pb.Metric_GAUGE,
				Value: 10.0,
			},
			{
				Id:    "gauge2",
				Type:  pb.Metric_GAUGE,
				Value: 20.0,
			},
			{
				Id:    "counter1",
				Type:  pb.Metric_COUNTER,
				Delta: 5,
			},
			{
				Id:    "counter2",
				Type:  pb.Metric_COUNTER,
				Delta: 15,
			},
		},
	}

	_, err = client.UpdateMetrics(ctx, req)
	if err != nil {
		t.Fatalf("Batch UpdateMetrics failed: %v", err)
	}

	// Verify all metrics were stored
	tests := []struct {
		name    string
		isGauge bool
		value   float64
		delta   int64
	}{
		{"gauge1", true, 10.0, 0},
		{"gauge2", true, 20.0, 0},
		{"counter1", false, 0, 5},
		{"counter2", false, 0, 15},
	}

	for _, tt := range tests {
		if tt.isGauge {
			value, exists := store.GetGauge(tt.name)
			if !exists {
				t.Errorf("Gauge %s was not stored", tt.name)
			}
			if value != tt.value {
				t.Errorf("Expected %s value %f, got %f", tt.name, tt.value, value)
			}
		} else {
			delta, exists := store.GetCounter(tt.name)
			if !exists {
				t.Errorf("Counter %s was not stored", tt.name)
			}
			if delta != tt.delta {
				t.Errorf("Expected %s delta %d, got %d", tt.name, tt.delta, delta)
			}
		}
	}
}

func TestGRPCTrustedSubnetInterceptor(t *testing.T) {
	tests := []struct {
		name          string
		trustedSubnet string
		realIP        string
		shouldSucceed bool
		expectedCode  codes.Code
	}{
		{
			name:          "No trusted subnet - allow all",
			trustedSubnet: "",
			realIP:        "192.168.1.1",
			shouldSucceed: true,
		},
		{
			name:          "IP in trusted subnet",
			trustedSubnet: "192.168.1.0/24",
			realIP:        "192.168.1.100",
			shouldSucceed: true,
		},
		{
			name:          "IP outside trusted subnet",
			trustedSubnet: "192.168.1.0/24",
			realIP:        "10.0.0.1",
			shouldSucceed: false,
			expectedCode:  codes.PermissionDenied,
		},
		{
			name:          "Missing x-real-ip metadata",
			trustedSubnet: "192.168.1.0/24",
			realIP:        "",
			shouldSucceed: false,
			expectedCode:  codes.PermissionDenied,
		},
		{
			name:          "Invalid IP format",
			trustedSubnet: "192.168.1.0/24",
			realIP:        "invalid-ip",
			shouldSucceed: false,
			expectedCode:  codes.PermissionDenied,
		},
		{
			name:          "Localhost in trusted subnet",
			trustedSubnet: "127.0.0.0/8",
			realIP:        "127.0.0.1",
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, lis, _ := setupTestServer(t, tt.trustedSubnet)
			defer s.Stop()

			ctx := context.Background()
			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithContextDialer(bufDialer(lis)),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewMetricsClient(conn)

			// Add x-real-ip metadata if specified
			if tt.realIP != "" {
				md := metadata.New(map[string]string{
					"x-real-ip": tt.realIP,
				})
				ctx = metadata.NewOutgoingContext(ctx, md)
			}

			// Try to update metrics
			req := &pb.UpdateMetricsRequest{
				Metrics: []*pb.Metric{
					{
						Id:    "test",
						Type:  pb.Metric_GAUGE,
						Value: 42.0,
					},
				},
			}

			_, err = client.UpdateMetrics(ctx, req)

			if tt.shouldSucceed {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error, got success")
				} else {
					st, ok := status.FromError(err)
					if !ok {
						t.Errorf("Expected gRPC status error, got: %v", err)
					} else if st.Code() != tt.expectedCode {
						t.Errorf("Expected error code %v, got %v", tt.expectedCode, st.Code())
					}
				}
			}
		})
	}
}

func TestGRPCInvalidMetricType(t *testing.T) {
	s, lis, _ := setupTestServer(t, "")
	defer s.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsClient(conn)

	// Test with invalid metric type (enum value 999)
	req := &pb.UpdateMetricsRequest{
		Metrics: []*pb.Metric{
			{
				Id:   "invalid",
				Type: pb.Metric_MType(999),
			},
		},
	}

	_, err = client.UpdateMetrics(ctx, req)
	if err == nil {
		t.Errorf("Expected error for invalid metric type, got success")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Errorf("Expected gRPC status error, got: %v", err)
	} else if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument error, got %v", st.Code())
	}
}
