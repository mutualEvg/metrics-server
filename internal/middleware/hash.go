package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/mutualEvg/metrics-server/internal/hash"
	"github.com/rs/zerolog/log"
)

// HashVerification returns middleware that verifies SHA256 hash signatures
func HashVerification(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no key is configured, skip hash verification
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Only verify hash for requests with body (POST, PUT, etc.)
			if r.Body == nil || r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Get the provided hash from header
			providedHash := r.Header.Get("HashSHA256")

			// If no hash is provided, allow the request to pass through
			// This allows test clients to work without hashes while
			// still verifying hashes when they are provided (like from agent)
			if providedHash == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Read the request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read request body for hash verification")
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			// Restore the request body for subsequent handlers
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Verify the hash
			if !hash.VerifyHash(body, key, providedHash) {
				log.Warn().
					Str("provided_hash", providedHash).
					Str("method", r.Method).
					Str("url", r.URL.Path).
					Msg("Hash verification failed")
				http.Error(w, "Hash verification failed", http.StatusBadRequest)
				return
			}

			log.Debug().
				Str("hash", providedHash).
				Str("method", r.Method).
				Str("url", r.URL.Path).
				Msg("Hash verification successful")

			next.ServeHTTP(w, r)
		})
	}
}
