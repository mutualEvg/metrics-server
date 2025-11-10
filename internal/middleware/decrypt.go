package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"log"
	"net/http"

	"github.com/mutualEvg/metrics-server/internal/crypto"
)

// DecryptionMiddleware creates a middleware that decrypts encrypted request bodies
func DecryptionMiddleware(privateKey *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request is encrypted
			if r.Header.Get("X-Encrypted") != "true" {
				// Not encrypted, pass through
				next.ServeHTTP(w, r)
				return
			}

			// Read encrypted body
			encryptedBody, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Failed to read encrypted body: %v", err)
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// Decrypt the body
			decryptedBody, err := crypto.DecryptChunked(encryptedBody, privateKey)
			if err != nil {
				log.Printf("Failed to decrypt body: %v", err)
				http.Error(w, "Failed to decrypt request", http.StatusBadRequest)
				return
			}

			// Replace the request body with decrypted data
			r.Body = io.NopCloser(bytes.NewReader(decryptedBody))
			r.ContentLength = int64(len(decryptedBody))

			// Remove the encryption header since body is now decrypted
			r.Header.Del("X-Encrypted")

			next.ServeHTTP(w, r)
		})
	}
}
