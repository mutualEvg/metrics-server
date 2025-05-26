package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// GzipMiddleware handles gzip compression and decompression
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle decompression of incoming requests
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		// Check if client accepts gzip encoding for response compression
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap the response writer to handle compression
		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			request:        r,
		}
		defer gzw.Close()

		next.ServeHTTP(gzw, r)
	})
}

// gzipResponseWriter wraps http.ResponseWriter to provide gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	request       *http.Request
	gzipWriter    *gzip.Writer
	headerWritten bool
}

func (grw *gzipResponseWriter) WriteHeader(statusCode int) {
	if grw.headerWritten {
		return
	}
	grw.headerWritten = true

	// Check if we should compress based on content type
	contentType := grw.Header().Get("Content-Type")
	if grw.shouldCompress(contentType) {
		grw.Header().Set("Content-Encoding", "gzip")
		grw.Header().Del("Content-Length") // Remove content-length as it will change
		grw.gzipWriter = gzip.NewWriter(grw.ResponseWriter)
	}

	grw.ResponseWriter.WriteHeader(statusCode)
}

func (grw *gzipResponseWriter) Write(data []byte) (int, error) {
	if !grw.headerWritten {
		// Set content type if not already set
		if grw.Header().Get("Content-Type") == "" {
			grw.Header().Set("Content-Type", http.DetectContentType(data))
		}
		grw.WriteHeader(http.StatusOK)
	}

	if grw.gzipWriter != nil {
		return grw.gzipWriter.Write(data)
	}
	return grw.ResponseWriter.Write(data)
}

func (grw *gzipResponseWriter) Close() error {
	if grw.gzipWriter != nil {
		return grw.gzipWriter.Close()
	}
	return nil
}

// shouldCompress determines if the content type should be compressed
func (grw *gzipResponseWriter) shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"application/json",
		"text/html",
		"text/plain",
	}

	for _, ct := range compressibleTypes {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}
