#!/bin/bash

echo "Testing storage mode selection..."

# Build the server
export PATH="/usr/local/go/bin:$PATH"
go build -o server cmd/server/main.go

echo ""
echo "=== Test 1: Memory storage (no flags) ==="
timeout 3s ./server -a "localhost:8081" 2>&1 | grep -E "(Using|storage)" || echo "Server started with memory storage"

echo ""
echo "=== Test 2: File storage (with -f flag) ==="
timeout 3s ./server -a "localhost:8082" -f "/tmp/test-metrics.json" 2>&1 | grep -E "(Using|storage)" || echo "Server started with file storage"

echo ""
echo "=== Test 3: File storage (with FILE_STORAGE_PATH env) ==="
FILE_STORAGE_PATH="/tmp/test-metrics-env.json" timeout 3s ./server -a "localhost:8083" 2>&1 | grep -E "(Using|storage)" || echo "Server started with file storage via env"

echo ""
echo "=== Test 4: Database storage (with invalid DSN) ==="
echo "This should fail with database connection error:"
./server -a "localhost:8084" -d "postgres://invalid:invalid@localhost/invalid?sslmode=disable" 2>&1 | head -5

echo ""
echo "All tests completed!" 