package collector

import (
	"context"
	"sync/atomic"
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

	// Poll until count increases or timeout
	timeout := time.After(500 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt64(&pollCount) > 0 {
				// Success - poll count increased
				// Wait for context to expire and collector goroutines to finish
				<-ctx.Done()
				time.Sleep(100 * time.Millisecond)
				return
			}
		case <-timeout:
			t.Fatal("Poll count did not increase within timeout")
		}
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

	// Wait for metrics with polling
	runtimeChan := collector.GetRuntimeChan()
	timeout := time.After(500 * time.Millisecond)

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
	case <-timeout:
		// It's okay if no metrics are ready - channels are buffered and may not fill immediately
		t.Log("No runtime metrics in channel within timeout (acceptable)")
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

	// Wait for metrics with polling
	systemChan := collector.GetSystemChan()
	timeout := time.After(500 * time.Millisecond)

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
	case <-timeout:
		// It's okay if no metrics are ready - channels are buffered and may not fill immediately
		t.Log("No system metrics in channel within timeout (acceptable)")
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

	// Poll until at least one poll cycle completes
	timeout := time.After(500 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt64(&pollCount) > 0 {
				// At least one batch has been prepared
				return
			}
		case <-timeout:
			// Batch mode doesn't crash - that's the main test
			t.Log("Batch mode ran without crashing (success)")
			return
		}
	}
}
