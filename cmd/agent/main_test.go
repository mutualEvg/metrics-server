package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

func TestCollectRuntimeMetrics_With_Channels(t *testing.T) {
	// Set poll interval for test
	oldPollInterval := pollInterval
	pollInterval = 100 * time.Millisecond
	defer func() { pollInterval = oldPollInterval }()

	// Create worker pool and metric collector
	workerPool := NewMetricsWorkerPool(2)
	workerPool.Start()
	defer workerPool.Stop()

	metricCollector := NewMetricCollector(workerPool)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start metric collection
	go metricCollector.collectRuntimeMetrics(ctx)

	// Collect metrics from the runtime channel
	time.Sleep(pollInterval + 200*time.Millisecond)

	// Check if we received some metrics
	receivedCount := 0
	timeout := time.After(1 * time.Second)

	for receivedCount < 5 { // Expect at least 5 metrics
		select {
		case metric := <-metricCollector.runtimeChan:
			receivedCount++
			if metric.Metric.ID == "RandomValue" {
				t.Logf("Received RandomValue metric: %v", *metric.Metric.Value)
			}
			if metric.Metric.ID == "Alloc" {
				t.Logf("Received Alloc metric: %v", *metric.Metric.Value)
			}
		case <-timeout:
			t.Errorf("Timeout waiting for metrics, received %d", receivedCount)
			return
		}
	}

	if receivedCount < 5 {
		t.Errorf("Expected at least 5 metrics, received %d", receivedCount)
	}
}

func TestCollectSystemMetrics_With_Channels(t *testing.T) {
	// Set poll interval for test
	oldPollInterval := pollInterval
	pollInterval = 100 * time.Millisecond
	defer func() { pollInterval = oldPollInterval }()

	// Create worker pool and metric collector
	workerPool := NewMetricsWorkerPool(2)
	workerPool.Start()
	defer workerPool.Stop()

	metricCollector := NewMetricCollector(workerPool)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start system metric collection
	go metricCollector.collectSystemMetrics(ctx)

	// Wait for metrics to be collected
	time.Sleep(pollInterval + 2*time.Second)

	// Check if we received system metrics
	receivedCount := 0
	foundTotalMemory := false
	foundFreeMemory := false
	foundCPU := false

	timeout := time.After(1 * time.Second)

collectionLoop:
	for receivedCount < 10 { // Expect several system metrics
		select {
		case metric := <-metricCollector.systemChan:
			receivedCount++
			if metric.Metric.ID == "TotalMemory" {
				foundTotalMemory = true
			}
			if metric.Metric.ID == "FreeMemory" {
				foundFreeMemory = true
			}
			if metric.Metric.ID == "CPUutilization1" {
				foundCPU = true
			}
		case <-timeout:
			break collectionLoop
		}
	}

	if !foundTotalMemory {
		t.Error("TotalMemory should be present in system metrics")
	}
	if !foundFreeMemory {
		t.Error("FreeMemory should be present in system metrics")
	}
	if !foundCPU {
		t.Error("CPUutilization1 should be present in system metrics")
	}
}

