# JSON Configuration File Support

This document describes how to configure the metrics server and agent using JSON configuration files.

## Overview

Both the server and agent support configuration via JSON files, which can be more convenient than using multiple command-line flags. The configuration file path is specified using the `-c` or `-config` flag, or via the `CONFIG` environment variable.

### Configuration Priority

Values are resolved in the following priority order (highest to lowest):

1. **Environment variables** (highest priority)
2. **Command-line flags**
3. **JSON configuration file**
4. **Default values** (lowest priority)

This means environment variables override flags, flags override JSON config, and JSON config overrides defaults.

## Server Configuration

### File Format

Create a JSON file with the following structure:

```json
{
    "address": "localhost:8080",
    "restore": true,
    "store_interval": "300s",
    "store_file": "/path/to/file.db",
    "database_dsn": "postgresql://user:pass@localhost/metrics",
    "crypto_key": "/path/to/private.pem"
}
```

### Field Descriptions

| Field | Type | Description | Flag Equivalent |
|-------|------|-------------|-----------------|
| `address` | string | Server listen address | `-a` or `ADDRESS` env |
| `restore` | boolean | Restore metrics on startup | `-r` or `RESTORE` env |
| `store_interval` | string | Interval to save metrics to file (e.g., "300s", "5m") | `-i` or `STORE_INTERVAL` env |
| `store_file` | string | Path to metrics storage file | `-f` or `FILE_STORAGE_PATH` env |
| `database_dsn` | string | PostgreSQL connection string | `-d` or `DATABASE_DSN` env |
| `crypto_key` | string | Path to private key file for decryption | `-crypto-key` or `CRYPTO_KEY` env |

### Usage Examples

**Using `-c` flag:**
```bash
./server -c config/server.json
```

**Using `-config` flag:**
```bash
./server -config config/server.json
```

**Using `CONFIG` environment variable:**
```bash
export CONFIG=config/server.json
./server
```

**Combining with other flags (flags take priority):**
```bash
./server -c config/server.json -a localhost:9090
# Server will listen on localhost:9090 (flag overrides JSON)
```

**With environment variables (env takes highest priority):**
```bash
export ADDRESS=localhost:7070
./server -c config/server.json -a localhost:9090
# Server will listen on localhost:7070 (env overrides both flag and JSON)
```

### Example Server Config

**config/server.json:**
```json
{
    "address": "localhost:8080",
    "restore": true,
    "store_interval": "300s",
    "store_file": "/var/lib/metrics/data.json",
    "database_dsn": "",
    "crypto_key": "/etc/metrics/private.pem"
}
```

**Running with this config:**
```bash
./server -c config/server.json
```

## Agent Configuration

### File Format

Create a JSON file with the following structure:

```json
{
    "address": "localhost:8080",
    "report_interval": "10s",
    "poll_interval": "2s",
    "crypto_key": "/path/to/public.pem"
}
```

### Field Descriptions

| Field | Type | Description | Flag Equivalent |
|-------|------|-------------|-----------------|
| `address` | string | Server address to send metrics to | `-a` or `ADDRESS` env |
| `report_interval` | string | How often to send metrics (e.g., "10s", "1m") | `-r` or `REPORT_INTERVAL` env |
| `poll_interval` | string | How often to collect metrics (e.g., "2s") | `-p` or `POLL_INTERVAL` env |
| `crypto_key` | string | Path to public key file for encryption | `-crypto-key` or `CRYPTO_KEY` env |

### Usage Examples

**Using `-c` flag:**
```bash
./agent -c config/agent.json
```

**Using `-config` flag:**
```bash
./agent -config config/agent.json
```

**Using `CONFIG` environment variable:**
```bash
export CONFIG=config/agent.json
./agent
```

**Combining with other flags:**
```bash
./agent -c config/agent.json -b 10 -l 5
# Batch size and rate limit from flags, other settings from JSON
```

### Example Agent Config

**config/agent.json:**
```json
{
    "address": "metrics.example.com:8080",
    "report_interval": "30s",
    "poll_interval": "5s",
    "crypto_key": "/etc/metrics/public.pem"
}
```

**Running with this config:**
```bash
./agent -c config/agent.json
```

## Time Duration Format

Time durations in JSON config files use Go's duration format:

- `"1s"` - 1 second
- `"30s"` - 30 seconds
- `"1m"` - 1 minute
- `"5m"` - 5 minutes
- `"1h"` - 1 hour
- `"300s"` - 300 seconds (5 minutes)

Valid units: `ns`, `us`/`µs`, `ms`, `s`, `m`, `h`

## Common Scenarios

### Development Environment

**server.json:**
```json
{
    "address": "localhost:8080",
    "restore": false,
    "store_interval": "10s",
    "store_file": "/tmp/dev-metrics.json"
}
```

**agent.json:**
```json
{
    "address": "localhost:8080",
    "report_interval": "5s",
    "poll_interval": "1s"
}
```

### Production Environment

**server.json:**
```json
{
    "address": "0.0.0.0:8080",
    "restore": true,
    "store_interval": "300s",
    "store_file": "/var/lib/metrics/prod.json",
    "database_dsn": "postgresql://metrics_user:password@db.internal:5432/metrics?sslmode=require",
    "crypto_key": "/etc/metrics/keys/private.pem"
}
```

