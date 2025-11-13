# Code Review Fixes

## Overview

This document summarizes the improvements made to the codebase based on the code review feedback.

## Review Feedback Points Addressed

### 1. Separation of Concerns ✅

**Issue**: The crypto package mixed file operations with cryptography logic

**Solution**:
- Split `internal/crypto/crypto.go` into two files:
  - `crypto.go` - Pure cryptographic operations (encryption, decryption, key generation)
  - `keys.go` - File I/O operations (loading/saving keys from/to files)
- Each file now has a single, clear responsibility

**Files Changed**:
- `internal/crypto/crypto.go` - Now contains only crypto operations
- `internal/crypto/keys.go` - New file for key file operations

### 2. Magic Numbers Extracted to Constants ✅

**Issue**: Hard-coded values like 2048, RSAOAEPOverhead scattered through code

**Solution**:
- Added comprehensive constants in `internal/crypto/crypto.go`:
  ```go
  const (
      DefaultKeySize         = 2048
      MinimumKeySize         = 2048
      RecommendedKeySize     = 4096
      RSAOAEPHashSize        = 32 // SHA-256 produces 32 bytes
      RSAOAEPOverhead        = 2*RSAOAEPHashSize + 2
      ChunkLengthSize        = 2 // bytes for chunk length prefix
  )
  ```

**Benefits**:
- Self-documenting code
- Easy to adjust cryptographic parameters
- Clear understanding of size calculations

### 3. More Specific Function Names ✅

**Issue**: Generic function names like `Encrypt`, `Decrypt` were ambiguous

**Solution**:
- Renamed to be more specific:
  - `Encrypt` → `EncryptRSA`
  - `Decrypt` → `DecryptRSA`
  - `EncryptChunked` → `EncryptRSAChunked`
  - `DecryptChunked` → `DecryptRSAChunked`
  - `LoadPublicKey` → `LoadPublicKeyFromFile`
  - `LoadPrivateKey` → `LoadPrivateKeyFromFile`
  - `SavePublicKey` → `SavePublicKeyToFile`
  - `SavePrivateKey` → `SavePrivateKeyToFile`

**Files Updated**:
- `internal/crypto/crypto.go`
- `internal/crypto/keys.go`
- `internal/worker/pool.go`
- `internal/batch/batch.go`
- `internal/middleware/decrypt.go`
- `cmd/server/main.go`
- `cmd/agent/main.go`
- `cmd/keygen/main.go`
- All test files

### 4. Pass Loaded Keys Instead of File Paths ✅

**Issue**: Functions accepted file paths and loaded keys internally, mixing concerns and reducing testability

**Solution**:
- Changed function signatures to accept loaded keys:
  - `worker.Pool.SetPublicKey()` - Now accepts `*rsa.PublicKey` instead of path string
  - `batch.SendWithEncryption()` - Now accepts `*rsa.PublicKey` instead of path string
  - `collector.SetPublicKey()` - Now accepts `*rsa.PublicKey` instead of path string
- Key loading happens once at application startup
- Keys are passed down to components that need them

**Before**:
```go
pool.SetPublicKey("/path/to/key.pem")  // Returns error, loads internally
```

**After**:
```go
publicKey, err := crypto.LoadPublicKeyFromFile("/path/to/key.pem")
if err != nil {
    log.Fatal(err)
}
pool.SetPublicKey(publicKey)  // No error, just sets the key
```

**Benefits**:
- Better testability - can pass mock keys
- Single Responsibility Principle - functions don't do file I/O
- Keys loaded once, not repeatedly
- Clearer error handling at startup

**Files Updated**:
- `internal/worker/pool.go`
- `internal/batch/batch.go`
- `internal/collector/collector.go`
- `cmd/agent/main.go`
- All integration tests

### 5. Removed time.Sleep from Tests ✅

**Issue**: Tests used `time.Sleep` which made them unstable and slow

**Solution**:
- Replaced `time.Sleep` with proper synchronization:
  - **Channels**: Used buffered channels to signal when operations complete
  - **Polling with timeout**: Implemented ticker-based polling for state changes
  - **Atomic operations**: Used `sync/atomic` for thread-safe counter checks
  
**Examples**:

**Worker Pool Tests** (`internal/worker/pool_test.go`):
- Before: `time.Sleep(200 * time.Millisecond)` to wait for request processing
- After: Channel-based signaling from mock server + timeout

**Collector Tests** (`internal/collector/collector_test.go`):
- Before: `time.Sleep(150 * time.Millisecond)` to wait for metrics
- After: Ticker-based polling with atomic counter checks

**Storage Tests** (`storage/file_storage_test.go`):
- Before: `time.Sleep(200 * time.Millisecond)` to wait for periodic save
- After: Polling for file existence with ticker + timeout