func TestMetricsWorkerPool(t *testing.T) {
	// Set up retry config for test
	oldRetryConfig := retryConfig
	retryConfig = retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}
	defer func() { retryConfig = oldRetryConfig }()

	// Create test server
	receivedMetrics := make([]models.Metrics, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("Expected Content-Encoding gzip, got %s", r.Header.Get("Content-Encoding"))
		}

		// Read compressed body
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("Failed to create gzip reader: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			t.Errorf("Failed to read compressed body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse JSON
		var metric models.Metrics
		if err := json.Unmarshal(body, &metric); err != nil {
			t.Errorf("Failed to parse JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedMetrics = append(receivedMetrics, metric)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set server address for test
	oldAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = oldAddress }()

	// Create worker pool
	pool := NewMetricsWorkerPool(2)
	pool.Start()
	defer pool.Stop()

	// Submit test metrics
	testValue := 123.45
	testDelta := int64(42)

	gaugeMetric := MetricData{
		Metric: models.Metrics{
			ID:    "TestGauge",
			MType: "gauge",
			Value: &testValue,
		},
		Type: "test",
	}

	counterMetric := MetricData{
		Metric: models.Metrics{
			ID:    "TestCounter",
			MType: "counter",
			Delta: &testDelta,
		},
		Type: "test",
	}

	pool.SubmitMetric(gaugeMetric)
	pool.SubmitMetric(counterMetric)

	// Wait for metrics to be sent
	time.Sleep(2 * time.Second)

	// Verify metrics were received
	if len(receivedMetrics) != 2 {
		t.Errorf("Expected 2 metrics, received %d", len(receivedMetrics))
	}

	// Check individual metrics
	foundGauge := false
	foundCounter := false
	for _, metric := range receivedMetrics {
		if metric.ID == "TestGauge" && metric.MType == "gauge" && metric.Value != nil && *metric.Value == testValue {
			foundGauge = true
		}
		if metric.ID == "TestCounter" && metric.MType == "counter" && metric.Delta != nil && *metric.Delta == testDelta {
			foundCounter = true
		}
	}

	if !foundGauge {
		t.Error("TestGauge metric not received correctly")
	}
	if !foundCounter {
		t.Error("TestCounter metric not received correctly")
	}
}

func TestMetricsBatch(t *testing.T) {
	batch := NewMetricsBatch()

	// Add metrics to batch
	batch.AddGauge("TestGauge", 123.45)
	batch.AddCounter("TestCounter", 42)

	// Get metrics from batch
	metrics := batch.GetAndClear()

	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics in batch, got %d", len(metrics))
	}

	// Verify metrics
	foundGauge := false
	foundCounter := false
	for _, metric := range metrics {
		if metric.ID == "TestGauge" && metric.MType == "gauge" && metric.Value != nil && *metric.Value == 123.45 {
			foundGauge = true
		}
		if metric.ID == "TestCounter" && metric.MType == "counter" && metric.Delta != nil && *metric.Delta == 42 {
			foundCounter = true
		}
	}

	if !foundGauge {
		t.Error("TestGauge not found in batch")
	}
	if !foundCounter {
		t.Error("TestCounter not found in batch")
	}

	// Verify batch is cleared
	metrics2 := batch.GetAndClear()
	if metrics2 != nil {
		t.Error("Batch should be empty after GetAndClear")
	}
}

func TestSendBatch(t *testing.T) {
	// Set up retry config for test
	oldRetryConfig := retryConfig
	retryConfig = retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}
	defer func() { retryConfig = oldRetryConfig }()

	// Create test server
	receivedBatch := make([]models.Metrics, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/updates/" {
			t.Errorf("Expected /updates/ path, got %s", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Read compressed body
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("Failed to create gzip reader: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			t.Errorf("Failed to read compressed body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse JSON
		if err := json.Unmarshal(body, &receivedBatch); err != nil {
			t.Errorf("Failed to parse JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set server address for test
	oldAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = oldAddress }()

	// Create test batch
	testValue := 123.45
	testDelta := int64(42)

	metrics := []models.Metrics{
		{
			ID:    "TestGauge",
			MType: "gauge",
			Value: &testValue,
		},
		{
			ID:    "TestCounter",
			MType: "counter",
			Delta: &testDelta,
		},
	}

	// Send batch
	err := sendBatch(metrics)
	if err != nil {
		t.Errorf("sendBatch failed: %v", err)
	}

	// Verify batch was received
	if len(receivedBatch) != 2 {
		t.Errorf("Expected 2 metrics in received batch, got %d", len(receivedBatch))
	}
}

func TestMetricCollector_Channel_Communication(t *testing.T) {
	// Set test mode to prevent race conditions during shutdown
	oldTestMode := os.Getenv("TEST_MODE")
	os.Setenv("TEST_MODE", "true")
	defer func() {
		if oldTestMode == "" {
			os.Unsetenv("TEST_MODE")
		} else {
			os.Setenv("TEST_MODE", oldTestMode)
		}
	}()

	// Set up retry config for test
	oldRetryConfig := retryConfig
	retryConfig = retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}
	defer func() { retryConfig = oldRetryConfig }()

	// Set intervals for test
	oldPollInterval := pollInterval
	oldReportInterval := reportInterval
	pollInterval = 200 * time.Millisecond // Slower to prevent queue overflow
	reportInterval = 500 * time.Millisecond
	defer func() {
		pollInterval = oldPollInterval
		reportInterval = oldReportInterval
	}()

	// Create a test server that accepts requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set server address for test
	oldAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = oldAddress }()

	// Create worker pool and metric collector
	workerPool := NewMetricsWorkerPool(5) // Larger pool to handle load
	workerPool.Start()
	defer workerPool.Stop()

	metricCollector := NewMetricCollector(workerPool)

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	// Start the metric collector
	metricCollector.Start(ctx)

	// Let it run for a bit
	time.Sleep(600 * time.Millisecond)

	// The test passes if no panics occur and goroutines start/stop cleanly
	t.Log("Channel-based metric collection completed successfully")
}
