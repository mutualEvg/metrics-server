# Trusted Subnet Implementation

## Overview
This implementation adds trusted subnet validation to the metrics server, allowing you to restrict which IP addresses can send metrics based on CIDR notation.

## Features Implemented

### 1. Server Configuration
Added `trusted_subnet` field to server configuration with support for:
- **JSON configuration**: `trusted_subnet` field in JSON config file
- **Environment variable**: `TRUSTED_SUBNET`
- **Command-line flag**: `-t`

Priority order: Environment Variable > Command-line Flag > JSON Config > Default (empty)

### 2. Agent X-Real-IP Header
The agent now automatically adds the `X-Real-IP` header to all HTTP requests containing the agent's outbound IP address.

Implementation details:
- Uses the outbound network interface IP (determined by connecting to 8.8.8.8:80)
- Falls back to 127.0.0.1 if unable to determine the IP
- Applied to both individual metric requests and batch requests

### 3. Server IP Validation Middleware
Created `TrustedSubnetMiddleware` that:
- Validates the `X-Real-IP` header against the configured trusted subnet
- Returns 403 Forbidden if the IP is not in the trusted subnet
- Allows all requests if `trusted_subnet` is empty (disabled mode)
- Handles IPv4 and IPv6 addresses
- Validates CIDR notation on startup

## Configuration Examples

### JSON Configuration
```json
{
    "address": "localhost:8080",
    "trusted_subnet": "192.168.1.0/24"
}
```

### Environment Variable
```bash
export TRUSTED_SUBNET="192.168.1.0/24"
./bin/server
```

### Command-line Flag
```bash
./bin/server -t "192.168.1.0/24"
```

### Disable Trusted Subnet Validation
Leave the field empty or don't set it:
```bash
./bin/server  # All IPs allowed
```

## CIDR Examples

- `192.168.1.0/24` - Allows IPs from 192.168.1.0 to 192.168.1.255
- `10.0.0.0/8` - Allows IPs from 10.0.0.0 to 10.255.255.255
- `127.0.0.0/8` - Allows all localhost addresses
- `192.168.1.100/32` - Allows only the specific IP 192.168.1.100
- `2001:db8::/32` - IPv6 subnet support

## Behavior

### When Trusted Subnet is Configured:
1. Server validates every incoming request for the `X-Real-IP` header
2. If header is missing → 403 Forbidden
3. If IP is invalid format → 403 Forbidden
4. If IP is not in trusted subnet → 403 Forbidden
5. If IP is in trusted subnet → Request processed normally

### When Trusted Subnet is Empty:
- All requests are processed without IP validation
- No `X-Real-IP` header requirement
- Maintains backward compatibility

## Files Modified

### Configuration
- `config/config.go` - Added `TrustedSubnet` field and resolution logic
- `config/server_config.json.example` - Added example `trusted_subnet` field

### Middleware
- `internal/middleware/trusted_subnet.go` - New middleware for IP validation
- `internal/middleware/trusted_subnet_test.go` - Comprehensive unit tests

### Agent
- `internal/batch/batch.go` - Added `X-Real-IP` header to batch requests
- `internal/worker/pool.go` - Added `X-Real-IP` header to individual requests

### Server
- `cmd/server/main.go` - Integrated trusted subnet middleware
- `cmd/server/trusted_subnet_integration_test.go` - End-to-end integration tests

## Testing

All tests pass successfully:

```bash
# Run all tests
go test ./...

# Run specific trusted subnet tests
go test -v ./internal/middleware/... -run TestTrustedSubnet
go test -v ./cmd/server/... -run TestTrustedSubnet
```

### Test Coverage:
- Empty trusted subnet (allow all)
- Valid IP in subnet
- IP outside subnet
- Missing X-Real-IP header
- Invalid IP format
- IPv4 and IPv6 support
- Single IP with /32 CIDR
- Batch requests
- Invalid CIDR handling

## Security Considerations

1. **IP Spoofing**: The `X-Real-IP` header is set by the agent. If you're using a reverse proxy, configure it to override this header with the actual client IP.

2. **Network Architecture**: This feature is designed for trusted networks. If your agents are behind NAT or proxies, configure the trusted subnet to match the NAT/proxy IP range.

3. **Empty Configuration**: An empty `trusted_subnet` allows all IPs. Ensure this is intentional if used in production.

## Backward Compatibility

- When `trusted_subnet` is not configured, the system behaves exactly as before
- Existing configurations without this field continue to work
- No breaking changes to API or behavior when feature is disabled

## Example Usage

### Scenario 1: Local Development
```bash
# Allow only localhost
export TRUSTED_SUBNET="127.0.0.0/8"
./bin/server
```

### Scenario 2: Private Network
```bash
# Allow entire private network
export TRUSTED_SUBNET="192.168.0.0/16"
./bin/server
```

### Scenario 3: Specific Subnet
```json
{
    "address": "localhost:8080",
    "trusted_subnet": "10.0.1.0/24"
}
```

### Scenario 4: No Restrictions (Default)
```bash
# No environment variable or config
./bin/server  # All IPs allowed
```

## Logging

The middleware logs all validation events:
- Subnet configuration on startup
- Rejected requests with reason
- IP addresses that fail validation

Example logs:
```
2025/11/23 17:48:27 Trusted subnet configured: 192.168.1.0/24
2025/11/23 17:48:27 Request from 192.168.2.10 rejected: IP not in trusted subnet 192.168.1.0/24
2025/11/23 17:48:27 Request from 192.0.2.1:1234 rejected: X-Real-IP header is missing
```

## Summary

This implementation provides a robust and flexible IP-based access control mechanism for the metrics server while maintaining full backward compatibility. The feature can be easily enabled or disabled through configuration and includes comprehensive testing to ensure reliability.

