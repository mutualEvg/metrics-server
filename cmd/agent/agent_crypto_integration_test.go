package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/batch"
	"github.com/mutualEvg/metrics-server/internal/crypto"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

// TestEncryptedCommunication tests encrypted communication between agent and server
func TestEncryptedCommunication(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := crypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save keys to temp files
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "private.pem")
	pubPath := filepath.Join(tmpDir, "public.pem")

	if err := crypto.SavePrivateKeyToFile(privPath, privateKey); err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	if err := crypto.SavePublicKeyToFile(pubPath, publicKey); err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Track received metrics
	var mu sync.Mutex
	receivedMetrics := make([]models.Metrics, 0)

	// Create test server with decryption middleware
	ts := httptest.NewServer(
		gzipmw.DecryptionMiddleware(privateKey)(
			gzipmw.GzipMiddleware(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Read and decode the body
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Logf("Failed to read body: %v", err)
						http.Error(w, "Failed to read body", http.StatusBadRequest)
						return
					}

					var metric models.Metrics
					if err := json.Unmarshal(body, &metric); err != nil {
						t.Logf("Failed to unmarshal JSON: %v", err)
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}

					mu.Lock()
					receivedMetrics = append(receivedMetrics, metric)
					mu.Unlock()
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(metric)
				}),
			),
		),
	)
	defer ts.Close()

	// Test single metric sending with encryption
	t.Run("single metric with encryption", func(t *testing.T) {
		mu.Lock()
		receivedMetrics = receivedMetrics[:0] // Clear
		mu.Unlock()

		retryConfig := retry.RetryConfig{
			MaxAttempts: 1,
			Intervals:   []time.Duration{},
		}

		// Load public key
		loadedPublicKey, err := crypto.LoadPublicKeyFromFile(pubPath)
		if err != nil {
			t.Fatalf("Failed to load public key: %v", err)
		}

		pool := worker.NewPool(1, ts.URL, "", retryConfig)
		pool.SetPublicKey(loadedPublicKey)
		pool.Start()
		defer pool.Stop()

		// Send a test metric
		value := 123.45
		pool.SubmitMetric(worker.MetricData{
			Metric: models.Metrics{
				ID:    "TestMetric",
				MType: "gauge",
				Value: &value,
			},
			Type: "test",
		})

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		count := len(receivedMetrics)
		var firstMetric models.Metrics
		if count > 0 {
			firstMetric = receivedMetrics[0]
		}
		mu.Unlock()

		if count != 1 {
			t.Errorf("Expected 1 metric, got %d", count)
		}

		if count > 0 {
			if firstMetric.ID != "TestMetric" {
				t.Errorf("Expected metric ID 'TestMetric', got '%s'", firstMetric.ID)
			}
			if firstMetric.Value == nil || *firstMetric.Value != 123.45 {
				t.Errorf("Expected metric value 123.45, got %v", firstMetric.Value)
			}
		}
	})
}

// TestBatchEncryptedCommunication tests batch sending with encryption
func TestBatchEncryptedCommunication(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := crypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save keys to temp files
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "private.pem")
	pubPath := filepath.Join(tmpDir, "public.pem")

	if err := crypto.SavePrivateKeyToFile(privPath, privateKey); err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	if err := crypto.SavePublicKeyToFile(pubPath, publicKey); err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Track received batches
	receivedBatches := make([][]models.Metrics, 0)

	// Create test server with decryption middleware
	ts := httptest.NewServer(
		gzipmw.DecryptionMiddleware(privateKey)(
			gzipmw.GzipMiddleware(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Read and decode the body
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Logf("Failed to read body: %v", err)
						http.Error(w, "Failed to read body", http.StatusBadRequest)
						return
					}

					var metrics []models.Metrics
					if err := json.Unmarshal(body, &metrics); err != nil {
						t.Logf("Failed to unmarshal JSON: %v", err)
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}

					receivedBatches = append(receivedBatches, metrics)
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(metrics)
				}),
			),
		),
	)
	defer ts.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	// Load public key
	loadedPublicKey, err := crypto.LoadPublicKeyFromFile(pubPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	// Create test metrics
	value1 := 10.5
	value2 := 20.5
	delta := int64(5)
	metrics := []models.Metrics{
		{
			ID:    "Metric1",
			MType: "gauge",
			Value: &value1,
		},
		{
			ID:    "Metric2",
			MType: "gauge",
			Value: &value2,
		},
		{
			ID:    "Counter1",
			MType: "counter",
			Delta: &delta,
		},
	}

	// Send batch with encryption
	err = batch.SendWithEncryption(metrics, ts.URL, "", loadedPublicKey, retryConfig)
	if err != nil {
		t.Fatalf("Failed to send encrypted batch: %v", err)
	}

	// Verify received batch
	if len(receivedBatches) != 1 {
		t.Fatalf("Expected 1 batch, got %d", len(receivedBatches))
	}

	if len(receivedBatches[0]) != 3 {
		t.Fatalf("Expected 3 metrics in batch, got %d", len(receivedBatches[0]))
	}

	// Verify metrics content
	for i, metric := range receivedBatches[0] {
		expectedMetric := metrics[i]
		if metric.ID != expectedMetric.ID {
			t.Errorf("Metric %d: expected ID '%s', got '%s'", i, expectedMetric.ID, metric.ID)
		}
		if metric.MType != expectedMetric.MType {
			t.Errorf("Metric %d: expected type '%s', got '%s'", i, expectedMetric.MType, metric.MType)
		}
	}
}

