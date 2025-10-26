# Build Information

This document describes the build information feature added in Increment 23.

## Overview

Both the `server` and `agent` applications display build information at startup:
- Build version
- Build date
- Build commit hash

## Default Behavior

When built normally (without ldflags), the applications display "N/A" for all build information:

```bash
go build -o server ./cmd/server/
./server
```

Output:
```
Build version: N/A
Build date: N/A
Build commit: N/A
```

## Setting Build Information

Build information can be set at compile time using Go's `-ldflags` flag:

### Manual Build

```bash
go build -ldflags "\
  -X main.buildVersion=v1.0.0 \
  -X main.buildDate=2025-10-26_18:00:00 \
  -X main.buildCommit=abc1234" \
  -o server ./cmd/server/
```

### Using the Build Script

A convenient build script is provided that automatically captures build information:

```bash
./build.sh
```

This script:
1. Reads version from `VERSION` environment variable (defaults to `v1.0.0`)
2. Generates build date as current UTC time
3. Captures git commit hash (short form)
4. Builds both server and agent with this information

**Custom version:**
```bash
VERSION=v2.1.0 ./build.sh
```

## Build Variables

The following global variables are defined in both applications:

### Server (`cmd/server/main.go`)
```go
var (
    buildVersion string = "N/A"
    buildDate    string = "N/A"
    buildCommit  string = "N/A"
)
```

### Agent (`cmd/agent/main.go`)
```go
var (
    buildVersion string = "N/A"
    buildDate    string = "N/A"
    buildCommit  string = "N/A"
)
```

## Output Format

Both applications print build information to stdout at startup:

```
Build version: v1.0.0
Build date: 2025-10-26_22:42:31
Build commit: 057690c
```

## Integration Examples

### CI/CD Pipeline

```bash
# In your CI/CD pipeline
VERSION=$(git describe --tags --always)
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_COMMIT=$(git rev-parse --short HEAD)

go build -ldflags "\
  -X main.buildVersion=${VERSION} \
  -X main.buildDate=${BUILD_DATE} \
  -X main.buildCommit=${BUILD_COMMIT}" \
  -o artifacts/server ./cmd/server/

go build -ldflags "\
  -X main.buildVersion=${VERSION} \
  -X main.buildDate=${BUILD_DATE} \
  -X main.buildCommit=${BUILD_COMMIT}" \
  -o artifacts/agent ./cmd/agent/
```

### GitHub Actions

```yaml
- name: Build with version info
  run: |
    VERSION=$(git describe --tags --always)
    BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')
    BUILD_COMMIT=${{ github.sha }}
    
    go build -ldflags "\
      -X main.buildVersion=${VERSION} \
      -X main.buildDate=${BUILD_DATE} \
      -X main.buildCommit=${BUILD_COMMIT}" \
      -o server ./cmd/server/
```

### Docker Multi-Stage Build

```dockerfile
FROM golang:1.21 AS builder

WORKDIR /app
COPY . .

ARG VERSION=v1.0.0
ARG BUILD_DATE
ARG BUILD_COMMIT

RUN go build -ldflags "\
    -X main.buildVersion=${VERSION} \
    -X main.buildDate=${BUILD_DATE} \
    -X main.buildCommit=${BUILD_COMMIT}" \
    -o server ./cmd/server/

FROM alpine:latest
COPY --from=builder /app/server /server
ENTRYPOINT ["/server"]
```

## Makefile Integration

```makefile
VERSION ?= $(shell git describe --tags --always)
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_COMMIT := $(shell git rev-parse --short HEAD)

LDFLAGS := -X main.buildVersion=$(VERSION) \
           -X main.buildDate=$(BUILD_DATE) \
           -X main.buildCommit=$(BUILD_COMMIT)

.PHONY: build
build:
	@echo "Building with version $(VERSION)"
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server/
	go build -ldflags "$(LDFLAGS)" -o bin/agent ./cmd/agent/

.PHONY: release
release:
	@echo "Building release $(VERSION)"
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/server-linux-amd64 ./cmd/server/
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/server-darwin-amd64 ./cmd/server/
```

## Verification

To verify build information is correctly set:

```bash
./build.sh
./bin/server 2>&1 | head -3
./bin/agent 2>&1 | head -3
```

Expected output format:
```
Build version: v1.0.0
Build date: 2025-10-26_22:42:31
Build commit: 057690c
```

## Notes

- Build information is displayed to **stdout**, not stderr
- The information is printed **before** any logging initialization
- Default values are "N/A" to clearly indicate when build info wasn't provided
- The `printBuildInfo()` function is called at the very start of `main()`
- Build variables must be exported (start with uppercase) to be accessible from ldflags

## Testing

Build information does not affect functionality and is purely informational. All existing tests continue to pass regardless of whether build information is set.

```bash
# Tests work with default "N/A" values
go test ./cmd/server/ ./cmd/agent/ -v

# Tests work with custom build info
go build -ldflags "-X main.buildVersion=test" ./cmd/server/
go test ./cmd/server/ -v
```

