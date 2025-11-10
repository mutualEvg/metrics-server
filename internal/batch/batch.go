package batch

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mutualEvg/metrics-server/internal/crypto"
	"github.com/mutualEvg/metrics-server/internal/hash"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

// Batch holds a collection of metrics to send as batch
type Batch struct {
	metrics []models.Metrics
	mu      sync.Mutex
}

// New creates a new batch
func New() *Batch {
	return &Batch{
		metrics: make([]models.Metrics, 0, 50), // Pre-allocate capacity to avoid slice growth
	}
}

// AddGauge adds a gauge metric to the batch
func (b *Batch) AddGauge(name string, value float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metrics = append(b.metrics, models.Metrics{
		ID:    name,
		MType: "gauge",
		Value: &value,
	})
}

// AddCounter adds a counter metric to the batch
func (b *Batch) AddCounter(name string, delta int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metrics = append(b.metrics, models.Metrics{
		ID:    name,
		MType: "counter",
		Delta: &delta,
	})
}

// GetAndClear returns all metrics and clears the batch
func (b *Batch) GetAndClear() []models.Metrics {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.metrics) == 0 {
		return nil
	}

	// Return the existing slice and create a new one instead of copying
	// This avoids the large memory allocation from copying
	result := b.metrics
	b.metrics = make([]models.Metrics, 0, cap(b.metrics)) // Reuse capacity
	return result
}

// Send sends a batch of metrics using the /updates/ endpoint
func Send(metrics []models.Metrics, serverAddr, key string, retryConfig retry.RetryConfig) error {
	return SendWithEncryption(metrics, serverAddr, key, "", retryConfig)
}

// SendWithEncryption sends a batch of metrics with optional encryption
func SendWithEncryption(metrics []models.Metrics, serverAddr, key, publicKeyPath string, retryConfig retry.RetryConfig) error {
	if len(metrics) == 0 {
		return nil // Don't send empty batches
	}

	// Load public key if provided
	var publicKey *rsa.PublicKey
	if publicKeyPath != "" {
		var err error
		publicKey, err = crypto.LoadPublicKey(publicKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load public key: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return retry.Do(ctx, retryConfig, func() error {
		// Marshal to JSON
		jsonData, err := json.Marshal(metrics)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}

		// Compress with gzip
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		if _, err := gzipWriter.Write(jsonData); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		// Prepare body data (may be encrypted)
		bodyData := compressedData.Bytes()

		// Encrypt if public key is configured
		if publicKey != nil {
			encryptedData, err := crypto.EncryptChunked(bodyData, publicKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt data: %w", err)
			}
			bodyData = encryptedData
		}

		// Create HTTP request
		url := fmt.Sprintf("%s/updates/", serverAddr)
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		// Add encryption header if data is encrypted
		if publicKey != nil {
			req.Header.Set("X-Encrypted", "true")
		}

		// Add hash header if key is configured (hash is computed before encryption)
		if key != "" {
			hashValue := hash.CalculateHash(compressedData.Bytes(), key)
			req.Header.Set("HashSHA256", hashValue)
		}

		// Send request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status %d", resp.StatusCode)
		}

		return nil
	})
}
