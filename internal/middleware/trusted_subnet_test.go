package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedSubnetMiddleware(t *testing.T) {
	// Create a simple handler that returns 200 OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		trustedSubnet  string
		realIP         string
		expectedStatus int
	}{
		{
			name:           "Empty trusted subnet - allow all",
			trustedSubnet:  "",
			realIP:         "192.168.1.1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Empty trusted subnet - no X-Real-IP header",
			trustedSubnet:  "",
			realIP:         "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid IP in trusted subnet",
			trustedSubnet:  "192.168.1.0/24",
			realIP:         "192.168.1.10",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid IP at subnet boundary",
			trustedSubnet:  "192.168.1.0/24",
			realIP:         "192.168.1.255",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "IP outside trusted subnet",
			trustedSubnet:  "192.168.1.0/24",
			realIP:         "192.168.2.10",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Missing X-Real-IP header with trusted subnet",
			trustedSubnet:  "192.168.1.0/24",
			realIP:         "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Invalid IP address format",
			trustedSubnet:  "192.168.1.0/24",
			realIP:         "invalid-ip",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Localhost in 127.0.0.0/8 subnet",
			trustedSubnet:  "127.0.0.0/8",
			realIP:         "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "IPv6 address in trusted subnet",
			trustedSubnet:  "2001:db8::/32",
			realIP:         "2001:db8::1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "IPv6 address outside trusted subnet",
			trustedSubnet:  "2001:db8::/32",
			realIP:         "2001:db9::1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Single IP with /32 CIDR",
			trustedSubnet:  "192.168.1.100/32",
			realIP:         "192.168.1.100",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Single IP with /32 CIDR - different IP",
			trustedSubnet:  "192.168.1.100/32",
			realIP:         "192.168.1.101",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware := TrustedSubnetMiddleware(tt.trustedSubnet)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("POST", "/update/", nil)
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// For successful requests, verify the handler was called
			if tt.expectedStatus == http.StatusOK {
				if rr.Body.String() != "OK" {
					t.Errorf("Expected body 'OK', got '%s'", rr.Body.String())
				}
			}
		})
	}
}

func TestTrustedSubnetMiddleware_InvalidCIDR(t *testing.T) {
	// Test with invalid CIDR notation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Invalid CIDR should behave like empty subnet (allow all)
	middleware := TrustedSubnetMiddleware("invalid-cidr")
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("POST", "/update/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	// Should allow all when CIDR is invalid
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d for invalid CIDR, got %d", http.StatusOK, rr.Code)
	}
}