**agent.json:**
```json
{
    "address": "metrics.internal:8080",
    "report_interval": "10s",
    "poll_interval": "2s",
    "crypto_key": "/etc/metrics/keys/public.pem"
}
```

### Staging with Overrides

Use JSON for base config, override with environment variables:

**staging.json:**
```json
{
    "address": "staging-metrics:8080",
    "report_interval": "15s",
    "poll_interval": "3s"
}
```

**Override address via environment:**
```bash
export ADDRESS=localhost:9090
./agent -c config/staging.json
# Connects to localhost:9090 (env override)
```

## Configuration Management

### Version Control

It's safe to commit example configs to version control:

```bash
# Add example configs
git add config/*.json.example

# Don't commit actual configs with secrets
echo "config/*.json" >> .gitignore
echo "!config/*.json.example" >> .gitignore
```

### Creating Configs from Examples

```bash
# Server
cp config/server_config.json.example config/server.json
vim config/server.json  # Edit with actual values

# Agent
cp config/agent_config.json.example config/agent.json
vim config/agent.json  # Edit with actual values
```

### Docker/Container Usage

**Dockerfile approach:**
```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM debian:bookworm-slim
COPY --from=builder /app/server /usr/local/bin/
COPY config/server.json /etc/metrics/config.json
CMD ["server", "-c", "/etc/metrics/config.json"]
```

**Docker Compose with config file:**
```yaml
version: '3.8'
services:
  metrics-server:
    image: metrics-server:latest
    volumes:
      - ./config/server.json:/etc/metrics/config.json:ro
      - ./keys/private.pem:/etc/metrics/private.pem:ro
    command: ["-c", "/etc/metrics/config.json"]
    
  metrics-agent:
    image: metrics-agent:latest
    volumes:
      - ./config/agent.json:/etc/metrics/config.json:ro
      - ./keys/public.pem:/etc/metrics/public.pem:ro
    command: ["-c", "/etc/metrics/config.json"]
```

## Validation

The configuration is validated on startup:

- **Invalid JSON:** Error message and server/agent won't start
- **Invalid durations:** Error message and fallback to defaults
- **Missing required fields:** Uses defaults
- **Invalid file path:** Warning logged, continues without config file

## Troubleshooting

### Config file not found

```
Warning: Failed to load config file config.json: open config.json: no such file or directory
```

**Solution:** Check the file path is correct, use absolute path if needed:
```bash
./server -c /full/path/to/config.json
```

### Invalid JSON syntax

```
Warning: Failed to load config file config.json: invalid character '}' looking for beginning of object key string
```

**Solution:** Validate JSON syntax:
```bash
python3 -m json.tool config.json
# or
jq . config.json
```

### Duration parsing errors

```
Invalid report_interval in config file: time: invalid duration "10"
```

**Solution:** Add time unit to duration:
```json
{
    "report_interval": "10s"  // ✓ Correct
    // "report_interval": "10"   // ✗ Wrong
}
```

### Priority confusion

If a setting isn't being applied from JSON:

1. Check environment variables: `env | grep ADDRESS`
2. Check if flag was used: review command line
3. Remember priority: env > flag > json > default

## Testing Config Files

### Dry Run

```bash
# Server
./server -c config/server.json &
SERVER_PID=$!
sleep 2
kill $SERVER_PID

# Check logs for "Loaded configuration from"
```

### Verify Settings

```bash
# Start server with config
./server -c config/server.json 2>&1 | grep "configuration"

# Should show:
# Loaded configuration from config/server.json
```

## Migration Guide

### From Flags to JSON Config

**Before:**
```bash
./server -a localhost:8080 -d "postgresql://..." -crypto-key /path/to/key.pem -i 300 -r
```

**After:**

Create `config.json`:
```json
{
    "address": "localhost:8080",
    "database_dsn": "postgresql://...",
    "crypto_key": "/path/to/key.pem",
    "store_interval": "300s",
    "restore": true
}
```

Run:
```bash
./server -c config.json
```

### From Environment Variables to JSON Config

**Before:**
```bash
export ADDRESS=localhost:8080
export DATABASE_DSN="postgresql://..."
export CRYPTO_KEY=/path/to/key.pem
./server
```

**After:**

Same JSON as above, plus keep ability to override:
```bash
# Base config in JSON, override address via env
export ADDRESS=localhost:9090
./server -c config.json
```

## Best Practices

1. **Use JSON for static configuration** (addresses, paths, intervals)
2. **Use environment variables for secrets** in production (database passwords, API keys)
3. **Use flags for temporary overrides** during debugging
4. **Version control examples**, not actual configs with secrets
5. **Validate JSON** before deployment
6. **Document your config** with comments in `.example` files (JSON doesn't support comments, but examples can include them in documentation)

## See Also

- [README.md](README.md) - Main documentation
- [ENCRYPTION.md](ENCRYPTION.md) - Encryption setup
- [config/server_config.json.example](config/server_config.json.example) - Server config example
- [config/agent_config.json.example](config/agent_config.json.example) - Agent config example

