package middleware

import (
	"log"
	"net"
	"net/http"
)

// TrustedSubnetMiddleware validates that the X-Real-IP header contains an IP
// that belongs to the trusted subnet (CIDR notation).
// If trustedSubnet is empty, all requests are allowed.
func TrustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	var ipNet *net.IPNet
	var err error

	// Parse the trusted subnet if provided
	if trustedSubnet != "" {
		_, ipNet, err = net.ParseCIDR(trustedSubnet)
		if err != nil {
			log.Printf("Warning: Invalid trusted subnet CIDR %s: %v. All IPs will be allowed.", trustedSubnet, err)
			ipNet = nil
		} else {
			log.Printf("Trusted subnet configured: %s", trustedSubnet)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no trusted subnet is configured, allow all requests
			if ipNet == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get X-Real-IP header
			realIP := r.Header.Get("X-Real-IP")
			if realIP == "" {
				log.Printf("Request from %s rejected: X-Real-IP header is missing", r.RemoteAddr)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Parse the IP address
			ip := net.ParseIP(realIP)
			if ip == nil {
				log.Printf("Request rejected: Invalid IP address in X-Real-IP header: %s", realIP)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if IP is in the trusted subnet
			if !ipNet.Contains(ip) {
				log.Printf("Request from %s rejected: IP not in trusted subnet %s", realIP, trustedSubnet)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// IP is in trusted subnet, allow the request
			next.ServeHTTP(w, r)
		})
	}
}
