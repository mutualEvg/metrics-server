package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

func TestCollectRuntimeMetrics(t *testing.T) {
	// Set poll interval for test
	oldPollInterval := pollInterval
	pollInterval = 100 * time.Millisecond
	defer func() { pollInterval = oldPollInterval }()

	var metrics sync.Map
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start collection in goroutine
	go collectRuntimeMetrics(ctx, &metrics)

	// Wait a bit for metrics to be collected
	time.Sleep(pollInterval + 500*time.Millisecond)

	// Check if metrics were collected
	foundRandom := false
	foundAlloc := false
	metrics.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			t.Errorf("Metric key should be string, got %T", key)
			return false
		}

		_, ok = value.(float64)
		if !ok {
			t.Errorf("Metric value should be float64, got %T", value)
			return false
		}

		if name == "RandomValue" {
			foundRandom = true
		}
		if name == "Alloc" {
			foundAlloc = true
		}
		return true
	})

	if !foundRandom {
		t.Error("RandomValue should be present in metrics")
	}
	if !foundAlloc {
		t.Error("Alloc should be present in metrics")
	}
}

func TestCollectSystemMetrics(t *testing.T) {
	// Set poll interval for test
	oldPollInterval := pollInterval
	pollInterval = 100 * time.Millisecond
	defer func() { pollInterval = oldPollInterval }()

	var metrics sync.Map
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start collection in goroutine
	go collectSystemMetrics(ctx, &metrics)

	// Wait for metrics to be collected
	time.Sleep(pollInterval + 2*time.Second)

	// Check if system metrics were collected
	foundTotalMemory := false
	foundFreeMemory := false
	foundCPU := false

	metrics.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			t.Errorf("Metric key should be string, got %T", key)
			return false
		}

		_, ok = value.(float64)
		if !ok {
			t.Errorf("Metric value should be float64, got %T", value)
			return false
		}

		if name == "TotalMemory" {
			foundTotalMemory = true
		}
		if name == "FreeMemory" {
			foundFreeMemory = true
		}
		if name == "CPUutilization1" {
			foundCPU = true
		}
		return true
	})

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
