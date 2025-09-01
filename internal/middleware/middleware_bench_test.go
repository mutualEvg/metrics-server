package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mutualEvg/metrics-server/internal/middleware"
)

// BenchmarkGzipMiddleware benchmarks the gzip middleware with different payload sizes
func BenchmarkGzipMiddleware(b *testing.B) {
	// Create a test handler that returns a sizeable response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := strings.Repeat("Hello, World! This is test data for compression benchmarking. ", 100) // ~6KB
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(data))
	})

	gzipHandler := middleware.GzipMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		gzipHandler.ServeHTTP(w, req)
	}
}

// BenchmarkGzipMiddlewareSmallPayload benchmarks gzip with small payloads
func BenchmarkGzipMiddlewareSmallPayload(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	gzipHandler := middleware.GzipMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		gzipHandler.ServeHTTP(w, req)
	}
}

// BenchmarkGzipMiddlewareLargePayload benchmarks gzip with large payloads
func BenchmarkGzipMiddlewareLargePayload(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := strings.Repeat("Large payload data for compression testing with gzip middleware. ", 1000) // ~60KB
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(data))
	})

	gzipHandler := middleware.GzipMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		gzipHandler.ServeHTTP(w, req)
	}
}

// BenchmarkGzipMiddlewareWithoutCompression benchmarks without gzip compression
func BenchmarkGzipMiddlewareWithoutCompression(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := strings.Repeat("Test data without compression. ", 100) // ~3KB
		w.Write([]byte(data))
	})

	gzipHandler := middleware.GzipMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		// Don't set Accept-Encoding header

		w := httptest.NewRecorder()
		gzipHandler.ServeHTTP(w, req)
	}
}

// BenchmarkGzipDecompression benchmarks gzip decompression of request bodies
func BenchmarkGzipDecompression(b *testing.B) {
	// Create compressed data
	originalData := strings.Repeat("JSON data for decompression testing: ", 50) // ~1.8KB
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	gzipWriter.Write([]byte(originalData))
	gzipWriter.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and decompress the request body
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Content-Encoding") == "gzip" {
			reader, _ := gzip.NewReader(bytes.NewReader(body))
			decompressed, _ := io.ReadAll(reader)
			reader.Close()
			_ = decompressed
		}
		w.Write([]byte("OK"))
	})

	gzipHandler := middleware.GzipMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(compressedData.Bytes()))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gzipHandler.ServeHTTP(w, req)
	}
}

// BenchmarkContentTypeMiddleware benchmarks content type middleware
func BenchmarkContentTypeMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	contentTypeHandler := middleware.RequireContentType("application/json")(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"test": "data"}`))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		contentTypeHandler.ServeHTTP(w, req)
	}
}

// BenchmarkHashMiddleware benchmarks hash verification middleware
func BenchmarkHashMiddleware(b *testing.B) {
	testKey := "test-secret-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	hashHandler := middleware.HashVerification(testKey)(handler)

	// Create test data with hash
	testData := `{"id": "test", "type": "gauge", "value": 123.45}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", strings.NewReader(testData))
		req.Header.Set("Content-Type", "application/json")
		// Note: In a real scenario, we'd calculate the correct hash
		req.Header.Set("HashSHA256", "dummy-hash-for-benchmark")

		w := httptest.NewRecorder()
		hashHandler.ServeHTTP(w, req)
	}
}

// BenchmarkResponseHashMiddleware benchmarks response hash middleware
func BenchmarkResponseHashMiddleware(b *testing.B) {
	testKey := "test-secret-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "test", "type": "gauge", "value": 123.45}`))
	})

	responseHashHandler := middleware.ResponseHash(testKey)(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)

		w := httptest.NewRecorder()
		responseHashHandler.ServeHTTP(w, req)
	}
}

// BenchmarkMiddlewareChain benchmarks a full middleware chain
func BenchmarkMiddlewareChain(b *testing.B) {
	testKey := "test-secret-key"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := strings.Repeat("Response data for middleware chain testing. ", 50) // ~2KB
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(data))
	})

	// Apply multiple middleware layers
	chainedHandler := middleware.GzipMiddleware(
		middleware.RequireContentType("application/json")(
			middleware.ResponseHash(testKey)(handler),
		),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		w := httptest.NewRecorder()
		chainedHandler.ServeHTTP(w, req)
	}
}
