# go-musthave-metrics-tpl

Repository template for the "Metrics Collection and Alerting Server" track.

## Features

This metrics server supports both legacy URL-based API and modern JSON API for collecting and retrieving metrics.

### API Endpoints

#### Legacy URL-based API
- `POST /update/{type}/{name}/{value}` - Update a metric
- `GET /value/{type}/{name}` - Get a metric value
- `GET /` - View all metrics in HTML format

#### JSON API
- `POST /update/` - Update a metric using JSON payload
- `POST /value/` - Get a metric value using JSON payload

#### JSON Structure
```json
{
  "id": "metric_name",
  "type": "gauge|counter", 
  "delta": 123,     // for counter metrics
  "value": 123.45   // for gauge metrics
}
```

### Compression Support

The server supports gzip compression for both requests and responses:

- **Request Compression**: Send `Content-Encoding: gzip` header with compressed request body
- **Response Compression**: Send `Accept-Encoding: gzip` header to receive compressed responses
- **Supported Content Types**: `application/json`, `text/html`, `text/plain`

The agent automatically sends compressed JSON data to reduce network traffic.

### File Storage

The server can persist metrics to disk and restore them on startup:

- **Periodic Saving**: Automatically save metrics at configurable intervals
- **Synchronous Saving**: Save immediately on every metric update (when interval = 0)
- **Graceful Shutdown**: Save all data when server receives shutdown signal
- **Restore on Startup**: Optionally load previously saved metrics on server start

#### Storage Configuration

**Environment Variables:**
- `STORE_INTERVAL` - Save interval in seconds (default: 300, 0 = synchronous)
- `FILE_STORAGE_PATH` - Path to storage file (default: `/tmp/metrics-db.json`)
- `RESTORE` - Restore data on startup (default: `true`)

**Command Line Flags:**
- `-i` - Store interval in seconds
- `-f` - File storage path
- `--restore` - Restore previously stored values

**Priority:** Environment variables > Command line flags > Default values

## How to Run Tests Locally

### Prerequisites
Make sure to run this first to ensure all dependencies are properly resolved:
```bash
go mod tidy
```

### Run All Tests
```bash
go test -v ./...
```

### Run Tests for Specific Components
```bash
# Server tests only
go test -v ./cmd/server

# Agent tests only
go test -v ./cmd/agent
```

### Run Specific Tests
```bash
# Run only the UpdateHandler test
go test -v ./cmd/server -run TestUpdateHandler

# Run only the PollMetrics test
go test -v ./cmd/agent -run TestPollMetrics

# Run gzip compression tests
go test -v ./cmd/server -run TestGzip
go test -v ./internal/middleware -run TestGzip

# Run file storage tests
go test -v ./storage -run TestFile
go test -v ./storage -run TestPeriodicSaver
```

### Static Code Analysis
```bash
# Check with go vet
go vet ./...

# Check import formatting
go install golang.org/x/tools/cmd/goimports@v0.20.0
goimports -l .
```

## Troubleshooting

### "no required module provides package" Error
If you encounter an error like:
```
no required module provides package github.com/mutualEvg/metrics-server/internal/models
```

Run the following commands:
```bash
go mod tidy
go mod verify
```

This ensures all internal packages are properly recognized by the Go module system.

## Getting Started

1. Clone the repository to any suitable directory on your computer.
2. In the repository root, run the command `go mod init <name>` (where `<name>` is your GitHub repository address without the `https://` prefix) to create a module.

## Building and Running

### Build the Server
```bash
# From project root
go build -o cmd/server/server ./cmd/server
./cmd/server/server
```

### Build the Agent
```bash
# From project root
go build -o cmd/agent/agent ./cmd/agent
./cmd/agent/agent
```

### Usage Examples

#### Run server with custom storage settings
```bash
# Periodic saving every 60 seconds
./cmd/server/server -i 60 -f /var/lib/metrics.json --restore

# Synchronous saving (save on every update)
./cmd/server/server -i 0 -f /var/lib/metrics.json

# Using environment variables
export STORE_INTERVAL=120
export FILE_STORAGE_PATH=/data/metrics.json
export RESTORE=true
./cmd/server/server
```

#### Example storage file format
```json
{
  "gauges": {
    "Alloc": 1234567,
    "HeapInuse": 2345678,
    "RandomValue": 0.123456
  },
  "counters": {
    "PollCount": 42
  }
}
```

### Configuration

#### Server
- Default address: `localhost:8080`
- Default store interval: 300 seconds
- Default file storage path: `/tmp/metrics-db.json`
- Default restore: `true`

Environment variables:
- `ADDRESS` - Server address
- `STORE_INTERVAL` - Metrics save interval in seconds (0 for synchronous)
- `FILE_STORAGE_PATH` - Path to metrics storage file
- `RESTORE` - Restore metrics on startup (true/false)

Command line flags:
- `-a` - Server address
- `-i` - Store interval in seconds
- `-f` - File storage path
- `--restore` - Restore previously stored values

#### Agent
- Default server address: `http://localhost:8080`
- Default poll interval: 2 seconds
- Default report interval: 10 seconds

Environment variables:
- `ADDRESS` - Server address
- `POLL_INTERVAL` - Metrics polling interval in seconds
- `REPORT_INTERVAL` - Metrics reporting interval in seconds

Command line flags:
- `-a` - Server address
- `-p` - Poll interval in seconds  
- `-r` - Report interval in seconds

## Template Updates

To be able to receive updates for autotests and other parts of the template, run the command:

```
git remote add -m main template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

To update the autotest code, run the command:

```
git fetch template && git checkout template/main .github
```

Then add the received changes to your repository.

## Running Autotests

For successful autotest execution, name branches `iter<number>`, where `<number>` is the increment sequence number. For example, in a branch named `iter4`, autotests for increments one through four will run.

When merging an increment branch into the main `main` branch, all autotests will run.

For more details about local and automatic execution, read the [autotests README](https://github.com/Yandex-Practicum/go-autotests).
