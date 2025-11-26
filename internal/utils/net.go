package utils

import "net"

// GetOutboundIP gets the preferred outbound IP address of this machine
func GetOutboundIP() string {
	// Try to get the outbound IP by connecting to a public DNS server
	// This doesn't actually send any data, just establishes which interface would be used
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1" // Fallback to localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
