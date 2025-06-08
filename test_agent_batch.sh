#!/bin/bash

echo "Testing Agent Batch Functionality..."

# Build both server and agent
export PATH="/usr/local/go/bin:$PATH"
go build -o server cmd/server/main.go
go build -o cmd/agent/agent cmd/agent/main.go

echo ""
echo "=== Test 1: Agent with batch processing (default batch size 10) ==="
echo "Starting server..."
./server -a "localhost:8086" &
SERVER_PID=$!
sleep 2

echo "Starting agent with batch processing..."
./cmd/agent/agent -a "localhost:8086" -r 3 -p 1 -b 10 &
AGENT_PID=$!
sleep 5

echo "Checking metrics on server..."
curl -s http://localhost:8086/value/gauge/Alloc | head -1
echo ""
curl -s http://localhost:8086/value/counter/PollCount | head -1
echo ""

echo "Stopping agent and server..."
kill $AGENT_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null
wait $AGENT_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo ""
echo "=== Test 2: Agent with individual sending (batch disabled) ==="
echo "Starting server..."
./server -a "localhost:8087" &
SERVER_PID=$!
sleep 2

echo "Starting agent with individual sending..."
./cmd/agent/agent -a "localhost:8087" -r 3 -p 1 -b 0 &
AGENT_PID=$!
sleep 5

echo "Checking metrics on server..."
curl -s http://localhost:8087/value/gauge/Alloc | head -1
echo ""
curl -s http://localhost:8087/value/counter/PollCount | head -1
echo ""

echo "Stopping agent and server..."
kill $AGENT_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null
wait $AGENT_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo ""
echo "=== Test 3: Agent with environment variable configuration ==="
echo "Starting server..."
./server -a "localhost:8088" &
SERVER_PID=$!
sleep 2

echo "Starting agent with env vars..."
BATCH_SIZE=5 REPORT_INTERVAL=2 ./cmd/agent/agent -a "localhost:8088" &
AGENT_PID=$!
sleep 4

echo "Checking metrics on server..."
curl -s http://localhost:8088/value/gauge/Alloc | head -1
echo ""

echo "Stopping agent and server..."
kill $AGENT_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null
wait $AGENT_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo ""
echo "Agent batch testing completed!" 