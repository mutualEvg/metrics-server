package middleware

import (
	"bytes"
	"net/http"

	"github.com/mutualEvg/metrics-server/internal/hash"
)

type responseHashWriter struct {
	http.ResponseWriter
	key    string
	buffer *bytes.Buffer
}

func (rw *responseHashWriter) Write(data []byte) (int, error) {
	// Buffer the response data
	rw.buffer.Write(data)
	return len(data), nil
}

func (rw *responseHashWriter) WriteHeader(statusCode int) {
	// Don't write the header yet, we need to capture the full response
	rw.ResponseWriter.WriteHeader(statusCode)
}

// ResponseHash returns middleware that adds SHA256 hash to response headers
func ResponseHash(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no key is configured, skip hash generation
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Create a response writer that buffers the response
			rw := &responseHashWriter{
				ResponseWriter: w,
				key:            key,
				buffer:         &bytes.Buffer{},
			}

			// Call the next handler with our buffering writer
			next.ServeHTTP(rw, r)

			// Calculate hash of the response body
			responseData := rw.buffer.Bytes()
			if len(responseData) > 0 {
				responseHash := hash.CalculateHash(responseData, key)
				w.Header().Set("HashSHA256", responseHash)
			}

			// Write the actual response
			w.Write(responseData)
		})
	}
}
