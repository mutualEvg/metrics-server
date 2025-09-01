package worker

import (
	"net/http"
	"net/http/httptest"
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

	// Give workers time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the pool
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

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)
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
	// Create a mock server that always returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
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

	// Give time for the request to be processed
	time.Sleep(200 * time.Millisecond)
}
