# Asymmetric Encryption Support

This document describes the asymmetric encryption feature for securing metrics data transmission between the agent and server.

## Overview

The metrics service supports RSA asymmetric encryption to protect metrics data in transit. The agent encrypts data using a public key, and the server decrypts it using the corresponding private key.

### Features

- **RSA-OAEP encryption** with SHA-256 hashing
- **Chunked encryption** for large payloads (automatically handles data larger than key size)
- **Backward compatible** - unencrypted requests still work when encryption is enabled
- **Optional** - encryption can be enabled/disabled independently of other features
- **Supports both single and batch metric sending**

## Usage

### 1. Generate RSA Key Pair

First, generate an RSA key pair using the provided utility:

```bash
go run cmd/reset/generate_keys.go -bits 2048 -priv private.pem -pub public.pem
```

Parameters:
- `-bits`: RSA key size in bits (default: 2048, recommended: 2048 or 4096)
- `-priv`: Output path for private key (default: private.pem)
- `-pub`: Output path for public key (default: public.pem)

Alternatively, you can generate keys using OpenSSL:

```bash
# Generate private key
openssl genrsa -out private.pem 2048

# Extract public key
openssl rsa -in private.pem -pubout -out public.pem
```

### 2. Configure the Server

The server requires the **private key** for decryption.

**Using command-line flag:**
```bash
./server -crypto-key=/path/to/private.pem
```

**Using environment variable:**
```bash
export CRYPTO_KEY=/path/to/private.pem
./server
```

### 3. Configure the Agent

The agent requires the **public key** for encryption.

**Using command-line flag:**
```bash
./agent -crypto-key=/path/to/public.pem -a=http://localhost:8080
```

**Using environment variable:**
```bash
export CRYPTO_KEY=/path/to/public.pem
export ADDRESS=http://localhost:8080
./agent
```

## Security Considerations

### Key Size

- **2048-bit keys**: Good balance between security and performance
- **4096-bit keys**: Higher security but slower encryption/decryption
- Minimum recommended: 2048 bits

### Key Management

1. **Keep private keys secure**: Never share or commit private keys to version control
2. **Restrict file permissions**: 
   ```bash
   chmod 600 private.pem
   chmod 644 public.pem
   ```
3. **Rotate keys regularly**: Generate new key pairs periodically
4. **Use different keys for different environments**: Production, staging, and development should have separate keys

### Network Security

- Encryption protects data in transit but doesn't replace other security measures
- Consider using HTTPS in addition to payload encryption for full TLS protection
- Implement proper authentication and authorization mechanisms

## How It Works

### Encryption Flow (Agent → Server)

1. Agent reads metrics data
2. Data is serialized to JSON
3. JSON is compressed with gzip
4. Compressed data is encrypted with RSA public key (chunked for large payloads)
5. Encrypted data is sent to server with `X-Encrypted: true` header
6. Hash is computed on compressed (pre-encryption) data if hash signing is enabled

### Decryption Flow (Server)

1. Server receives encrypted request with `X-Encrypted: true` header
2. Decryption middleware decrypts the body using RSA private key
3. Decrypted compressed data is decompressed by gzip middleware
4. Hash verification happens on decompressed data if hash verification is enabled
5. JSON is parsed and metrics are stored

### Chunked Encryption

RSA encryption has a size limit based on key size:
- 2048-bit key: ~190 bytes per chunk (with SHA-256 padding)
- 4096-bit key: ~446 bytes per chunk

For data larger than the chunk size:
1. Data is split into chunks
2. Each chunk is encrypted separately
3. Chunks are concatenated with length prefixes
4. Server decrypts chunks and reassembles data

## Configuration Examples

### Example 1: Encryption Only

```bash
# Server
./server -crypto-key=private.pem

# Agent
./agent -crypto-key=public.pem -a=http://localhost:8080
```

### Example 2: Encryption + Hash Signing

```bash
# Server
./server -crypto-key=private.pem -k=mysecretkey

# Agent
./agent -crypto-key=public.pem -k=mysecretkey -a=http://localhost:8080
```

