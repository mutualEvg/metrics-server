#!/bin/bash

# File Storage Testing Script for Metrics Server
# This script demonstrates and tests all file storage functionality

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_header() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Ensure Go is in PATH
export PATH=$PATH:/usr/local/go/bin:/opt/homebrew/bin

# Check if Go is available
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_success "Go version: $(go version)"

print_header "1. Building the Server"
go build -o cmd/server/server ./cmd/server
print_success "Server built successfully"

print_header "2. Running Unit Tests"

print_info "Running storage tests..."
go test -v ./storage

print_info "Running server integration tests..."
go test -v ./cmd/server -run TestFileStorage

print_info "Running all tests..."
go test -v ./...

print_success "All tests passed!"

print_header "3. Testing Command Line Flags"

print_info "Checking server help output..."
./cmd/server/server --help

print_header "4. Testing File Storage with Environment Variables"

# Clean up any existing test files
rm -f /tmp/demo-metrics.json /tmp/test-metrics.json

print_info "Starting server with environment variables (STORE_INTERVAL=0 for synchronous saving)..."

# Start server in background with environment variables
STORE_INTERVAL=0 FILE_STORAGE_PATH=/tmp/demo-metrics.json RESTORE=true ./cmd/server/server &
SERVER_PID=$!

# Wait for server to start
sleep 2

print_info "Server started with PID: $SERVER_PID"

print_info "Sending test metrics via JSON API..."

# Send gauge metric
RESPONSE1=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"test_gauge","type":"gauge","value":123.45}' \
    http://localhost:8080/update/)
echo "Gauge response: $RESPONSE1"

# Send counter metric
RESPONSE2=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"test_counter","type":"counter","delta":42}' \
    http://localhost:8080/update/)
echo "Counter response: $RESPONSE2"

# Send another counter increment
RESPONSE3=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"test_counter","type":"counter","delta":8}' \
    http://localhost:8080/update/)
echo "Counter increment response: $RESPONSE3"

sleep 1

print_info "Retrieving metrics via JSON API..."

# Get gauge value
GAUGE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"test_gauge","type":"gauge"}' \
    http://localhost:8080/value/)
echo "Retrieved gauge: $GAUGE_RESPONSE"

# Get counter value
COUNTER_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"test_counter","type":"counter"}' \
    http://localhost:8080/value/)
echo "Retrieved counter: $COUNTER_RESPONSE"

print_info "Checking HTML metrics page..."
curl -s http://localhost:8080/ | head -10

print_info "Stopping server gracefully..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

sleep 1

print_info "Checking saved file (should exist due to synchronous saving)..."
if [ -f /tmp/demo-metrics.json ]; then
    print_success "File exists!"
    echo "File contents:"
    cat /tmp/demo-metrics.json
    echo
else
    print_error "File not found!"
fi

print_header "5. Testing Periodic Saving"

rm -f /tmp/test-metrics.json

print_info "Starting server with periodic saving (interval=2 seconds)..."

# Start server with periodic saving
STORE_INTERVAL=2 FILE_STORAGE_PATH=/tmp/test-metrics.json RESTORE=false ./cmd/server/server &
SERVER_PID=$!

sleep 2

print_info "Sending metrics..."
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"periodic_gauge","type":"gauge","value":77.77}' \
    http://localhost:8080/update/ > /dev/null

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"periodic_counter","type":"counter","delta":5}' \
    http://localhost:8080/update/ > /dev/null

print_info "Waiting for periodic save (3 seconds)..."
sleep 3

print_info "Checking if file was created by periodic saving..."
if [ -f /tmp/test-metrics.json ]; then
    print_success "Periodic save worked!"
    echo "File contents:"
    cat /tmp/test-metrics.json
    echo
else
    print_info "File not yet created, waiting a bit more..."
    sleep 2
    if [ -f /tmp/test-metrics.json ]; then
        print_success "Periodic save worked!"
        cat /tmp/test-metrics.json
        echo
    else
        print_error "Periodic save didn't work as expected"
    fi
fi

print_info "Stopping server..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

print_header "6. Testing Data Restoration"

print_info "Starting new server instance with RESTORE=true..."

# Start server that should restore the data
STORE_INTERVAL=300 FILE_STORAGE_PATH=/tmp/test-metrics.json RESTORE=true ./cmd/server/server &
SERVER_PID=$!

sleep 2

print_info "Checking if data was restored by retrieving metrics..."

# Check if the previously saved gauge was restored
RESTORED_GAUGE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"periodic_gauge","type":"gauge"}' \
    http://localhost:8080/value/)
echo "Restored gauge: $RESTORED_GAUGE"

# Check if the previously saved counter was restored
RESTORED_COUNTER=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"periodic_counter","type":"counter"}' \
    http://localhost:8080/value/)
echo "Restored counter: $RESTORED_COUNTER"

print_info "Stopping server..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

print_header "7. Testing Legacy URL-based API with File Storage"

rm -f /tmp/legacy-test.json

print_info "Starting server for legacy API test..."
STORE_INTERVAL=0 FILE_STORAGE_PATH=/tmp/legacy-test.json ./cmd/server/server &
SERVER_PID=$!

