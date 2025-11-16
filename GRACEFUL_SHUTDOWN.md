# Graceful Shutdown

This document describes the graceful shutdown functionality for both the metrics server and agent.

## Overview

Both the server and agent implement graceful shutdown to ensure:
- **Server**: All in-flight HTTP requests are processed before shutdown
- **Server**: All unsaved data is persisted to storage
- **Agent**: All collected metrics are sent to the server before shutdown
- **Both**: Clean termination without data loss

## Supported Signals

The following POSIX signals trigger graceful shutdown:

- **SIGTERM** (`syscall.SIGTERM`) - Termination signal (default for `kill` command)
- **SIGINT** (`syscall.SIGINT`) - Interrupt signal (Ctrl+C)
- **SIGQUIT** (`syscall.SIGQUIT`) - Quit signal (Ctrl+\)

## Server Shutdown Behavior

When the server receives a shutdown signal, it performs the following steps:

### 1. Signal Reception
```
Shutdown signal received: terminated
```

### 2. HTTP Server Shutdown
- Stops accepting new connections
- Waits for in-flight HTTP requests to complete (up to 30 seconds)
- Returns errors for any requests that don't complete in time

```
Shutting down HTTP server...
HTTP server stopped gracefully
```

### 3. Data Persistence

**For File Storage:**
- Stops periodic saver (if enabled)
- Saves final state to disk
- Ensures all metrics are persisted

```
Stopping periodic saver...
Saving final state...
Final state saved file=/path/to/metrics.json
```

**For Database Storage:**
- Closes database connections properly
- Ensures all transactions are committed

```
Closing database connection...
Database connection closed
```

### 4. Completion
```
Server shutdown complete
```

## Agent Shutdown Behavior

When the agent receives a shutdown signal, it performs the following steps:

### 1. Signal Reception
```
Shutdown signal received: terminated
Stopping agent gracefully...
```

### 2. Metric Collection Stop
- Cancels metric collection goroutines
- Stops collecting new metrics

### 3. Metric Flush
- Waits for final batch of metrics to be sent
- Ensures all collected metrics are transmitted

```
Flushing final metrics...
```

### 4. Worker Pool Shutdown
- Stops worker pool
- Waits for in-flight HTTP requests to complete
- Ensures all metrics are sent to server

```
Stopping worker pool...
Worker pool stopped
```

### 5. Completion
```
Agent shutdown complete
```

## Usage Examples

### Sending Signals

**Using kill command:**
```bash
# Find process ID
ps aux | grep server
# or
ps aux | grep agent

# Send SIGTERM (graceful shutdown)
kill -TERM <pid>
# or
kill <pid>  # SIGTERM is default

# Send SIGINT
kill -INT <pid>

# Send SIGQUIT
kill -QUIT <pid>
```

**Using Ctrl+C (SIGINT):**
```bash
# Start server or agent
./server

# Press Ctrl+C to trigger graceful shutdown
^C
```

**Using Ctrl+\ (SIGQUIT):**
```bash
# Start server or agent
./agent

# Press Ctrl+\ to trigger graceful shutdown
^\
```

### Docker/Container Shutdown

**Docker:**
```bash
# Graceful shutdown (sends SIGTERM)
docker stop <container-name>

# Force shutdown after timeout
docker stop -t 60 <container-name>  # 60 second timeout
```

**Docker Compose:**
```yaml
version: '3.8'
services:
  metrics-server:
    image: metrics-server:latest
    stop_grace_period: 30s  # Wait up to 30s for graceful shutdown
    
  metrics-agent:
    image: metrics-agent:latest
    stop_grace_period: 15s  # Wait up to 15s for graceful shutdown
```

**Kubernetes:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: metrics-server
spec:
  terminationGracePeriodSeconds: 30  # Wait up to 30s for graceful shutdown
  containers:
  - name: server
    image: metrics-server:latest
```

### Systemd Service

**server.service:**
```ini
[Unit]
Description=Metrics Server
After=network.target

[Service]
Type=simple
User=metrics
ExecStart=/usr/local/bin/server -c /etc/metrics/server.json
Restart=on-failure

# Graceful shutdown configuration
TimeoutStopSec=30
KillMode=mixed
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
```

**agent.service:**
```ini
[Unit]
Description=Metrics Agent
After=network.target

[Service]
Type=simple
User=metrics
ExecStart=/usr/local/bin/agent -c /etc/metrics/agent.json
Restart=on-failure

# Graceful shutdown configuration
TimeoutStopSec=15
KillMode=mixed
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
```

## Timeout Configuration

### Server
- **HTTP Shutdown Timeout**: 30 seconds
  - Configurable in code: `context.WithTimeout(context.Background(), 30*time.Second)`
  - After timeout, remaining connections are forcefully closed

### Agent
- **Metric Flush Time**: 2 seconds
  - Time allowed for final metrics to be sent
  - Configurable in code: `time.Sleep(2 * time.Second)`

### Recommendations

**Server:**
- Set container/systemd timeout to at least 35 seconds (30s + 5s buffer)
- For heavy traffic, increase to 60 seconds or more

**Agent:**
- Set container/systemd timeout to at least 10 seconds (2s flush + worker pool shutdown)
- For large batch sizes, increase timeout accordingly

## Testing Graceful Shutdown

### Manual Testing

**Server:**
```bash
# Terminal 1: Start server with file storage
./server -f /tmp/test-metrics.json -i 10

# Terminal 2: Send some metrics
curl -X POST http://localhost:8080/update/ \
  -H "Content-Type: application/json" \
  -d '{"id":"test","type":"gauge","value":123.45}'