### Example 3: Full Security Stack

```bash
# Server with database, encryption, and hash verification
./server \
  -crypto-key=private.pem \
  -k=mysecretkey \
  -d="postgresql://user:pass@localhost/metrics?sslmode=require"

# Agent with encryption, hash signing, and batch mode
./agent \
  -crypto-key=public.pem \
  -k=mysecretkey \
  -a=https://server.example.com \
  -b=10 \
  -l=5
```

## Performance Impact

### Encryption Overhead

- **Single metric**: ~5-10ms additional latency per request
- **Batch metrics (10 items)**: ~10-20ms additional latency per batch
- **CPU usage**: Moderate increase (~10-15%) on both agent and server

### Recommendations for High-Throughput Systems

1. Use **batch mode** (`-b` flag) to reduce encryption overhead per metric
2. Use **2048-bit keys** for better performance
3. Increase **rate limit** (`-l` flag) to handle concurrent requests efficiently
4. Monitor CPU usage and adjust accordingly

## Testing

### Unit Tests

Run crypto package tests:
```bash
go test -v ./internal/crypto/...
```

### Integration Tests

Run encrypted communication tests:
```bash
go test -v ./cmd/agent/... -run Crypto
go test -v ./cmd/agent/... -run Encrypted
```

### Manual Testing

1. Generate test keys:
   ```bash
   go run cmd/reset/generate_keys.go -priv test_private.pem -pub test_public.pem
   ```

2. Start server with encryption:
   ```bash
   ./server -crypto-key=test_private.pem
   ```

3. Start agent with encryption:
   ```bash
   ./agent -crypto-key=test_public.pem -p=2 -r=5
   ```

4. Verify encrypted metrics are received:
   ```bash
   curl http://localhost:8080/
   ```

## Troubleshooting

### Common Issues

**Error: "Failed to load public/private key"**
- Check file path is correct
- Verify file permissions (readable by the process)
- Ensure key format is PEM

**Error: "Failed to decrypt data"**
- Ensure agent and server are using matching key pair
- Check that encryption is enabled on both sides
- Verify key files are not corrupted

**Error: "Invalid chunked data"**
- May indicate key mismatch or corrupted data
- Check network stability
- Verify both sides are using compatible crypto versions

**Performance Degradation**
- Use batch mode to reduce per-metric overhead
- Consider using 2048-bit keys instead of 4096-bit
- Increase worker pool size (`-l` flag)
- Check CPU and memory usage

### Debug Mode

Enable detailed logging:
```bash
# Server
LOG_LEVEL=debug ./server -crypto-key=private.pem

# Agent  
./agent -crypto-key=public.pem -a=http://localhost:8080
```

## Migration Guide

### Enabling Encryption on Existing Setup

1. **Generate keys** without stopping services
2. **Update server** first with crypto-key flag (backward compatible)
3. **Update agents** one by one with crypto-key flag
4. All agents can be updated gradually - both encrypted and unencrypted work simultaneously

### Disabling Encryption

Simply remove the `-crypto-key` flag or unset `CRYPTO_KEY` environment variable.

## API Impact

### Request Headers

When encryption is enabled, the agent adds:
```
X-Encrypted: true
```

This header tells the server to decrypt the request body.

### Response Format

Responses remain unchanged - encryption only affects request bodies from agent to server.

## Compatibility

- **Go version**: 1.19+
- **Key formats**: PEM (PKCS#1 and PKCS#8 for private keys, PKIX for public keys)
- **Encryption algorithm**: RSA-OAEP with SHA-256
- **Compatible with**: All existing features (hash signing, compression, batch mode, database storage, etc.)

## References

- [RFC 8017 - PKCS #1: RSA Cryptography Specifications Version 2.2](https://tools.ietf.org/html/rfc8017)
- [Go crypto/rsa package](https://pkg.go.dev/crypto/rsa)
- [OAEP (Optimal Asymmetric Encryption Padding)](https://en.wikipedia.org/wiki/Optimal_asymmetric_encryption_padding)

