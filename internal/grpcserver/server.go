package grpcserver

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/mutualEvg/metrics-server/internal/proto"
	"github.com/mutualEvg/metrics-server/storage"
)

// MetricsServer implements the gRPC Metrics service
type MetricsServer struct {
	pb.UnimplementedMetricsServer
	storage storage.Storage
}

// NewMetricsServer creates a new gRPC metrics server
func NewMetricsServer(storage storage.Storage) *MetricsServer {
	return &MetricsServer{
		storage: storage,
	}
}

// UpdateMetrics implements the UpdateMetrics RPC method
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	log.Printf("Received gRPC UpdateMetrics request with %d metrics", len(req.Metrics))

	for _, metric := range req.Metrics {
		switch metric.Type {
		case pb.Metric_GAUGE:
			s.storage.UpdateGauge(metric.Id, metric.Value)
			log.Printf("Updated gauge metric: %s = %f", metric.Id, metric.Value)

		case pb.Metric_COUNTER:
			s.storage.UpdateCounter(metric.Id, metric.Delta)
			log.Printf("Updated counter metric: %s += %d", metric.Id, metric.Delta)

		default:
			log.Printf("Unknown metric type for %s", metric.Id)
			return nil, status.Errorf(codes.InvalidArgument, "unknown metric type")
		}
	}

	return &pb.UpdateMetricsResponse{}, nil
}

// TrustedSubnetInterceptor creates a UnaryInterceptor that validates IP addresses
// against a trusted subnet (CIDR notation). If trustedSubnet is empty, all requests are allowed.
func TrustedSubnetInterceptor(trustedSubnet string) grpc.UnaryServerInterceptor {
	var ipNet *net.IPNet
	var err error

	// Parse the trusted subnet if provided
	if trustedSubnet != "" {
		_, ipNet, err = net.ParseCIDR(trustedSubnet)
		if err != nil {
			log.Printf("Warning: Invalid trusted subnet CIDR %s: %v. All IPs will be allowed.", trustedSubnet, err)
			ipNet = nil
		} else {
			log.Printf("gRPC trusted subnet configured: %s", trustedSubnet)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// If no trusted subnet is configured, allow all requests
		if ipNet == nil {
			return handler(ctx, req)
		}

		// Extract metadata from context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Printf("gRPC request rejected: no metadata found")
			return nil, status.Error(codes.PermissionDenied, "no metadata found")
		}

		// Get X-Real-IP from metadata
		realIPs := md.Get("x-real-ip")
		if len(realIPs) == 0 {
			log.Printf("gRPC request rejected: x-real-ip not found in metadata")
			return nil, status.Error(codes.PermissionDenied, "x-real-ip not found in metadata")
		}

		realIP := realIPs[0]

		// Parse the IP address
		ip := net.ParseIP(realIP)
		if ip == nil {
			log.Printf("gRPC request rejected: invalid IP address in x-real-ip: %s", realIP)
			return nil, status.Error(codes.PermissionDenied, "invalid IP address in x-real-ip")
		}

		// Check if IP is in the trusted subnet
		if !ipNet.Contains(ip) {
			log.Printf("gRPC request from %s rejected: IP not in trusted subnet %s", realIP, trustedSubnet)
			return nil, status.Error(codes.PermissionDenied, "IP not in trusted subnet")
		}

		log.Printf("gRPC request from %s allowed (in trusted subnet)", realIP)
		return handler(ctx, req)
	}
}