**Audit Tests** (`internal/audit/audit_test.go`):
- Before: `time.Sleep(50 * time.Millisecond)` assuming async writes
- After: Removed (audit writes are synchronous)

**Benefits**:
- Tests are more reliable
- Faster test execution (tests complete as soon as operation finishes)
- Clearer test intent
- No arbitrary wait times

**Files Updated**:
- `internal/worker/pool_test.go`
- `internal/collector/collector_test.go`
- `storage/file_storage_test.go`
- `internal/audit/audit_test.go`

### 6. Refactored Large Config Functions ✅

**Issue**: `config/config.go` and `internal/agent/config.go` had very large functions with multiple responsibilities

**Solution**:
- Broke down `Load()` and `ParseConfig()` into smaller, focused functions:
  - **Flag parsing**: `parseFlags()`, `parseAgentFlags()`
  - **Config file handling**: `resolveConfigPath()`, `loadJSONConfigFile()`
  - **Per-field resolution**: `resolveServerAddress()`, `resolvePollInterval()`, etc.
  - **Helper functions**: `parseStoreIntervalFromJSON()`, `shouldUseFileStorage()`
  - **Validation**: `validateAgentFlags()`
  - **Logging**: `logAgentConfig()`

**Server Config Structure** (`config/config.go`):
```go
Load()
├── parseFlags()
├── loadJSONConfigFile()
│   ├── resolveConfigPath()
│   └── loadJSONConfig()
├── resolveServerAddress()
├── resolvePollInterval()
├── resolveReportInterval()
├── resolveStoreInterval()
│   └── parseStoreIntervalFromJSON()
├── resolveDatabaseDSN()
├── resolveRestore()
├── resolveKey()
├── resolveCryptoKey()
├── resolveAuditFile()
├── resolveAuditURL()
├── resolveFileStoragePath()
└── shouldUseFileStorage()
```

**Agent Config Structure** (`internal/agent/config.go`):
```go
ParseConfig()
├── parseAgentFlags()
├── validateAgentFlags()
├── loadAgentJSONConfig()
│   ├── resolveAgentConfigPath()
│   └── loadJSONConfig()
├── resolveAgentServerAddress()
├── resolveAgentKey()
├── resolveAgentCryptoKey()
├── resolveAgentRateLimit()
├── resolveAgentReportInterval()
│   └── parseAgentIntervalFromJSON()
├── resolveAgentPollInterval()
│   └── parseAgentIntervalFromJSON()
├── resolveAgentBatchSize()
├── resolveAgentRetryConfig()
└── logAgentConfig()
```

**Benefits**:
- Each function has a single, clear purpose
- Easier to test individual components
- Better readability and maintainability
- Clearer data flow
- Easier to add new configuration options

**Files Updated**:
- `config/config.go` - Completely refactored
- `internal/agent/config.go` - Completely refactored

## Summary of Changes

### New Files Created
- `internal/crypto/keys.go` - Key file I/O operations

### Files Significantly Refactored
- `internal/crypto/crypto.go` - Pure crypto operations with constants
- `config/config.go` - Broken into small, focused functions
- `internal/agent/config.go` - Broken into small, focused functions

### Files Updated for API Changes
- `internal/worker/pool.go`
- `internal/batch/batch.go`
- `internal/collector/collector.go`
- `internal/middleware/decrypt.go`
- `cmd/server/main.go`
- `cmd/agent/main.go`
- `cmd/keygen/main.go`

### Test Files Improved
- `internal/crypto/crypto_test.go`
- `internal/worker/pool_test.go`
- `internal/collector/collector_test.go`
- `storage/file_storage_test.go`
- `internal/audit/audit_test.go`
- `cmd/agent/agent_crypto_integration_test.go`

## Testing

All tests pass successfully:
```bash
go test ./... -short
# All packages: ok
# Total: 100% pass rate
```

## Code Quality Improvements

1. **Better Separation of Concerns**: Each function/file has a single responsibility
2. **Improved Testability**: Components can be tested in isolation with mock data
3. **Self-Documenting Code**: Constants and function names clearly express intent
4. **More Reliable Tests**: No race conditions or timing dependencies
5. **Better Maintainability**: Smaller functions are easier to understand and modify
6. **Follows Best Practices**: Single Responsibility Principle, clear naming, proper abstraction

## Review Response Summary

All review feedback has been addressed:
- ✅ Separated file operations from crypto logic
- ✅ Extracted magic numbers to constants
- ✅ Renamed functions to be more specific
- ✅ Changed to pass loaded keys instead of file paths
- ✅ Removed time.Sleep from tests
- ✅ Refactored large config functions

The codebase is now cleaner, more maintainable, and follows better software engineering practices.

