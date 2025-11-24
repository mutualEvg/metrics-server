# gRPC Implementation Documentation

## Overview
This document describes the gRPC implementation for the metrics server, which allows agents to send metrics in batches using the gRPC protocol alongside the existing HTTP/REST API.

## Features

### 1. Protocol Definition (metrics.proto)
- Defined in `internal/proto/metrics.proto`
- Supports two metric types: GAUGE and COUNTER
- Batch updates via `UpdateMetrics` RPC
- Generated Go code in `internal/proto/metrics.pb.go` and `internal/proto/metrics_grpc.pb.go`

### 2. gRPC Server
- Located in `internal/grpcserver/server.go`
- Implements `UpdateMetrics` RPC method
- Stores metrics using the same storage interface as HTTP API
- Runs alongside HTTP server on a separate port

### 3. gRPC Client
- Located in `internal/grpcclient/client.go`
- Automatically adds `x-real-ip` metadata for subnet validation
- Sends metrics in batches
- Handles connection management and timeouts

### 4. Trusted Subnet Validation
- Implemented as a `UnaryInterceptor`
- Validates IP addresses from `x-real-ip` metadata
- Returns `codes.PermissionDenied` when IP is not in trusted subnet
- Same CIDR validation as HTTP middleware

### 5. Agent Integration
- Agent can use either HTTP or gRPC based on configuration
- When `GRPC_ADDRESS` is set, agent uses gRPC protocol
- Collects and sends metrics in batches
- Automatic IP detection and metadata injection

## Configuration

### Server Configuration

#### JSON Configuration
```json
{
    "address": "localhost:8080",
    "grpc_address": "localhost:8081",
    "trusted_subnet": "192.168.1.0/24"
}
```

#### Environment Variables
```bash
export GRPC_ADDRESS="localhost:8081"
export TRUSTED_SUBNET="192.168.1.0/24"
./bin/server
```

#### Command-line Flags
```bash
./bin/server -g "localhost:8081" -t "192.168.1.0/24"
```

### Agent Configuration

#### JSON Configuration
```json
{
    "address": "http://localhost:8080",
    "grpc_address": "localhost:8081"
}
```

#### Environment Variables
```bash
export GRPC_ADDRESS="localhost:8081"
./bin/agent
```

#### Command-line Flags
```bash
./bin/agent -g "localhost:8081"
```

## Usage Examples

### Example 1: HTTP Only (Default)
```bash
# Start server with HTTP only
./bin/server -a "localhost:8080"

# Start agent with HTTP
./bin/agent -a "http://localhost:8080"
```

### Example 2: gRPC with Trusted Subnet
```bash
# Start server with gRPC and trusted subnet
export GRPC_ADDRESS="localhost:8081"
export TRUSTED_SUBNET="127.0.0.0/8"
./bin/server

# Start agent with gRPC
export GRPC_ADDRESS="localhost:8081"
./bin/agent
```

### Example 3: Both HTTP and gRPC
```bash
# Server runs both HTTP and gRPC
./bin/server -a "localhost:8080" -g "localhost:8081"

# Agent uses gRPC (prefers gRPC if configured)
./bin/agent -g "localhost:8081"
```

## Protocol Behavior

### Message Format
```protobuf
message Metric {
  string id = 1;           // Metric name
  MType type = 2;          // GAUGE or COUNTER
  int64 delta = 3;         // For COUNTER metrics
  double value = 4;        // For GAUGE metrics
}

message UpdateMetricsRequest {
  repeated Metric metrics = 1;  // Batch of metrics
}
```

### Metadata
- `x-real-ip`: Contains the agent's IP address for subnet validation

### Error Codes
- `OK`: Metrics successfully stored
- `PermissionDenied`: IP not in trusted subnet or missing x-real-ip
- `InvalidArgument`: Unknown metric type
- `Internal`: Storage or server error

## Architecture

### Server Components
1. **gRPC Server**: Listens on configured gRPC address
2. **UnaryInterceptor**: Validates IP addresses before request processing
3. **MetricsServer**: Implements the Metrics service
4. **Storage Backend**: Shared with HTTP API (MemStorage or DBStorage)

### Agent Components
1. **gRPC Client**: Manages connection to gRPC server
2. **Metric Collector**: Collects runtime and system metrics
3. **Batch Sender**: Sends collected metrics in configurable intervals

### Data Flow
```
Agent                     gRPC Server              Storage
  |                            |                      |
  |--[Collect Metrics]-------->|                      |
  |                            |                      |
  |--[Add x-real-ip metadata]->|                      |
  |                            |                      |
  |--[UpdateMetrics RPC]------>|                      |
  |                            |--[Validate IP]------>|
  |                            |                      |
  |                            |--[Store Metrics]---->|
  |                            |                      |
  |<--[Response]---------------|<--[Confirm]----------|
```

## Testing

### Unit Tests
```bash
# Test gRPC server
go test -v ./internal/grpcserver/...

# Test gRPC client
go test -v ./internal/grpcclient/...
```

### Integration Tests
The implementation includes comprehensive tests for:
- Single metric updates
- Batch metric updates
- Trusted subnet validation
- Invalid metric types
- Missing metadata
- Invalid IP addresses

### Manual Testing

