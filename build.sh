#!/bin/bash

# Build script for metrics-server with version information
# This script demonstrates how to inject build information at compile time

set -e

# Get build information
VERSION=${VERSION:-"v1.0.0"}
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Building metrics-server...${NC}"
echo "Version: $VERSION"
echo "Date: $BUILD_DATE"
echo "Commit: $BUILD_COMMIT"
echo ""

# Build flags
LDFLAGS="-X main.buildVersion=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.buildCommit=${BUILD_COMMIT}"

# Build server
echo -e "${GREEN}Building server...${NC}"
go build -ldflags "${LDFLAGS}" -o bin/server ./cmd/server/
echo "✓ Server built: bin/server"

# Build agent
echo -e "${GREEN}Building agent...${NC}"
go build -ldflags "${LDFLAGS}" -o bin/agent ./cmd/agent/
echo "✓ Agent built: bin/agent"

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo ""
echo "To run the server: ./bin/server"
echo "To run the agent:  ./bin/agent"