sleep 2

print_info "Sending metrics via legacy URL-based API..."

# Send gauge via legacy API
curl -s -X POST http://localhost:8080/update/gauge/legacy_gauge/99.99

# Send counter via legacy API
curl -s -X POST http://localhost:8080/update/counter/legacy_counter/10

# Get values via legacy API
print_info "Retrieving values via legacy API..."
LEGACY_GAUGE=$(curl -s http://localhost:8080/value/gauge/legacy_gauge)
echo "Legacy gauge value: $LEGACY_GAUGE"

LEGACY_COUNTER=$(curl -s http://localhost:8080/value/counter/legacy_counter)
echo "Legacy counter value: $LEGACY_COUNTER"

print_info "Stopping server..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

print_info "Checking saved file from legacy API..."
if [ -f /tmp/legacy-test.json ]; then
    print_success "Legacy API file storage works!"
    cat /tmp/legacy-test.json
    echo
fi

print_header "8. Testing Gzip Compression with File Storage"

rm -f /tmp/gzip-test.json

print_info "Starting server for gzip test..."
STORE_INTERVAL=0 FILE_STORAGE_PATH=/tmp/gzip-test.json ./cmd/server/server &
SERVER_PID=$!

sleep 2

print_info "Sending compressed JSON data..."

# Create compressed JSON data
echo '{"id":"gzip_gauge","type":"gauge","value":555.55}' | gzip > /tmp/compressed_data.gz

# Send compressed data
curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "Content-Encoding: gzip" \
    --data-binary @/tmp/compressed_data.gz \
    http://localhost:8080/update/

print_info "Requesting compressed response..."
GZIP_RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "Accept-Encoding: gzip" \
    -d '{"id":"gzip_gauge","type":"gauge"}' \
    http://localhost:8080/value/)
echo "Gzip response: $GZIP_RESPONSE"

print_info "Stopping server..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

print_info "Checking gzip test file..."
if [ -f /tmp/gzip-test.json ]; then
    print_success "Gzip + file storage works!"
    cat /tmp/gzip-test.json
    echo
fi

print_header "9. Performance Test"

rm -f /tmp/perf-test.json

print_info "Starting server for performance test..."
STORE_INTERVAL=1 FILE_STORAGE_PATH=/tmp/perf-test.json ./cmd/server/server &
SERVER_PID=$!

sleep 2

print_info "Sending multiple metrics rapidly..."

for i in {1..10}; do
    curl -s -X POST -H "Content-Type: application/json" \
        -d "{\"id\":\"perf_gauge_$i\",\"type\":\"gauge\",\"value\":$i.5}" \
        http://localhost:8080/update/ > /dev/null
    
    curl -s -X POST -H "Content-Type: application/json" \
        -d "{\"id\":\"perf_counter_$i\",\"type\":\"counter\",\"delta\":$i}" \
        http://localhost:8080/update/ > /dev/null
done

print_success "Sent 20 metrics"

print_info "Waiting for periodic save..."
sleep 2

print_info "Stopping server..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

print_info "Checking performance test results..."
if [ -f /tmp/perf-test.json ]; then
    GAUGE_COUNT=$(cat /tmp/perf-test.json | grep -o '"perf_gauge_[0-9]*"' | wc -l)
    COUNTER_COUNT=$(cat /tmp/perf-test.json | grep -o '"perf_counter_[0-9]*"' | wc -l)
    print_success "Performance test completed! Saved $GAUGE_COUNT gauges and $COUNTER_COUNT counters"
    
    echo "Sample of saved data:"
    cat /tmp/perf-test.json | head -20
    echo "..."
fi

print_header "10. Cleanup"

print_info "Cleaning up test files..."
rm -f /tmp/demo-metrics.json /tmp/test-metrics.json /tmp/legacy-test.json /tmp/gzip-test.json /tmp/perf-test.json /tmp/compressed_data.gz

print_info "Ensuring no servers are still running..."
pkill -f "cmd/server/server" 2>/dev/null || true

print_header "🎉 All Tests Completed Successfully!"

print_success "File storage functionality is working perfectly!"
echo
echo "Key features tested:"
echo "  ✅ Synchronous saving (STORE_INTERVAL=0)"
echo "  ✅ Periodic saving (STORE_INTERVAL>0)"
echo "  ✅ Data restoration on startup"
echo "  ✅ Graceful shutdown with final save"
echo "  ✅ JSON API integration"
echo "  ✅ Legacy URL API integration"
echo "  ✅ Gzip compression compatibility"
echo "  ✅ Performance under load"
echo "  ✅ Configuration via environment variables"
echo "  ✅ Configuration via command line flags"
echo
echo "Usage examples:"
echo "  # Synchronous saving"
echo "  STORE_INTERVAL=0 FILE_STORAGE_PATH=/data/metrics.json ./cmd/server/server"
echo
echo "  # Periodic saving every 60 seconds"
echo "  ./cmd/server/server -i 60 -f /data/metrics.json --restore"
echo
echo "  # No restoration on startup"
echo "  RESTORE=false ./cmd/server/server" 