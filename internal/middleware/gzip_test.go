package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzipMiddleware_Compression(t *testing.T) {
	// Create a test handler that returns JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"test": "data"}`))
	})

	// Wrap with gzip middleware
	gzipHandler := GzipMiddleware(handler)

	// Create request with Accept-Encoding: gzip
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	gzipHandler.ServeHTTP(rec, req)

	// Check that response is compressed
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected Content-Encoding: gzip header")
	}

	// Decompress and verify content
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	expected := `{"test": "data"}`
	if string(decompressed) != expected {
		t.Errorf("Expected %s, got %s", expected, string(decompressed))
	}
}

func TestGzipMiddleware_NoCompression(t *testing.T) {
	// Create a test handler that returns JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"test": "data"}`))
	})

	// Wrap with gzip middleware
	gzipHandler := GzipMiddleware(handler)

	// Create request without Accept-Encoding: gzip
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	gzipHandler.ServeHTTP(rec, req)

	// Check that response is not compressed
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Expected no Content-Encoding header")
	}

	// Verify content
	expected := `{"test": "data"}`
	if rec.Body.String() != expected {
		t.Errorf("Expected %s, got %s", expected, rec.Body.String())
	}
}

func TestGzipMiddleware_Decompression(t *testing.T) {
	// Create a test handler that reads the request body
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		w.Write(body)
	})

	// Wrap with gzip middleware
	gzipHandler := GzipMiddleware(handler)

	// Create compressed request body
	testData := `{"test": "compressed data"}`
	var compressedData bytes.Buffer
	gz := gzip.NewWriter(&compressedData)
	gz.Write([]byte(testData))
	gz.Close()

	// Create request with compressed body
	req := httptest.NewRequest("POST", "/", &compressedData)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gzipHandler.ServeHTTP(rec, req)

	// Verify that the handler received decompressed data
	if rec.Body.String() != testData {
		t.Errorf("Expected %s, got %s", testData, rec.Body.String())
	}
}

func TestGzipMiddleware_HTMLCompression(t *testing.T) {
	// Create a test handler that returns HTML
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>Test</body></html>"))
	})

	// Wrap with gzip middleware
	gzipHandler := GzipMiddleware(handler)

	// Create request with Accept-Encoding: gzip
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	gzipHandler.ServeHTTP(rec, req)

	// Check that response is compressed
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected Content-Encoding: gzip header for HTML")
	}
}

func TestGzipMiddleware_NonCompressibleContent(t *testing.T) {
	// Create a test handler that returns binary content
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("binary data"))
	})

	// Wrap with gzip middleware
	gzipHandler := GzipMiddleware(handler)

	// Create request with Accept-Encoding: gzip
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	gzipHandler.ServeHTTP(rec, req)

	// Check that response is not compressed
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Expected no compression for binary content")
	}
}
