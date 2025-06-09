#!/bin/bash

echo "=== Testing Different Retry Modes ==="
echo

# Clean up any existing processes
pkill -f "./agent" 2>/dev/null || true
sleep 1

echo "1. Testing DEFAULT mode (fast retry: 2 attempts, 50ms interval)..."
echo "   Starting agent for 3 seconds..."
./agent -a localhost:8080 -p 1 -r 1 &
AGENT_PID=$!
sleep 3
kill $AGENT_PID 2>/dev/null || true
wait $AGENT_PID 2>/dev/null || true
echo "   Default mode test complete"
echo

echo "2. Testing DISABLE_RETRY mode (no retries)..."
echo "   Starting agent for 3 seconds..."
DISABLE_RETRY=true ./agent -a localhost:8080 -p 1 -r 1 &
AGENT_PID=$!
sleep 3
kill $AGENT_PID 2>/dev/null || true
wait $AGENT_PID 2>/dev/null || true
echo "   No retry mode test complete"
echo

echo "3. Testing FULL_RETRY mode (4 attempts, 1s/3s/5s intervals)..."
echo "   Starting agent for 5 seconds..."
ENABLE_FULL_RETRY=true ./agent -a localhost:8080 -p 1 -r 1 &
AGENT_PID=$!
sleep 5
kill $AGENT_PID 2>/dev/null || true
wait $AGENT_PID 2>/dev/null || true
echo "   Full retry mode test complete"
echo

echo "4. Testing with server running..."
echo "   Starting server..."
./server -a localhost:8080 &
SERVER_PID=$!
sleep 2

echo "   Starting agent (should succeed)..."
./agent -a localhost:8080 -p 1 -r 1 &
AGENT_PID=$!
sleep 3

echo "   Stopping processes..."
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
wait $AGENT_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

echo
echo "=== Test Complete ==="
echo
echo "Expected behavior:"
echo "- Default mode: Fast failures with minimal retry delay"
echo "- No retry mode: Immediate failures, no retries"
echo "- Full retry mode: Longer delays between retry attempts"
echo "- With server: Successful metric sending" 