// TestUnencryptedCommunicationWithEncryptionEnabled tests that unencrypted requests still work
func TestUnencryptedCommunicationWithEncryptionEnabled(t *testing.T) {
	// Generate test key pair for server
	privateKey, _, err := crypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Track received metrics
	var mu sync.Mutex
	receivedMetrics := make([]models.Metrics, 0)

	// Create test server with decryption middleware (but send unencrypted data)
	ts := httptest.NewServer(
		gzipmw.DecryptionMiddleware(privateKey)(
			gzipmw.GzipMiddleware(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "Failed to read body", http.StatusBadRequest)
						return
					}

					var metric models.Metrics
					if err := json.Unmarshal(body, &metric); err != nil {
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}

					mu.Lock()
					receivedMetrics = append(receivedMetrics, metric)
					mu.Unlock()
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(metric)
				}),
			),
		),
	)
	defer ts.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	// Create worker pool WITHOUT encryption (no crypto key set)
	pool := worker.NewPool(1, ts.URL, "", retryConfig)
	pool.Start()
	defer pool.Stop()

	// Send a test metric (unencrypted)
	value := 99.99
	pool.SubmitMetric(worker.MetricData{
		Metric: models.Metrics{
			ID:    "UnencryptedMetric",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	})

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify metric was received
	mu.Lock()
	count := len(receivedMetrics)
	var firstMetric models.Metrics
	if count > 0 {
		firstMetric = receivedMetrics[0]
	}
	mu.Unlock()

	if count != 1 {
		t.Errorf("Expected 1 metric, got %d", count)
	}

	if count > 0 {
		if firstMetric.ID != "UnencryptedMetric" {
			t.Errorf("Expected metric ID 'UnencryptedMetric', got '%s'", firstMetric.ID)
		}
	}
}

// TestEncryptionWithInvalidKey tests that encryption fails gracefully with invalid key
func TestEncryptionWithInvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	invalidKeyPath := filepath.Join(tmpDir, "invalid.pem")

	// Create invalid key file
	if err := os.WriteFile(invalidKeyPath, []byte("not a valid key"), 0644); err != nil {
		t.Fatalf("Failed to create invalid key file: %v", err)
	}

	// Try to load invalid key - should fail
	_, err := crypto.LoadPublicKeyFromFile(invalidKeyPath)
	if err == nil {
		t.Error("Expected error when loading invalid key, got nil")
	}
}

// TestLargePayloadEncryption tests encryption of large payloads
func TestLargePayloadEncryption(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := crypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save keys to temp files
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "private.pem")
	pubPath := filepath.Join(tmpDir, "public.pem")

	if err := crypto.SavePrivateKeyToFile(privPath, privateKey); err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	if err := crypto.SavePublicKeyToFile(pubPath, publicKey); err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Track total metrics received
	totalReceived := 0

	// Create test server with decryption middleware
	ts := httptest.NewServer(
		gzipmw.DecryptionMiddleware(privateKey)(
			gzipmw.GzipMiddleware(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "Failed to read body", http.StatusBadRequest)
						return
					}

					var metrics []models.Metrics
					if err := json.Unmarshal(body, &metrics); err != nil {
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}

					totalReceived += len(metrics)
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(metrics)
				}),
			),
		),
	)
	defer ts.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	// Load public key
	loadedPublicKey, err := crypto.LoadPublicKeyFromFile(pubPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	// Create large batch of metrics
	metrics := make([]models.Metrics, 100)
	for i := 0; i < 100; i++ {
		value := float64(i)
		metrics[i] = models.Metrics{
			ID:    fmt.Sprintf("Metric%d", i),
			MType: "gauge",
			Value: &value,
		}
	}

	// Send large batch with encryption
	err = batch.SendWithEncryption(metrics, ts.URL, "", loadedPublicKey, retryConfig)
	if err != nil {
		t.Fatalf("Failed to send large encrypted batch: %v", err)
	}

	// Verify all metrics were received
	if totalReceived != 100 {
		t.Errorf("Expected 100 metrics, got %d", totalReceived)
	}
}
