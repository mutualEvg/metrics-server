package batch

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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
		metrics: make([]models.Metrics, 0),
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

	result := make([]models.Metrics, len(b.metrics))
	copy(result, b.metrics)
	b.metrics = b.metrics[:0] // Clear the slice
	return result
}

// Send sends a batch of metrics using the /updates/ endpoint
func Send(metrics []models.Metrics, serverAddr, key string, retryConfig retry.RetryConfig) error {
	if len(metrics) == 0 {
		return nil // Don't send empty batches
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

		// Create HTTP request
		url := fmt.Sprintf("%s/updates/", serverAddr)
		req, err := http.NewRequest("POST", url, &compressedData)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		// Add hash header if key is configured
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
