package worker_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

// BenchmarkWorkerPoolCreation benchmarks worker pool creation
func BenchmarkWorkerPoolCreation(b *testing.B) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 3,
		Intervals:   []time.Duration{100 * time.Millisecond, 1 * time.Second},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool := worker.NewPool(5, "http://localhost:8080", "test-key", retryConfig)
		_ = pool
	}
}

// BenchmarkMetricSubmission benchmarks metric submission to worker pool
func BenchmarkMetricSubmission(b *testing.B) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,                 // Minimal retries for benchmark
		Intervals:   []time.Duration{}, // No retry intervals for minimal latency
	}

	pool := worker.NewPool(5, "http://localhost:8080", "test-key", retryConfig)

	value := 123.45
	metric := worker.MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "runtime",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.SubmitMetric(metric)
	}
}

// BenchmarkJSONMarshaling benchmarks JSON marshaling performance
func BenchmarkJSONMarshaling(b *testing.B) {
	value := 123.45
	metric := models.Metrics{
		ID:    "test_metric_with_long_name_for_benchmarking",
		MType: "gauge",
		Value: &value,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(metric)
	}
}

// BenchmarkGzipCompression benchmarks gzip compression
func BenchmarkGzipCompression(b *testing.B) {
	value := 123.45
	metric := models.Metrics{
		ID:    "test_metric",
		MType: "gauge",
		Value: &value,
	}

	jsonData, _ := json.Marshal(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		gzipWriter.Write(jsonData)
		gzipWriter.Close()
	}
}

// BenchmarkHTTPRequestCreation benchmarks HTTP request creation
func BenchmarkHTTPRequestCreation(b *testing.B) {
	value := 123.45
	metric := models.Metrics{
		ID:    "test_metric",
		MType: "gauge",
		Value: &value,
	}

	jsonData, _ := json.Marshal(metric)

	// Compress data
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	gzipWriter.Write(jsonData)
	gzipWriter.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "http://localhost:8080/update/", bytes.NewReader(compressedData.Bytes()))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		_ = req
	}
}

// BenchmarkHTTPClientWithMockServer benchmarks HTTP client performance
func BenchmarkHTTPClientWithMockServer(b *testing.B) {
	// Create a mock HTTP server that responds quickly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 1 * time.Second}

	value := 123.45
	metric := models.Metrics{
		ID:    "test_metric",
		MType: "gauge",
		Value: &value,
	}

	jsonData, _ := json.Marshal(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", server.URL+"/update/", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// BenchmarkChannelOperations benchmarks channel operations in worker
func BenchmarkChannelOperations(b *testing.B) {
	ch := make(chan worker.MetricData, 100)

	value := 123.45
	metric := worker.MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "runtime",
	}

	// Start consumer
	done := make(chan bool)
	go func() {
		count := 0
		for range ch {
			count++
			if count >= b.N {
				break
			}
		}
		done <- true
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		select {
		case ch <- metric:
		default:
		}
	}

	close(ch)
	<-done
}

// BenchmarkConcurrentWorkers benchmarks performance with multiple workers
func BenchmarkConcurrentWorkers(b *testing.B) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Simulate processing time
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{}, // No retry intervals for minimal latency
	}

	pool := worker.NewPool(10, server.URL, "", retryConfig)
	pool.Start()
	defer pool.Stop()

	value := 123.45
	metric := worker.MetricData{
		Metric: models.Metrics{
			ID:    "test_metric",
			MType: "gauge",
			Value: &value,
		},
		Type: "runtime",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.SubmitMetric(metric)
		}
	})

	// Give workers time to process
	time.Sleep(100 * time.Millisecond)
}