# Terminal 1: Press Ctrl+C
# Verify output shows:
# - "Shutdown signal received"
# - "Shutting down HTTP server"
# - "Saving final state"
# - "Server shutdown complete"

# Verify file was saved
cat /tmp/test-metrics.json
```

**Agent:**
```bash
# Terminal 1: Start agent
./agent -a http://localhost:8080 -p 2 -r 5

# Wait for some metrics to be collected

# Terminal 1: Press Ctrl+C
# Verify output shows:
# - "Shutdown signal received"
# - "Stopping agent gracefully"
# - "Flushing final metrics"
# - "Stopping worker pool"
# - "Agent shutdown complete"
```

### Automated Testing

Run the included tests:
```bash
# Run basic signal handling tests
go test ./cmd/server/... -v -run TestSignal
go test ./cmd/agent/... -v -run TestSignal

# Run HTTP shutdown test
go test ./cmd/server/... -v -run TestHTTPServerShutdown
```

## Troubleshooting

### Server doesn't shutdown gracefully

**Symptom:** Server terminates immediately without processing requests

**Possible Causes:**
1. Using `kill -9` (SIGKILL) - cannot be caught
2. Container timeout too short
3. Process manager not sending correct signal

**Solution:**
```bash
# Use SIGTERM instead of SIGKILL
kill -TERM <pid>
# NOT: kill -9 <pid>

# Increase container timeout
docker stop -t 60 <container>

# Check systemd timeout
TimeoutStopSec=30  # In service file
```

### Data not saved on shutdown

**Symptom:** Metrics lost when server shuts down

**Possible Causes:**
1. Using in-memory storage without file backup
2. File permissions issues
3. Shutdown timeout expired

**Solution:**
```bash
# Enable file storage
./server -f /var/lib/metrics/data.json -i 300

# Check file permissions
ls -l /var/lib/metrics/
chmod 644 /var/lib/metrics/data.json

# Increase timeout
docker stop -t 60 metrics-server
```

### Agent metrics not sent

**Symptom:** Final metrics not received by server

**Possible Causes:**
1. Shutdown too fast
2. Network issues
3. Server already shut down

**Solution:**
```bash
# Increase agent flush time (modify code)
time.Sleep(5 * time.Second)  # Instead of 2 seconds

# Ensure server is still running when agent shuts down

# Check logs for transmission errors
./agent 2>&1 | tee agent.log
```

### Forced shutdown after timeout

**Symptom:** "context deadline exceeded" errors

**Cause:** Requests taking longer than shutdown timeout

**Solution:**
```bash
# Increase server shutdown timeout (modify code)
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

# Or wait for long-running requests to complete before shutdown
```

## Best Practices

### 1. Use Appropriate Signals

✅ **Do:**
- Use SIGTERM for graceful shutdown
- Use SIGINT during development (Ctrl+C)
- Configure orchestrators to send SIGTERM

❌ **Don't:**
- Use SIGKILL (`kill -9`) unless absolutely necessary
- Mix signals unnecessarily

### 2. Set Adequate Timeouts

```bash
# Server: Allow time for request processing
TimeoutStopSec=30

# Agent: Allow time for metric transmission
TimeoutStopSec=15
```

### 3. Monitor Shutdown Process

```bash
# Add logging to track shutdown
./server 2>&1 | tee server.log

# Check for graceful shutdown messages
grep "shutdown complete" server.log
```

### 4. Test Shutdown Regularly

```bash
# Include in CI/CD pipeline
make test-shutdown

# Test in staging before production
kubectl delete pod metrics-server-xxx --grace-period=30
```

### 5. Handle Shutdown in Load Balancers

```yaml
# Kubernetes: Remove from service before shutdown
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 5"]
```

## Metrics and Monitoring

### Server Shutdown Metrics

Monitor these during shutdown:
- In-flight request count
- Time to complete shutdown
- Data save success/failure
- Connection closure time

### Agent Shutdown Metrics

Monitor these during shutdown:
- Pending metrics count
- Successful metric transmissions
- Failed transmissions
- Worker pool drain time

## Implementation Details

### Server Shutdown Flow

```go
// 1. Catch signals
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
sig := <-sigChan

// 2. Create shutdown context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 3. Shutdown HTTP server (waits for requests)
server.Shutdown(ctx)

// 4. Stop periodic saver and save final state
if periodicSaver != nil {
    periodicSaver.Stop()
    fileManager.SaveToFile()
}

// 5. Close database connections
if dbStorage != nil {
    dbStorage.Close()
}
```

### Agent Shutdown Flow

```go
// 1. Catch signals
signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
sig := <-signalChan

// 2. Cancel metric collection
cancel()

// 3. Wait for final metrics to be sent
time.Sleep(2 * time.Second)

// 4. Stop worker pool (waits for in-flight requests)
workerPool.Stop()
```

## See Also

- [README.md](README.md) - Main documentation
- [PROFILING.md](PROFILING.md) - Performance profiling
- [ENCRYPTION.md](ENCRYPTION.md) - Encryption setup

## Summary

✅ **Server:**
- Handles SIGTERM, SIGINT, SIGQUIT
- Completes in-flight HTTP requests
- Saves all unsaved data
- 30-second shutdown timeout

✅ **Agent:**
- Handles SIGTERM, SIGINT, SIGQUIT
- Flushes pending metrics
- Waits for metric transmission
- Clean worker pool shutdown

Both server and agent implement proper graceful shutdown to ensure no data loss and clean termination. 🎯

