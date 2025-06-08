#!/bin/bash

echo "Testing batch API functionality..."

# Build and start server in background
export PATH="/usr/local/go/bin:$PATH"
go build -o server cmd/server/main.go

echo "Starting server..."
./server -a "localhost:8085" &
SERVER_PID=$!
sleep 2

echo ""
echo "=== Test 1: Valid batch request ==="
curl -X POST http://localhost:8085/updates/ \
  -H "Content-Type: application/json" \
  -d '[
    {
      "id": "cpu_usage",
      "type": "gauge", 
      "value": 85.5
    },
    {
      "id": "requests_total",
      "type": "counter",
      "delta": 10
    },
    {
      "id": "memory_usage", 
      "type": "gauge",
      "value": 67.2
    }
  ]' | jq '.' 2>/dev/null || echo "Response received"

echo ""
echo ""
echo "=== Test 2: Empty batch (should fail) ==="
curl -X POST http://localhost:8085/updates/ \
  -H "Content-Type: application/json" \
  -d '[]' 

echo ""
echo ""
echo "=== Test 3: Invalid content type (should fail) ==="
curl -X POST http://localhost:8085/updates/ \
  -H "Content-Type: text/plain" \
  -d '[{"id":"test","type":"gauge","value":1.0}]'

echo ""
echo ""
echo "=== Test 4: Verify metrics were stored ==="
echo "Getting cpu_usage:"
curl -s http://localhost:8085/value/gauge/cpu_usage

echo ""
echo "Getting requests_total:"
curl -s http://localhost:8085/value/counter/requests_total

echo ""
echo "Getting memory_usage:"
curl -s http://localhost:8085/value/gauge/memory_usage

echo ""
echo ""
echo "=== Test 5: Gzip compressed batch ==="
echo '[{"id":"compressed_metric","type":"gauge","value":99.9}]' | gzip | \
curl -X POST http://localhost:8085/updates/ \
  -H "Content-Type: application/json" \
  -H "Content-Encoding: gzip" \
  --data-binary @-

echo ""
echo ""
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo "Batch API tests completed!" 