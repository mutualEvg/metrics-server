# Quick Start: Asymmetric Encryption

This guide shows you how to quickly set up encrypted metrics transmission in 5 minutes.

## Prerequisites

- Go 1.19+ installed
- Project built (`go build ./cmd/server && go build ./cmd/agent`)

## Quick Setup (3 Steps)

### Step 1: Generate Keys

```bash
# Build the key generator
go build -o bin/keygen ./cmd/keygen

# Generate a 2048-bit RSA key pair
./bin/keygen -priv server_private.pem -pub agent_public.pem
```

**Output:**
```
Generating 2048-bit RSA key pair...
Private key saved to: server_private.pem
Public key saved to: agent_public.pem

Key pair generated successfully!
```

### Step 2: Start Server with Private Key

```bash
./bin/server -crypto-key=server_private.pem -a=localhost:8080
```

**Expected log:**
```
Build version: N/A
Build date: N/A
Build commit: N/A
[INFO] Asymmetric decryption enabled key_path=server_private.pem
[INFO] Using in-memory storage (no persistence)
Server running at localhost:8080
```

### Step 3: Start Agent with Public Key

```bash
./bin/agent -crypto-key=agent_public.pem -a=http://localhost:8080 -p=2 -r=10
```

**Expected log:**
```
Build version: N/A
Build date: N/A
Build commit: N/A
Asymmetric encryption enabled with public key: agent_public.pem
Agent starting with server=http://localhost:8080, poll=2s, report=10s, batch_size=0, rate_limit=10, crypto=enabled
Public key loaded for encryption
Started worker pool with 10 workers
```

That's it! Your metrics are now encrypted end-to-end.

## Verify It Works

### Check Metrics on Server

```bash
curl http://localhost:8080/
```

You should see metrics like:
```html
<html><body><h1>Metrics</h1><ul>
<li>Alloc (gauge): 123456</li>
<li>TotalAlloc (gauge): 987654</li>
<li>PollCount (counter): 5</li>
...
</ul></body></html>
```

### View Agent Logs

The agent should show successful metric sends:
```
Successfully sent batch of 30 metrics
```

### View Server Logs

The server should show encrypted requests being processed (look for X-Encrypted header in logs if debug enabled).

## Common Configurations

### Minimal Configuration (Default Settings)

```bash
# Server
./bin/server -crypto-key=server_private.pem

# Agent
./bin/agent -crypto-key=agent_public.pem
```

### With Batch Mode (Recommended for Production)

```bash
# Server
./bin/server -crypto-key=server_private.pem -d="postgresql://user:pass@localhost/metrics"

# Agent (send 10 metrics per batch)
./bin/agent -crypto-key=agent_public.pem -a=http://localhost:8080 -b=10
```

### With Hash Signing (Maximum Security)

```bash
# Server
./bin/server -crypto-key=server_private.pem -k=mysecretkey

# Agent
./bin/agent -crypto-key=agent_public.pem -k=mysecretkey -a=http://localhost:8080
```

### With File Storage

```bash
# Server
./bin/server \
  -crypto-key=server_private.pem \
  -f=/var/lib/metrics/data.json \
  -i=300 \
  -r=true

# Agent
./bin/agent -crypto-key=agent_public.pem -a=http://localhost:8080
```

## Environment Variables (Alternative to Flags)

Instead of flags, you can use environment variables:

### Server

```bash
export CRYPTO_KEY=server_private.pem
export ADDRESS=localhost:8080
export DATABASE_DSN=postgresql://user:pass@localhost/metrics

./bin/server
```

### Agent

```bash
export CRYPTO_KEY=agent_public.pem
export ADDRESS=http://localhost:8080
export POLL_INTERVAL=2
export REPORT_INTERVAL=10
export BATCH_SIZE=10

./bin/agent
```

## Testing Your Setup

### 1. Test Key Generation

```bash
go run cmd/keygen/main.go -priv test_priv.pem -pub test_pub.pem
ls -l test_*.pem
```

Should show two files:
- `test_priv.pem` (private key for server)
- `test_pub.pem` (public key for agent)

### 2. Test Encryption/Decryption

```bash
# Run crypto tests
go test -v ./internal/crypto/...
```

### 3. Test Encrypted Communication

```bash
# Run integration tests
go test -v ./cmd/agent/... -run Encrypted
```

### 4. Run Demo Script

```bash
./demo_encryption.sh
```

## Troubleshooting

### Issue: "Failed to load public key"

**Cause:** File not found or wrong path

**Solution:**
```bash
# Check file exists
ls -l agent_public.pem

# Check file permissions
chmod 644 agent_public.pem

# Use absolute path
./bin/agent -crypto-key=/full/path/to/agent_public.pem
```

### Issue: "Failed to decrypt data"

**Cause:** Key mismatch - agent and server using different key pairs

**Solution:**
- Ensure agent uses the public key from the same pair as server's private key
- Regenerate keys and restart both services

### Issue: "No metrics appearing"

**Cause:** Network or configuration issue

**Solution:**
```bash
# Check server is running
curl http://localhost:8080/

# Check agent is connecting
# Look for errors in agent output

# Test without encryption first
./bin/server -a=localhost:8080
./bin/agent -a=http://localhost:8080
```

## Security Best Practices

1. **Never commit private keys to version control**
   ```bash
   # Add to .gitignore
   echo "*.pem" >> .gitignore
   echo "!example*.pem" >> .gitignore
   ```

2. **Restrict private key permissions**
   ```bash
   chmod 600 server_private.pem
   ```

3. **Use different keys per environment**
   ```bash
   ./bin/keygen -priv prod_private.pem -pub prod_public.pem
   ./bin/keygen -priv staging_private.pem -pub staging_public.pem
   ./bin/keygen -priv dev_private.pem -pub dev_public.pem
   ```

4. **Rotate keys regularly**
   ```bash
   # Generate new keys
   ./bin/keygen -priv new_private.pem -pub new_public.pem
   
   # Update server (rolling restart)
   # Update agents one by one
   ```

## Next Steps

- Read [ENCRYPTION.md](ENCRYPTION.md) for detailed documentation
- Explore [demo_encryption.sh](demo_encryption.sh) for a complete example
- Check out performance benchmarks: `go test -bench=. ./internal/crypto/...`
- Learn about combining encryption with other features in the main [README.md](README.md)

## Need Help?

- Check server logs: Look for errors related to "crypto" or "decrypt"
- Check agent logs: Look for errors related to "encrypt" or "public key"
- Run tests: `go test -v ./...` to ensure everything is working
- Review the [ENCRYPTION.md](ENCRYPTION.md) for advanced topics

