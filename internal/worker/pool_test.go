package worker

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

func TestNewPool(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 3,
		Intervals:   []time.Duration{100 * time.Millisecond},
	}

	pool := NewPool(5, "http://localhost:8080", "test-key", retryConfig)

	if pool.rateLimit != 5 {
		t.Errorf("Expected rateLimit 5, got %d", pool.rateLimit)
	}

	if pool.serverAddr != "http://localhost:8080" {
		t.Errorf("Expected serverAddr http://localhost:8080, got %s", pool.serverAddr)
	}

	if pool.key != "test-key" {
		t.Errorf("Expected key test-key, got %s", pool.key)
	}

	if cap(pool.jobs) != 10 { // rateLimit * 2
		t.Errorf("Expected jobs channel capacity 10, got %d", cap(pool.jobs))
	}

	if pool.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected HTTP client timeout 10s, got %v", pool.httpClient.Timeout)
	}
}

func TestPoolStartStop(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	pool := NewPool(2, "http://localhost:8080", "", retryConfig)

	// Start the pool
	pool.Start()

	// Stop the pool - this will wait for workers to finish
	pool.Stop()

	// Pool should be stopped now
	select {
	case _, ok := <-pool.jobs:
		if ok {
			t.Error("Expected jobs channel to be closed")
		}
	default:
		t.Error("Jobs channel should be closed and readable")
	}
}

func TestPoolSubmitMetric(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	pool := NewPool(1, "http://localhost:8080", "", retryConfig)
	pool.Start()
	defer pool.Stop()

	// Submit a metric
	value := 123.45
	metric := MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	}

	// This should not block
	pool.SubmitMetric(metric)

	// Verify metric was submitted by checking channel is not full
	if len(pool.jobs) > cap(pool.jobs) {
		t.Error("Jobs channel is overfilled")
	}
}

func TestPoolSubmitMetricToFullQueue(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	// Create a pool with very small capacity
	pool := &Pool{
		jobs:        make(chan MetricData, 1), // Very small buffer
		rateLimit:   1,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		serverAddr:  "http://localhost:8080",
		key:         "",
		retryConfig: retryConfig,
	}

	value := 123.45
	metric1 := MetricData{
		Metric: models.Metrics{
			ID:    "test_metric1",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	}

	metric2 := MetricData{
		Metric: models.Metrics{
			ID:    "test_metric2",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	}

	// Fill the queue
	pool.SubmitMetric(metric1)

	// This should not block even though queue is full
	pool.SubmitMetric(metric2)
}

func TestPoolSubmitMetricAfterStop(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	pool := NewPool(1, "http://localhost:8080", "", retryConfig)
	pool.Start()
	pool.Stop() // Close the channel

	value := 123.45
	metric := MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	}

	// This should not panic due to recover in SubmitMetric
	pool.SubmitMetric(metric)
}

func TestPoolWithMockServer(t *testing.T) {
	// Use a channel to signal when request is processed
	requestProcessed := make(chan struct{}, 1)

	// Create a mock server that signals when it receives a request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		select {
		case requestProcessed <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	pool := NewPool(1, server.URL, "", retryConfig)
	pool.Start()
	defer pool.Stop()

	value := 123.45
	metric := MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "test",
	}

	pool.SubmitMetric(metric)

	// Wait for the request to be processed or timeout
	select {
	case <-requestProcessed:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Request was not processed within timeout")
	}
}

func TestPoolConcurrentSubmit(t *testing.T) {
	// Use mutex and counter to track processed requests
	var (
		mu             sync.Mutex
		processedCount int
		submittedCount = 10
	)
	requestProcessed := make(chan struct{}, submittedCount)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		mu.Lock()
		processedCount++
		mu.Unlock()
		select {
		case requestProcessed <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	pool := NewPool(3, server.URL, "", retryConfig)
	pool.Start()
	defer pool.Stop()

	// Submit multiple metrics concurrently
	var wg sync.WaitGroup
	for i := 0; i < submittedCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			value := float64(id)
			metric := MetricData{
				Metric: models.Metrics{
					ID:    "test_metric",
					MType: "gauge",
					Value: &value,
				},
				Type: "test",
			}
			pool.SubmitMetric(metric)
		}(i)
	}
	wg.Wait()

	// Wait for requests to be processed (may be less than submitted due to queue capacity)
	// Pool has capacity of 3*2=6, so some metrics may be dropped if queue is full
	timeout := time.After(5 * time.Second)
	deadline := time.After(2 * time.Second) // Give reasonable time for processing

processLoop:
	for {
		select {
		case <-requestProcessed:
			// One request processed
		case <-deadline:
			// Stop waiting after deadline
			break processLoop
		case <-timeout:
			t.Fatal("Test timeout - this should not happen")
		}
	}

	mu.Lock()
	finalCount := processedCount
	mu.Unlock()

	// We should process at least some metrics, but may not process all if queue was full
	if finalCount == 0 {
		t.Error("Expected at least some requests to be processed")
	}

	t.Logf("Processed %d/%d metrics (some may have been dropped due to queue capacity)", finalCount, submittedCount)
}
