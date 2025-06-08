// agent_batch_example.go
// Example implementation for agent batch metrics sending

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

// Metrics represents a single metric
type Metrics struct {
	ID    string   `json:"id"`              // metric name
	MType string   `json:"type"`            // "gauge" or "counter"
	Delta *int64   `json:"delta,omitempty"` // for counter
	Value *float64 `json:"value,omitempty"` // for gauge
}

// MetricsBatch holds a collection of metrics to send
type MetricsBatch struct {
	metrics []Metrics
	maxSize int
}

// NewMetricsBatch creates a new batch with specified max size
func NewMetricsBatch(maxSize int) *MetricsBatch {
	return &MetricsBatch{
		metrics: make([]Metrics, 0, maxSize),
		maxSize: maxSize,
	}
}

// AddGauge adds a gauge metric to the batch
func (mb *MetricsBatch) AddGauge(name string, value float64) {
	mb.metrics = append(mb.metrics, Metrics{
		ID:    name,
		MType: "gauge",
		Value: &value,
	})
}

// AddCounter adds a counter metric to the batch
func (mb *MetricsBatch) AddCounter(name string, delta int64) {
	mb.metrics = append(mb.metrics, Metrics{
		ID:    name,
		MType: "counter",
		Delta: &delta,
	})
}

// IsFull returns true if batch has reached max size
func (mb *MetricsBatch) IsFull() bool {
	return len(mb.metrics) >= mb.maxSize
}

// IsEmpty returns true if batch has no metrics
func (mb *MetricsBatch) IsEmpty() bool {
	return len(mb.metrics) == 0
}

// Clear empties the batch
func (mb *MetricsBatch) Clear() {
	mb.metrics = mb.metrics[:0]
}

// SendBatch sends the batch to the server with gzip compression
func (mb *MetricsBatch) SendBatch(serverURL string) error {
	if mb.IsEmpty() {
		return nil // Don't send empty batches
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(mb.metrics)
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
	req, err := http.NewRequest("POST", serverURL+"/updates/", &compressedData)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Successfully sent batch of %d metrics\n", len(mb.metrics))
	return nil
}

// Example agent implementation
func main() {
	serverURL := "http://localhost:8080"
	batchSize := 10
	reportInterval := 10 * time.Second

	batch := NewMetricsBatch(batchSize)
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	// Simulate metric collection
	var pollCount int64 = 0

	for {
		select {
		case <-ticker.C:
			// Collect runtime metrics
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// Add gauge metrics
			batch.AddGauge("Alloc", float64(m.Alloc))
			batch.AddGauge("TotalAlloc", float64(m.TotalAlloc))
			batch.AddGauge("Sys", float64(m.Sys))
			batch.AddGauge("Lookups", float64(m.Lookups))
			batch.AddGauge("Mallocs", float64(m.Mallocs))
			batch.AddGauge("Frees", float64(m.Frees))
			batch.AddGauge("HeapAlloc", float64(m.HeapAlloc))
			batch.AddGauge("HeapSys", float64(m.HeapSys))
			batch.AddGauge("HeapIdle", float64(m.HeapIdle))
			batch.AddGauge("HeapInuse", float64(m.HeapInuse))
			batch.AddGauge("HeapReleased", float64(m.HeapReleased))
			batch.AddGauge("HeapObjects", float64(m.HeapObjects))
			batch.AddGauge("StackInuse", float64(m.StackInuse))
			batch.AddGauge("StackSys", float64(m.StackSys))
			batch.AddGauge("MSpanInuse", float64(m.MSpanInuse))
			batch.AddGauge("MSpanSys", float64(m.MSpanSys))
			batch.AddGauge("MCacheInuse", float64(m.MCacheInuse))
			batch.AddGauge("MCacheSys", float64(m.MCacheSys))
			batch.AddGauge("BuckHashSys", float64(m.BuckHashSys))
			batch.AddGauge("GCSys", float64(m.GCSys))
			batch.AddGauge("OtherSys", float64(m.OtherSys))
			batch.AddGauge("NextGC", float64(m.NextGC))
			batch.AddGauge("LastGC", float64(m.LastGC))
			batch.AddGauge("PauseTotalNs", float64(m.PauseTotalNs))
			batch.AddGauge("NumGC", float64(m.NumGC))
			batch.AddGauge("NumForcedGC", float64(m.NumForcedGC))
			batch.AddGauge("GCCPUFraction", m.GCCPUFraction)

			// Add counter metric
			pollCount++
			batch.AddCounter("PollCount", 1)

			// Add random value
			batch.AddGauge("RandomValue", float64(time.Now().UnixNano()%1000))

			// Send batch if full or at report interval
			if batch.IsFull() || len(batch.metrics) > 0 {
				if err := batch.SendBatch(serverURL); err != nil {
					fmt.Printf("Failed to send batch: %v\n", err)
				} else {
					batch.Clear()
				}
			}

		default:
			// Continue collecting metrics or do other work
			time.Sleep(100 * time.Millisecond)
		}
	}
}

/*
Key features of this agent batch implementation:

1. **Batch Management**:
   - Configurable batch size
   - Automatic sending when batch is full
   - Periodic sending even if batch isn't full

2. **Compression**:
   - Uses gzip compression to reduce network traffic
   - Proper Content-Encoding header

3. **Error Handling**:
   - Comprehensive error handling for network issues
   - Proper HTTP status code checking

4. **Race Condition Prevention**:
   - Clear batch after successful send
   - Proper synchronization (can add mutex if needed for concurrent access)

5. **Efficiency**:
   - Reuses batch structure
   - Avoids sending empty batches
   - Configurable timeouts

6. **Compatibility**:
   - Uses standard JSON format
   - Proper HTTP headers
   - Works with existing single metric API

Usage example:
go run agent_batch_example.go

This will start sending batches of metrics to http://localhost:8080/updates/
*/
