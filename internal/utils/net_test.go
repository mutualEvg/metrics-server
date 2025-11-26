package utils

import (
	"net"
	"testing"
)

func TestGetOutboundIP(t *testing.T) {
	ip := GetOutboundIP()

	// Should return a valid IP address
	if ip == "" {
		t.Error("GetOutboundIP returned empty string")
	}

	// Should be a valid IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		t.Errorf("GetOutboundIP returned invalid IP: %s", ip)
	}

	// Should return either localhost or a valid IPv4/IPv6 address
	if ip != "127.0.0.1" && parsedIP.To4() == nil && parsedIP.To16() == nil {
		t.Errorf("GetOutboundIP returned unexpected IP format: %s", ip)
	}
}
