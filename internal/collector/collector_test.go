package collector

import (
	"context"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

func TestNew(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 3,
		Intervals:   []time.Duration{100 * time.Millisecond},
	}

	workerPool := worker.NewPool(5, "http://localhost:8080", "", retryConfig)

	var pollCount int64 = 0
	collector := New(workerPool, 2*time.Second, 10*time.Second, 10, "http://localhost:8080", "", retryConfig, &pollCount)

	if collector.pollInterval != 2*time.Second {
		t.Errorf("Expected pollInterval 2s, got %v", collector.pollInterval)
	}

	if collector.reportInterval != 10*time.Second {
		t.Errorf("Expected reportInterval 10s, got %v", collector.reportInterval)
	}

	if collector.batchSize != 10 {
		t.Errorf("Expected batchSize 10, got %d", collector.batchSize)
	}

	if collector.serverAddr != "http://localhost:8080" {
		t.Errorf("Expected serverAddr http://localhost:8080, got %s", collector.serverAddr)
	}

	if collector.pollCount != &pollCount {
		t.Error("PollCount pointer should be set")
	}
}

func TestCollectorChannels(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	workerPool := worker.NewPool(1, "http://localhost:8080", "", retryConfig)

	var pollCount int64 = 0
	collector := New(workerPool, 100*time.Millisecond, 200*time.Millisecond, 0, "http://localhost:8080", "", retryConfig, &pollCount)

	// Check that channels are accessible
	runtimeChan := collector.GetRuntimeChan()
	systemChan := collector.GetSystemChan()

	if runtimeChan == nil {
		t.Error("Runtime channel should not be nil")
	}

	if systemChan == nil {
		t.Error("System channel should not be nil")
	}
}

func TestCollectorStart(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	workerPool := worker.NewPool(1, "http://localhost:8080", "", retryConfig)
	workerPool.Start()
	defer workerPool.Stop()

	var pollCount int64 = 0
	collector := New(workerPool, 50*time.Millisecond, 100*time.Millisecond, 0, "http://localhost:8080", "", retryConfig, &pollCount)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Start the collector
	collector.Start(ctx)

	// Wait a bit for some metrics to be collected
	time.Sleep(150 * time.Millisecond)

	// Check that poll count increased
	if pollCount == 0 {
		t.Error("Poll count should have increased")
	}
}

func TestCollectorRuntimeMetrics(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	workerPool := worker.NewPool(1, "http://localhost:8080", "", retryConfig)

	var pollCount int64 = 0
	collector := New(workerPool, 50*time.Millisecond, 1*time.Second, 0, "http://localhost:8080", "", retryConfig, &pollCount)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start the collector
	collector.Start(ctx)

	// Wait for some metrics to be collected
	time.Sleep(100 * time.Millisecond)

	// Try to read from runtime channel (non-blocking)
	runtimeChan := collector.GetRuntimeChan()

	select {
	case metric := <-runtimeChan:
		// Check that we got a valid runtime metric
		if metric.Metric.ID == "" {
			t.Error("Metric ID should not be empty")
		}
		if metric.Metric.MType != "gauge" {
			t.Error("Runtime metrics should be gauge type")
		}
		if metric.Type != "runtime" {
			t.Error("Metric type should be 'runtime'")
		}
	case <-time.After(50 * time.Millisecond):
		// It's okay if no metrics are ready yet
		t.Log("No runtime metrics in channel yet")
	}
}

func TestCollectorSystemMetrics(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	workerPool := worker.NewPool(1, "http://localhost:8080", "", retryConfig)

	var pollCount int64 = 0
	collector := New(workerPool, 50*time.Millisecond, 1*time.Second, 0, "http://localhost:8080", "", retryConfig, &pollCount)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start the collector
	collector.Start(ctx)

	// Wait for some metrics to be collected
	time.Sleep(100 * time.Millisecond)

	// Try to read from system channel (non-blocking)
	systemChan := collector.GetSystemChan()

	select {
	case metric := <-systemChan:
		// Check that we got a valid system metric
		if metric.Metric.ID == "" {
			t.Error("Metric ID should not be empty")
		}
		if metric.Metric.MType != "gauge" {
			t.Error("System metrics should be gauge type")
		}
		if metric.Type != "system" {
			t.Error("Metric type should be 'system'")
		}
	case <-time.After(50 * time.Millisecond):
		// It's okay if no metrics are ready yet
		t.Log("No system metrics in channel yet")
	}
}

func TestCollectorBatchMode(t *testing.T) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,
		Intervals:   []time.Duration{},
	}

	workerPool := worker.NewPool(1, "http://localhost:8080", "", retryConfig)

	var pollCount int64 = 0
	// Enable batch mode with batchSize > 0
	collector := New(workerPool, 50*time.Millisecond, 100*time.Millisecond, 10, "http://localhost:8080", "", retryConfig, &pollCount)

	if collector.batchSize != 10 {
		t.Errorf("Expected batch size 10, got %d", collector.batchSize)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start the collector
	collector.Start(ctx)

	// Wait for at least one report interval
	time.Sleep(150 * time.Millisecond)

	// In batch mode, metrics should be processed differently
	// This is mainly a smoke test to ensure batch mode doesn't crash
}
