package batch

import (
	"testing"
)

func TestNew(t *testing.T) {
	batcher := New()

	if batcher.metrics == nil {
		t.Error("Metrics slice should be initialized")
	}
}

func TestBatchAddGauge(t *testing.T) {
	batcher := New()

	// Add gauge metric
	batcher.AddGauge("test1", 123.45)
	batcher.AddGauge("test2", 67.89)

	// Check the batch has metrics
	metrics := batcher.GetAndClear()
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics in batch, got %d", len(metrics))
	}

	// Verify first metric
	if metrics[0].ID != "test1" || metrics[0].MType != "gauge" || *metrics[0].Value != 123.45 {
		t.Errorf("First metric incorrect: %+v", metrics[0])
	}

	// Verify second metric
	if metrics[1].ID != "test2" || metrics[1].MType != "gauge" || *metrics[1].Value != 67.89 {
		t.Errorf("Second metric incorrect: %+v", metrics[1])
	}
}

func TestBatchAddCounter(t *testing.T) {
	batcher := New()

	// Add counter metrics
	batcher.AddCounter("requests", 100)
	batcher.AddCounter("errors", 5)

	// Check the batch has metrics
	metrics := batcher.GetAndClear()
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics in batch, got %d", len(metrics))
	}

	// Verify first metric
	if metrics[0].ID != "requests" || metrics[0].MType != "counter" || *metrics[0].Delta != 100 {
		t.Errorf("First metric incorrect: %+v", metrics[0])
	}

	// Verify second metric
	if metrics[1].ID != "errors" || metrics[1].MType != "counter" || *metrics[1].Delta != 5 {
		t.Errorf("Second metric incorrect: %+v", metrics[1])
	}
}

func TestBatchMixed(t *testing.T) {
	batcher := New()

	// Add mixed metrics
	batcher.AddGauge("cpu", 45.3)
	batcher.AddCounter("requests", 100)
	batcher.AddGauge("memory", 67.8)

	// Check the batch has metrics
	metrics := batcher.GetAndClear()
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics in batch, got %d", len(metrics))
	}

	// Should have both gauge and counter metrics
	gaugeCount := 0
	counterCount := 0
	for _, metric := range metrics {
		switch metric.MType {
		case "gauge":
			gaugeCount++
		case "counter":
			counterCount++
		}
	}

	if gaugeCount != 2 {
		t.Errorf("Expected 2 gauge metrics, got %d", gaugeCount)
	}

	if counterCount != 1 {
		t.Errorf("Expected 1 counter metric, got %d", counterCount)
	}
}

func TestBatchGetAndClear(t *testing.T) {
	batcher := New()

	// Add some metrics
	batcher.AddGauge("test1", 123.45)
	batcher.AddCounter("test2", 100)

	// Get the batch
	batch := batcher.GetAndClear()

	if len(batch) != 2 {
		t.Errorf("Expected batch of 2 metrics, got %d", len(batch))
	}

	// Verify metric contents
	if batch[0].ID != "test1" || batch[0].MType != "gauge" {
		t.Errorf("First metric incorrect: %+v", batch[0])
	}

	if batch[1].ID != "test2" || batch[1].MType != "counter" {
		t.Errorf("Second metric incorrect: %+v", batch[1])
	}

	// After GetAndClear, should return empty on next call
	batch2 := batcher.GetAndClear()
	if batch2 != nil {
		t.Errorf("Expected nil after second GetAndClear, got %d metrics", len(batch2))
	}
}

func TestBatchEmptyGetAndClear(t *testing.T) {
	batcher := New()

	batch := batcher.GetAndClear()

	if batch != nil {
		t.Errorf("Expected nil for empty batch, got %d metrics", len(batch))
	}
}

func TestBatchConcurrency(t *testing.T) {
	batcher := New()

	// Add metrics concurrently
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			batcher.AddGauge("gauge_metric", float64(i))
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			batcher.AddCounter("counter_metric", int64(i))
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	batch := batcher.GetAndClear()
	if len(batch) != 100 {
		t.Errorf("Expected 100 metrics after concurrent adds, got %d", len(batch))
	}
}
