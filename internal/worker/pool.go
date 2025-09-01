package worker

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mutualEvg/metrics-server/internal/hash"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

// MetricData represents a single metric to be sent
type MetricData struct {
	Metric models.Metrics
	Type   string // "runtime", "system", or other types
}

// Pool manages concurrent metric sending
type Pool struct {
	jobs        chan MetricData
	wg          sync.WaitGroup
	rateLimit   int
	httpClient  *http.Client
	serverAddr  string
	key         string // Key for SHA256 signature
	retryConfig retry.RetryConfig
}

// NewPool creates a new worker pool
func NewPool(rateLimit int, serverAddr, key string, retryConfig retry.RetryConfig) *Pool {
	return &Pool{
		jobs:        make(chan MetricData, rateLimit*2), // Buffer to prevent blocking
		rateLimit:   rateLimit,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		serverAddr:  serverAddr,
		key:         key,
		retryConfig: retryConfig,
	}
}

// Start initializes the worker pool
func (p *Pool) Start() {
	for i := 0; i < p.rateLimit; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Started worker pool with %d workers", p.rateLimit)
}

// Stop gracefully shuts down the worker pool
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	log.Printf("Worker pool stopped")
}

// SubmitMetric adds a metric to the sending queue
func (p *Pool) SubmitMetric(metric MetricData) {
	// Recover from panic if channel is closed
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker pool channel closed, dropping metric: %s", metric.Metric.ID)
		}
	}()

	select {
	case p.jobs <- metric:
		// Metric submitted successfully
	default:
		log.Printf("Worker pool queue full, dropping metric: %s", metric.Metric.ID)
	}
}

// worker processes metrics from the queue
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	log.Printf("Worker %d started", id)

	for metric := range p.jobs {
		p.sendMetric(metric)
	}

	log.Printf("Worker %d stopped", id)
}

// sendMetric sends a single metric to the server
func (p *Pool) sendMetric(metricData MetricData) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := retry.Do(ctx, p.retryConfig, func() error {
		jsonData, err := json.Marshal(metricData.Metric)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Compress the JSON data
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		_, err = gzipWriter.Write(jsonData)
		if err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		err = gzipWriter.Close()
		if err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		url := fmt.Sprintf("%s/update/", p.serverAddr)
		req, err := http.NewRequest(http.MethodPost, url, &compressedData)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		// Add hash header if key is configured
		if p.key != "" {
			hashValue := hash.CalculateHash(compressedData.Bytes(), p.key)
			req.Header.Set("HashSHA256", hashValue)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send metric: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned non-OK status: %s", resp.Status)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to send %s metric %s after retries: %v", metricData.Type, metricData.Metric.ID, err)
	}
}
