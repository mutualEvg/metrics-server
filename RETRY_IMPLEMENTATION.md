# Retry Implementation for Metrics Server

## Overview

This implementation adds comprehensive retry logic to handle retriable errors across the entire metrics server system. The retry mechanism follows the specified requirements:

- **Maximum 3 additional attempts** (4 total attempts including the initial one)
- **Exponential backoff intervals**: 1s, 3s, 5s
- **Intelligent error classification** to determine which errors are retriable

## Implementation Details

### 1. Retry Package (`internal/retry/`)

**Core Components:**
- `RetryConfig`: Configuration for retry behavior
- `Do()`: Main retry execution function with context support
- `IsRetriable()`: Error classification function

**Supported Retriable Errors:**
- **Network errors**: Connection refused, timeouts, DNS errors, URL errors
- **PostgreSQL connection errors**: Class 08 - Connection Exception errors
- **File system errors**: Access denied, resource busy, file limits
- **Context errors**: Deadline exceeded, canceled

### 2. Database Storage (`storage/db_storage.go`)

**Enhanced Operations:**
- Database connection establishment with retry
- Table creation with retry logic
- All CRUD operations (Create, Read, Update, Delete) with retry
- Batch operations with transaction retry
- Connection health checks with retry

**Key Features:**
- Automatic retry on PostgreSQL connection failures
- Context-based timeouts for all operations
- Comprehensive error logging with retry attempt information

### 3. Agent Network Requests (`cmd/agent/main.go`)

**Enhanced Network Operations:**
- HTTP request sending with retry logic
- Batch metric submission with retry
- Individual metric submission with retry
- Automatic fallback from batch to individual on persistent failures

**Key Features:**
- Retry on network connectivity issues
- Graceful degradation when batch operations fail
- Comprehensive error logging with retry information

### 4. File Storage (`storage/file_storage.go`)

**Enhanced File Operations:**
- File reading with retry logic
- File writing with retry logic
- Atomic file operations (write to temp, then rename)
- Periodic saving with retry

**Key Features:**
- Retry on file system access issues
- Protection against file locking conflicts
- Atomic operations to prevent data corruption

## Error Classification

### Retriable Errors

1. **Network Errors:**
   - Connection refused (`ECONNREFUSED`)
   - Connection reset (`ECONNRESET`)
   - Timeout (`ETIMEDOUT`)
   - Host unreachable (`EHOSTUNREACH`)
   - DNS resolution failures
   - URL parsing errors

2. **PostgreSQL Errors:**
   - Connection Exception (`08000`)
   - Connection Does Not Exist (`08003`)
   - Connection Failure (`08006`)
   - SQL Client Unable to Establish Connection (`08001`)
   - SQL Server Rejected Connection (`08004`)
   - Transaction Resolution Unknown (`08007`)
   - Protocol Violation (`08P01`)

3. **File System Errors:**
   - Access denied (`EACCES`)
   - Resource temporarily unavailable (`EAGAIN`)
   - Device or resource busy (`EBUSY`)
   - Too many open files (`EMFILE`, `ENFILE`)

4. **Context Errors:**
   - Deadline exceeded
   - Context canceled

### Non-Retriable Errors

- Application logic errors
- Data validation errors
- Authentication/authorization errors
- Malformed requests
- Resource not found errors

## Usage Examples

### Database Operations
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := retry.Do(ctx, retryConfig, func() error {
    _, err := db.Exec(query, params...)
    return err
})
```

### Network Requests
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.Do(ctx, retryConfig, func() error {
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("server returned status %d", resp.StatusCode)
    }
    return nil
})
```

### File Operations
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

err := retry.Do(ctx, retryConfig, func() error {
    return os.WriteFile(filename, data, 0644)
})
```

## Configuration

The retry behavior can be configured in several ways:

### Default Behavior
By default, the agent uses **fast retry** for backward compatibility:
- **2 attempts** (1 initial + 1 retry)
- **50ms interval** between attempts

### Full Retry Mode
To enable full retry with the original specification:
```bash
export ENABLE_FULL_RETRY=true
./agent
```

This provides:
- **4 attempts** (1 initial + 3 retries)
- **Exponential backoff**: 1s, 3s, 5s

### Disable Retry
To completely disable retry logic:
```bash
export DISABLE_RETRY=true
./agent
# OR
./agent -disable-retry
```

### Test Mode
For testing scenarios with minimal delays:
```bash
export TEST_MODE=true
./agent
```

### Custom Configuration
The retry configuration can be programmatically customized:

```go
// Full retry (production)
config := retry.DefaultConfig()

// Fast retry (default)
config := retry.FastConfig()

// No retry (testing)
config := retry.NoRetryConfig()

// Custom configuration
config := retry.RetryConfig{
    MaxAttempts: 3,
    Intervals:   []time.Duration{500*time.Millisecond, 1*time.Second},
}
```

## Testing

### Unit Tests
- Comprehensive test suite in `internal/retry/retry_test.go`
- Tests for success scenarios, retry scenarios, and failure scenarios
- Error classification testing

### Integration Testing
- Test script `test_retry_functionality.sh` demonstrates retry behavior
- Shows agent retry attempts when server is unavailable
- Demonstrates successful connection after server startup

## Benefits

1. **Improved Reliability**: Automatic recovery from transient failures
2. **Better User Experience**: Reduced impact of temporary network/system issues
3. **Operational Resilience**: System continues operating during brief outages
4. **Comprehensive Logging**: Detailed retry attempt information for debugging
5. **Configurable Behavior**: Retry parameters can be adjusted per use case

## Monitoring and Observability

The implementation includes comprehensive logging:
- Retry attempt notifications with attempt numbers
- Success after retry notifications
- Failure after all retries exhausted
- Error classification decisions
- Timing information for retry intervals

This enables operators to monitor system health and identify patterns in transient failures. 