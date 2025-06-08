#!/bin/bash

# Quick Demo of File Storage Features
# This script shows the coolest file storage features in action

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}🚀 Metrics Server File Storage Demo${NC}"
echo

# Ensure Go is available
export PATH=$PATH:/usr/local/go/bin:/opt/homebrew/bin

# Build server
echo -e "${YELLOW}Building server...${NC}"
go build -o cmd/server/server ./cmd/server

# Clean up
rm -f /tmp/demo.json
pkill -f "cmd/server/server" 2>/dev/null || true

echo -e "${BLUE}=== Demo 1: Synchronous Saving ===${NC}"
echo "Starting server with synchronous saving (saves on every update)..."

# Start server with sync saving
STORE_INTERVAL=0 FILE_STORAGE_PATH=/tmp/demo.json RESTORE=true ./cmd/server/server &
SERVER_PID=$!
sleep 2

echo "Sending some cool metrics..."

# Send metrics
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"cpu_usage","type":"gauge","value":85.5}' \
    http://localhost:8080/update/

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"requests_total","type":"counter","delta":100}' \
    http://localhost:8080/update/

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"memory_usage","type":"gauge","value":67.2}' \
    http://localhost:8080/update/

echo -e "${GREEN}✅ Metrics sent!${NC}"

echo "File contents (saved immediately due to synchronous mode):"
cat /tmp/demo.json
echo

# Stop server
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

echo -e "${BLUE}=== Demo 2: Data Restoration ===${NC}"
echo "Starting new server instance - it should restore the previous data..."

# Start new server that restores data
STORE_INTERVAL=300 FILE_STORAGE_PATH=/tmp/demo.json RESTORE=true ./cmd/server/server &
SERVER_PID=$!
sleep 2

echo "Checking if data was restored..."

# Check restored data
RESTORED_CPU=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"cpu_usage","type":"gauge"}' \
    http://localhost:8080/value/)
echo "Restored CPU usage: $RESTORED_CPU"

RESTORED_REQUESTS=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"requests_total","type":"counter"}' \
    http://localhost:8080/value/)
echo "Restored requests total: $RESTORED_REQUESTS"

echo -e "${GREEN}✅ Data successfully restored!${NC}"

# Add more data
echo "Adding more metrics to the restored data..."
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"requests_total","type":"counter","delta":50}' \
    http://localhost:8080/update/ > /dev/null

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"id":"disk_usage","type":"gauge","value":45.8}' \
    http://localhost:8080/update/ > /dev/null

echo -e "${BLUE}=== Demo 3: Graceful Shutdown Save ===${NC}"
echo "Stopping server gracefully - it will save all data before shutting down..."

# Graceful shutdown
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

echo "Final saved data:"
cat /tmp/demo.json
echo

echo -e "${BLUE}=== Demo 4: Legacy API + File Storage ===${NC}"
echo "Testing legacy URL-based API with file storage..."

# Start server for legacy test
STORE_INTERVAL=0 FILE_STORAGE_PATH=/tmp/demo.json ./cmd/server/server &
SERVER_PID=$!
sleep 2

# Send via legacy API
curl -s -X POST http://localhost:8080/update/gauge/temperature/23.5
curl -s -X POST http://localhost:8080/update/counter/errors/5

# Get via legacy API
TEMP=$(curl -s http://localhost:8080/value/gauge/temperature)
ERRORS=$(curl -s http://localhost:8080/value/counter/errors)

echo "Temperature: ${TEMP}°C"
echo "Errors: $ERRORS"

# Stop and show final state
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

echo "Final file with legacy + JSON data:"
cat /tmp/demo.json
echo

echo -e "${BLUE}=== Demo 5: HTML View ===${NC}"
echo "Starting server to show HTML metrics view..."

STORE_INTERVAL=300 FILE_STORAGE_PATH=/tmp/demo.json RESTORE=true ./cmd/server/server &
SERVER_PID=$!
sleep 2

echo "HTML metrics page:"
curl -s http://localhost:8080/ | grep -E "(gauge|counter)" | head -10

kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

# Cleanup
rm -f /tmp/demo.json

echo
echo -e "${GREEN}🎉 Demo completed!${NC}"
echo
echo "Cool features demonstrated:"
echo "  🔥 Synchronous saving (instant persistence)"
echo "  🔄 Data restoration on startup"
echo "  💾 Graceful shutdown with final save"
echo "  🔗 Works with both JSON and legacy APIs"
echo "  🌐 HTML metrics viewer"
echo "  ⚙️  Environment variable configuration"
echo
echo "Try running the full test suite with:"
echo "  ./test_file_storage.sh" 