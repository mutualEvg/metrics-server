#!/bin/bash

echo "=== Testing Retry Functionality ==="
echo

# Clean up any existing processes
pkill -f "./server" 2>/dev/null || true
pkill -f "./agent" 2>/dev/null || true
sleep 1

echo "1. Starting agent without server (should show retry attempts)..."
echo "   Agent will try to connect and retry with intervals: 1s, 3s, 5s"
echo

# Start agent in background - it will fail and retry
./agent -a localhost:8080 -r 5 -p 2 -b 5 &
AGENT_PID=$!

echo "   Agent started (PID: $AGENT_PID)"
echo "   Waiting 10 seconds to see retry attempts..."
sleep 10

echo
echo "2. Starting server (agent should now succeed)..."
echo

# Start server
./server -a localhost:8080 &
SERVER_PID=$!

echo "   Server started (PID: $SERVER_PID)"
echo "   Waiting 15 seconds to see successful connections..."
sleep 15

echo
echo "3. Testing server endpoints..."
echo

# Test server endpoints
echo "   Testing /ping endpoint:"
curl -s http://localhost:8080/ping && echo " ✓ Ping successful" || echo " ✗ Ping failed"

echo "   Testing metrics endpoint:"
curl -s http://localhost:8080/ | head -1 && echo " ✓ Metrics endpoint accessible"

echo
echo "4. Cleanup..."

# Cleanup
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2

echo "   Processes terminated"
echo
echo "=== Test Complete ==="
echo
echo "Expected behavior:"
echo "- Agent should show retry attempts when server is down"
echo "- Agent should succeed once server starts"
echo "- Database operations should use retry logic for connection errors"
echo "- File operations should use retry logic for filesystem errors" 