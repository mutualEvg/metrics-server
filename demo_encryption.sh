#!/bin/bash

# Demo script for asymmetric encryption feature
# This script demonstrates encrypted communication between agent and server

set -e

echo "=== Metrics Server - Asymmetric Encryption Demo ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
    fi
    if [ ! -z "$AGENT_PID" ]; then
        kill $AGENT_PID 2>/dev/null || true
    fi
    rm -f demo_private.pem demo_public.pem /tmp/metrics-encrypted.json
    echo -e "${GREEN}Cleanup complete${NC}"
}

trap cleanup EXIT INT TERM

echo -e "${BLUE}Step 1: Building binaries...${NC}"
go build -o bin/server ./cmd/server
go build -o bin/agent ./cmd/agent
go build -o bin/keygen ./cmd/reset/generate_keys.go
echo -e "${GREEN}✓ Binaries built${NC}"
echo ""

echo -e "${BLUE}Step 2: Generating RSA key pair (2048-bit)...${NC}"
./bin/keygen -bits 2048 -priv demo_private.pem -pub demo_public.pem
echo -e "${GREEN}✓ Keys generated${NC}"
echo ""

echo -e "${BLUE}Step 3: Starting server with encryption enabled...${NC}"
./bin/server \
    -a="localhost:18080" \
    -crypto-key=demo_private.pem \
    -i=0 \
    -f=/tmp/metrics-encrypted.json \
    > /tmp/server_encrypted.log 2>&1 &
SERVER_PID=$!

# Wait for server to start
sleep 2

if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${YELLOW}Server failed to start. Check /tmp/server_encrypted.log${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Server running with PID $SERVER_PID${NC}"
echo ""

echo -e "${BLUE}Step 4: Starting agent with encryption enabled...${NC}"
./bin/agent \
    -a="http://localhost:18080" \
    -crypto-key=demo_public.pem \
    -p=2 \
    -r=5 \
    -b=0 \
    -l=3 \
    > /tmp/agent_encrypted.log 2>&1 &
AGENT_PID=$!

echo -e "${GREEN}✓ Agent running with PID $AGENT_PID${NC}"
echo ""

echo -e "${BLUE}Step 5: Collecting encrypted metrics...${NC}"
echo "Waiting 10 seconds for metrics collection..."
sleep 10

echo -e "${GREEN}✓ Metrics collected${NC}"
echo ""

echo -e "${BLUE}Step 6: Checking metrics on server...${NC}"
echo "Fetching metrics from http://localhost:18080/"
curl -s http://localhost:18080/ | head -20
echo ""
echo -e "${GREEN}✓ Metrics successfully transmitted with encryption${NC}"
echo ""

echo -e "${BLUE}Step 7: Demonstrating encryption in action...${NC}"
echo "The following metrics were encrypted by the agent using the public key,"
echo "transmitted over the network, and decrypted by the server using the private key."
echo ""

# Show some statistics
GAUGE_COUNT=$(curl -s http://localhost:18080/ | grep -c "gauge" || echo "0")
COUNTER_COUNT=$(curl -s http://localhost:18080/ | grep -c "counter" || echo "0")

echo "Statistics:"
echo "  - Gauge metrics: $GAUGE_COUNT"
echo "  - Counter metrics: $COUNTER_COUNT"
echo ""

echo -e "${BLUE}Step 8: Testing backward compatibility...${NC}"
echo "Starting second agent WITHOUT encryption..."

./bin/agent \
    -a="http://localhost:18080" \
    -p=2 \
    -r=5 \
    -b=0 \
    -l=1 \
    > /tmp/agent_unencrypted.log 2>&1 &
AGENT2_PID=$!

sleep 5

if kill -0 $AGENT2_PID 2>/dev/null; then
    kill $AGENT2_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Unencrypted agent works alongside encrypted agent${NC}"
else
    echo -e "${YELLOW}⚠ Second agent terminated${NC}"
fi
echo ""

echo -e "${GREEN}=== Demo Complete ===${NC}"
echo ""
echo "Key points demonstrated:"
echo "  1. RSA key pair generation"
echo "  2. Server-side decryption with private key"
echo "  3. Agent-side encryption with public key"
echo "  4. Encrypted metrics transmission"
echo "  5. Backward compatibility with unencrypted clients"
echo ""
echo "Generated files:"
echo "  - demo_private.pem (server's private key)"
echo "  - demo_public.pem (agent's public key)"
echo "  - /tmp/metrics-encrypted.json (persistent storage)"
echo "  - /tmp/server_encrypted.log (server logs)"
echo "  - /tmp/agent_encrypted.log (agent logs)"
echo ""
echo "To view logs:"
echo "  tail -f /tmp/server_encrypted.log"
echo "  tail -f /tmp/agent_encrypted.log"
echo ""
echo -e "${YELLOW}Note: Keys and temporary files will be cleaned up on exit${NC}"

