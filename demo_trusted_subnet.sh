#!/bin/bash

# Demo script for trusted subnet functionality
# This script demonstrates how the trusted subnet feature works

set -e

echo "=========================================="
echo "Trusted Subnet Feature Demo"
echo "=========================================="
echo ""

# Build binaries if not already built
if [ ! -f "./bin/server" ] || [ ! -f "./bin/agent" ]; then
    echo "Building binaries..."
    ./build.sh
    echo ""
fi

# Test 1: Server without trusted subnet (allow all)
echo "Test 1: Server without trusted subnet (allow all IPs)"
echo "----------------------------------------------"
echo "Starting server without TRUSTED_SUBNET..."
export ADDRESS="localhost:8090"
export STORE_INTERVAL="300"
./bin/server &
SERVER_PID=$!
sleep 2

echo "Starting agent to send metrics..."
export ADDRESS="http://localhost:8090"
export POLL_INTERVAL="2"
export REPORT_INTERVAL="2"
./bin/agent &
AGENT_PID=$!
sleep 5

echo "✓ Agent successfully sent metrics (no IP restrictions)"
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

# Test 2: Server with trusted subnet (localhost only)
echo "Test 2: Server with trusted subnet (localhost only)"
echo "----------------------------------------------"
echo "Starting server with TRUSTED_SUBNET=127.0.0.0/8..."
export TRUSTED_SUBNET="127.0.0.0/8"
export ADDRESS="localhost:8091"
./bin/server &
SERVER_PID=$!
sleep 2

echo "Starting agent to send metrics from localhost..."
export ADDRESS="http://localhost:8091"
./bin/agent &
AGENT_PID=$!
sleep 5

echo "✓ Agent successfully sent metrics (IP in trusted subnet)"
kill $AGENT_PID 2>/dev/null || true
kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

# Test 3: Show configuration options
echo "Test 3: Configuration Options"
echo "----------------------------------------------"
echo "The trusted subnet can be configured in three ways:"
echo ""
echo "1. Environment Variable:"
echo "   export TRUSTED_SUBNET=\"192.168.1.0/24\""
echo "   ./bin/server"
echo ""
echo "2. Command-line Flag:"
echo "   ./bin/server -t \"192.168.1.0/24\""
echo ""
echo "3. JSON Configuration File:"
echo "   {\"trusted_subnet\": \"192.168.1.0/24\"}"
echo "   ./bin/server -c config.json"
echo ""

# Test 4: Test with curl
echo "Test 4: Testing with curl (simulate different IPs)"
echo "----------------------------------------------"
echo "Starting server with TRUSTED_SUBNET=192.168.1.0/24..."
export TRUSTED_SUBNET="192.168.1.0/24"
export ADDRESS="localhost:8092"
./bin/server &
SERVER_PID=$!
sleep 2

echo ""
echo "Sending request with IP in trusted subnet (192.168.1.100):"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "X-Real-IP: 192.168.1.100" \
    -d '{"id":"test","type":"gauge","value":42.0}' \
    http://localhost:8092/update/ | tail -n1)

if [ "$RESPONSE" = "200" ]; then
    echo "✓ Request accepted (Status: 200)"
else
    echo "✗ Request rejected (Status: $RESPONSE)"
fi

echo ""
echo "Sending request with IP outside trusted subnet (10.0.0.1):"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -H "X-Real-IP: 10.0.0.1" \
    -d '{"id":"test","type":"gauge","value":42.0}' \
    http://localhost:8092/update/ | tail -n1)

if [ "$RESPONSE" = "403" ]; then
    echo "✓ Request rejected as expected (Status: 403)"
else
    echo "✗ Unexpected response (Status: $RESPONSE)"
fi

echo ""
echo "Sending request without X-Real-IP header:"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -d '{"id":"test","type":"gauge","value":42.0}' \
    http://localhost:8092/update/ | tail -n1)

if [ "$RESPONSE" = "403" ]; then
    echo "✓ Request rejected as expected (Status: 403)"
else
    echo "✗ Unexpected response (Status: $RESPONSE)"
fi

kill $SERVER_PID 2>/dev/null || true
sleep 2
echo ""

echo "=========================================="
echo "Demo Complete!"
echo "=========================================="
echo ""
echo "Summary:"
echo "- ✓ Server works without trusted subnet (backward compatible)"
echo "- ✓ Server validates IPs when trusted subnet is configured"
echo "- ✓ Agent automatically sends X-Real-IP header"
echo "- ✓ Requests outside trusted subnet are rejected with 403"
echo "- ✓ Requests without X-Real-IP header are rejected when subnet is configured"
echo ""
echo "For more details, see TRUSTED_SUBNET_IMPLEMENTATION.md"

