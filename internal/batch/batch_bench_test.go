package batch_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mutualEvg/metrics-server/internal/batch"
	"github.com/mutualEvg/metrics-server/internal/models"
)

// BenchmarkBatchAddGauge benchmarks adding gauge metrics to batch
func BenchmarkBatchAddGauge(b *testing.B) {
	batch := batch.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batch.AddGauge(fmt.Sprintf("gauge_%d", i%1000), float64(i))
			i++
		}
	})
}

// BenchmarkBatchAddCounter benchmarks adding counter metrics to batch
func BenchmarkBatchAddCounter(b *testing.B) {
	batch := batch.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batch.AddCounter(fmt.Sprintf("counter_%d", i%1000), int64(i))
			i++
		}
	})
}

// BenchmarkBatchGetAndClear benchmarks getting and clearing batch
func BenchmarkBatchGetAndClear(b *testing.B) {
	batch := batch.New()

	// Pre-populate with data
	for i := 0; i < 100; i++ {
		batch.AddGauge(fmt.Sprintf("gauge_%d", i), float64(i))
		batch.AddCounter(fmt.Sprintf("counter_%d", i), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := batch.GetAndClear()
		// Re-populate for next iteration
		if len(metrics) == 0 {
			for j := 0; j < 100; j++ {
				batch.AddGauge(fmt.Sprintf("gauge_%d", j), float64(j))
				batch.AddCounter(fmt.Sprintf("counter_%d", j), int64(j))
			}
		}
	}
}

// BenchmarkJSONMarshal benchmarks JSON marshaling of metrics
func BenchmarkJSONMarshal(b *testing.B) {
	metrics := make([]models.Metrics, 100)
	for i := 0; i < 100; i++ {
		value := float64(i)
		delta := int64(i)
		if i%2 == 0 {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("gauge_%d", i),
				MType: "gauge",
				Value: &value,
			}
		} else {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("counter_%d", i),
				MType: "counter",
				Delta: &delta,
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(metrics)
	}
}

// BenchmarkGzipCompress benchmarks gzip compression of JSON data
func BenchmarkGzipCompress(b *testing.B) {
	metrics := make([]models.Metrics, 100)
	for i := 0; i < 100; i++ {
		value := float64(i)
		delta := int64(i)
		if i%2 == 0 {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("gauge_%d", i),
				MType: "gauge",
				Value: &value,
			}
		} else {
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("counter_%d", i),
				MType: "counter",
				Delta: &delta,
			}
		}
	}

	jsonData, _ := json.Marshal(metrics)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		gzipWriter.Write(jsonData)
		gzipWriter.Close()
	}
}

// BenchmarkBatchMixedOperations benchmarks mixed batch operations
func BenchmarkBatchMixedOperations(b *testing.B) {
	batch := batch.New()

	// Pre-compute metric names to avoid string formatting during benchmark
	gaugeNames := make([]string, 100)
	counterNames := make([]string, 100)
	for i := 0; i < 100; i++ {
		gaugeNames[i] = fmt.Sprintf("gauge_%d", i)
		counterNames[i] = fmt.Sprintf("counter_%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % 100
			if i%3 == 0 {
				batch.AddGauge(gaugeNames[idx], float64(i))
			} else if i%3 == 1 {
				batch.AddCounter(counterNames[idx], int64(i))
			} else {
				metrics := batch.GetAndClear()
				if len(metrics) > 0 {
					// Simulate processing
					json.Marshal(metrics)
				}
			}
			i++
		}
	})
}
