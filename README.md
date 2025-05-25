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

### Configuration

#### Server
- Default address: `localhost:8080`
- Set via `ADDRESS` environment variable

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