#### Test 1: gRPC with Localhost
```bash
# Terminal 1: Start server
export GRPC_ADDRESS="localhost:8081"
export TRUSTED_SUBNET="127.0.0.0/8"
./bin/server

# Terminal 2: Start agent
export GRPC_ADDRESS="localhost:8081"
./bin/agent

# Observe logs showing gRPC communication
```

#### Test 2: Trusted Subnet Validation
```bash
# Terminal 1: Start server with strict subnet
export GRPC_ADDRESS="localhost:8081"
export TRUSTED_SUBNET="192.168.1.0/24"
./bin/server

# Terminal 2: Agent with IP in subnet (will succeed)
export GRPC_ADDRESS="localhost:8081"
./bin/agent
```

## Performance Considerations

### gRPC Advantages
1. **Binary Protocol**: More efficient than JSON over HTTP
2. **HTTP/2**: Multiplexing and streaming support
3. **Batch Processing**: Send multiple metrics in one request
4. **Connection Reuse**: Persistent connections reduce overhead

### Batching
- Agent collects metrics over `PollInterval`
- Sends batch every `ReportInterval`
- Reduces network overhead compared to individual requests

### Timeouts
- Connection timeout: 10 seconds
- Request timeout: 10 seconds
- Configurable via context

## Security

### IP Validation
- Validates IP address from `x-real-ip` metadata
- Same CIDR validation as HTTP middleware
- Configurable trusted subnet

### Transport Security
- Currently uses insecure credentials (no TLS)
- **Production**: Should enable TLS with proper certificates
- Can be enhanced with mTLS for mutual authentication

### Recommended Production Setup
```go
// Server with TLS
creds, err := credentials.NewServerTLSFromFile("server.crt", "server.key")
opts := []grpc.ServerOption{
    grpc.Creds(creds),
    grpc.UnaryInterceptor(TrustedSubnetInterceptor(trustedSubnet)),
}
s := grpc.NewServer(opts...)
```

```go
// Client with TLS
creds, err := credentials.NewClientTLSFromFile("server.crt", "")
conn, err := grpc.Dial(address, grpc.WithTransportCredentials(creds))
```

## Troubleshooting

### Common Issues

#### 1. Connection Refused
```
failed to connect to gRPC server: connection refused
```
**Solution**: Ensure server is running and `GRPC_ADDRESS` matches

#### 2. Permission Denied
```
rpc error: code = PermissionDenied desc = IP not in trusted subnet
```
**Solution**: Check `TRUSTED_SUBNET` configuration or agent's IP address

#### 3. Missing Metadata
```
rpc error: code = PermissionDenied desc = x-real-ip not found in metadata
```
**Solution**: Verify agent is sending x-real-ip metadata correctly

#### 4. Invalid CIDR
```
Warning: Invalid trusted subnet CIDR
```
**Solution**: Fix CIDR notation (e.g., "192.168.1.0/24")

### Debug Logging
Enable verbose logging to troubleshoot:
```bash
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info
./bin/agent -g "localhost:8081"
```

## Backward Compatibility

- **HTTP API**: Remains fully functional
- **Existing Agents**: Continue to work without changes
- **Configuration**: gRPC is opt-in (configured via `GRPC_ADDRESS`)
- **Storage**: Shared backend ensures consistency

## Migration Guide

### From HTTP to gRPC

#### Step 1: Test with Both Protocols
```bash
# Keep HTTP running, add gRPC
./bin/server -a "localhost:8080" -g "localhost:8081"
```

#### Step 2: Update Agent Configuration
```bash
# Switch individual agents to gRPC
export GRPC_ADDRESS="localhost:8081"
./bin/agent
```

#### Step 3: Monitor and Validate
- Check server logs for gRPC requests
- Verify metrics are being stored correctly
- Compare performance metrics

#### Step 4: Complete Migration
- Update all agents to use gRPC
- Optionally disable HTTP if no longer needed

## Future Enhancements

### Potential Improvements
1. **TLS Support**: Add transport security
2. **Streaming**: Implement server-side streaming for real-time metrics
3. **Compression**: Enable gzip compression for large batches
4. **Authentication**: Add token-based authentication
5. **Load Balancing**: Support multiple gRPC servers
6. **Health Checks**: Implement gRPC health check protocol

## Code Examples

### Server Setup
```go
// Initialize storage
store := storage.NewMemStorage()

// Create gRPC server with interceptor
opts := []grpc.ServerOption{
    grpc.UnaryInterceptor(grpcserver.TrustedSubnetInterceptor("192.168.1.0/24")),
}
s := grpc.NewServer(opts...)

// Register service
metricsServer := grpcserver.NewMetricsServer(store)
pb.RegisterMetricsServer(s, metricsServer)

// Start server
lis, _ := net.Listen("tcp", ":8081")
s.Serve(lis)
```

### Client Usage
```go
// Create client
client, err := grpcclient.NewMetricsClient("localhost:8081")
defer client.Close()

// Prepare metrics
metrics := []models.Metrics{
    {ID: "cpu_usage", MType: "gauge", Value: &cpuValue},
    {ID: "requests", MType: "counter", Delta: &requestCount},
}

// Send metrics
err = client.SendMetrics(ctx, metrics)
```

## Summary

The gRPC implementation provides:
- High-performance binary protocol
- Batch metric updates
- IP-based access control
- Backward compatibility with HTTP API
- Comprehensive testing
- Easy configuration and deployment
- Production-ready with optional TLS

The implementation follows gRPC best practices and integrates seamlessly with the existing metrics server architecture.

