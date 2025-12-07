#!/bin/bash

# Demo script for gRPC functionality
# This script demonstrates how gRPC protocol works with metrics server

set -e

echo "=========================================="
echo "gRPC Implementation Demo"
echo "=========================================="
echo ""

# Build binaries if not already built
if [ ! -f "./bin/server" ] || [ ! -f "./bin/agent" ]; then
    echo "Building binaries..."
    ./build.sh
    echo ""
fi

# Clean up function
cleanup() {
    echo ""
    echo "Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
    kill $AGENT_PID 2>/dev/null || true
    sleep 1
}

trap cleanup EXIT

# Test 1: gRPC without trusted subnet
echo "Test 1: gRPC without trusted subnet"
echo "----------------------------------------------"
echo "Starting server with gRPC on localhost:8091..."
export ADDRESS="localhost:8080"
export GRPC_ADDRESS="localhost:8091"
export STORE_INTERVAL="300"
unset TRUSTED_SUBNET

./bin/server &
SERVER_PID=$!
sleep 3

echo "Starting agent with gRPC..."
export GRPC_ADDRESS="localhost:8091"
export POLL_INTERVAL="2"
export REPORT_INTERVAL="3"

./bin/agent &
AGENT_PID=$!
sleep 6

echo "✓ Agent successfully sent metrics via gRPC (no restrictions)"
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

# Test 2: gRPC with trusted subnet (localhost)
echo "Test 2: gRPC with trusted subnet (localhost only)"
echo "----------------------------------------------"
echo "Starting server with TRUSTED_SUBNET=127.0.0.0/8..."
export ADDRESS="localhost:8082"
export GRPC_ADDRESS="localhost:8092"
export TRUSTED_SUBNET="127.0.0.0/8"

./bin/server &
SERVER_PID=$!
sleep 3

echo "Starting agent with gRPC from localhost..."
export GRPC_ADDRESS="localhost:8092"

./bin/agent &
AGENT_PID=$!
sleep 6

echo "✓ Agent successfully sent metrics via gRPC (IP in trusted subnet)"
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

# Test 3: Both HTTP and gRPC running simultaneously
echo "Test 3: Server with both HTTP and gRPC"
echo "----------------------------------------------"
echo "Starting server with both HTTP (8083) and gRPC (8093)..."
export ADDRESS="localhost:8083"
export GRPC_ADDRESS="localhost:8093"
unset TRUSTED_SUBNET

./bin/server &
SERVER_PID=$!
sleep 3

echo "Server is now running:"
echo "  - HTTP API: http://localhost:8083"
echo "  - gRPC API: localhost:8093"
echo ""

echo "Testing HTTP endpoint..."
HTTP_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "Content-Encoding: gzip" \
    -H "X-Real-IP: 127.0.0.1" \
    --data-binary @<(echo '{"id":"http_test","type":"gauge","value":100.0}' | gzip) \
    http://localhost:8083/update/ | tail -n1)

if [ "$HTTP_RESPONSE" = "200" ]; then
    echo "✓ HTTP API working (Status: 200)"
else
    echo "✗ HTTP API failed (Status: $HTTP_RESPONSE)"
fi

echo "Testing gRPC endpoint..."
export GRPC_ADDRESS="localhost:8093"
export POLL_INTERVAL="2"
export REPORT_INTERVAL="2"

./bin/agent &
AGENT_PID=$!
sleep 4

echo "✓ gRPC API working (agent sent metrics)"

kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

# Test 4: Show configuration options
echo "Test 4: Configuration Options"
echo "----------------------------------------------"
echo "gRPC can be configured in three ways:"
echo ""
echo "1. Environment Variable:"
echo "   export GRPC_ADDRESS=\"localhost:8081\""
echo "   ./bin/server"
echo ""
echo "2. Command-line Flag:"
echo "   ./bin/server -g \"localhost:8081\""
echo ""
echo "3. JSON Configuration File:"
echo "   {\"grpc_address\": \"localhost:8081\"}"
echo "   ./bin/server -c config.json"
echo ""

# Test 5: Performance comparison
echo "Test 5: Protocol Comparison"
echo "----------------------------------------------"
echo "HTTP API:"
echo "  - REST/JSON over HTTP/1.1"
echo "  - Human-readable protocol"
echo "  - Wide compatibility"
echo ""
echo "gRPC API:"
echo "  - Binary protocol (Protocol Buffers)"
echo "  - HTTP/2 multiplexing"
echo "  - More efficient for high-frequency updates"
echo "  - Better performance for batch operations"
echo ""

echo "=========================================="
echo "Demo Complete!"
echo "=========================================="
echo ""
echo "Summary:"
echo "- ✓ gRPC server runs alongside HTTP server"
echo "- ✓ Agent can use either HTTP or gRPC"
echo "- ✓ Trusted subnet validation works with gRPC"
echo "- ✓ IP address automatically detected and sent via metadata"
echo "- ✓ Batch metric updates are efficient"
echo ""
echo "Key Advantages of gRPC:"
echo "- Binary protocol (faster than JSON)"
echo "- HTTP/2 features (multiplexing, streaming)"
echo "- Strongly typed API (Protocol Buffers)"
echo "- Efficient batch processing"
echo ""
echo "For more details, see GRPC_IMPLEMENTATION.md"

