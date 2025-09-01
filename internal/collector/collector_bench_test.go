package collector_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mutualEvg/metrics-server/internal/collector"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

// BenchmarkRuntimeMetricCollection benchmarks runtime metric collection
func BenchmarkRuntimeMetricCollection(b *testing.B) {
	var memStats runtime.MemStats

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.ReadMemStats(&memStats)

		// Simulate reading key metrics
		_ = memStats.Alloc
		_ = memStats.TotalAlloc
		_ = memStats.Sys
		_ = memStats.NumGC
		_ = memStats.HeapAlloc
		_ = memStats.HeapInuse
	}
}

// BenchmarkMetricChannelOperations benchmarks metric channel send/receive
func BenchmarkMetricChannelOperations(b *testing.B) {
	ch := make(chan worker.MetricData, 1000)

	// Start a goroutine to consume from channel
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
		case ch <- worker.MetricData{
			Type: "runtime",
		}:
		default:
			// Channel full, skip
		}
	}

	close(ch)
	<-done
}

// BenchmarkCollectorCreation benchmarks creating new collectors
func BenchmarkCollectorCreation(b *testing.B) {
	retryConfig := retry.RetryConfig{
		MaxAttempts: 3,
		Intervals:   []time.Duration{100 * time.Millisecond, 1 * time.Second},
	}

	var pollCount int64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workerPool := worker.NewPool(3, "http://localhost:8080", "", retryConfig)
		collector.New(
			workerPool,
			2*time.Second,
			10*time.Second,
			10,
			"http://localhost:8080",
			"",
			retryConfig,
			&pollCount,
		)
	}
}

// BenchmarkAtomicOperations benchmarks atomic operations used in collector
func BenchmarkAtomicOperations(b *testing.B) {
	var counter int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&counter, 1)
		}
	})
}

// BenchmarkChannelWithBuffer benchmarks buffered channel performance
func BenchmarkChannelWithBuffer(b *testing.B) {
	ch := make(chan int, 100)

	// Consumer goroutine
	go func() {
		for range ch {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		select {
		case ch <- i:
		default:
		}
	}

	close(ch)
}

// BenchmarkGoroutineSpawn benchmarks goroutine creation overhead
func BenchmarkGoroutineSpawn(b *testing.B) {
	ch := make(chan bool, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func() {
			ch <- true
		}()
	}

	for i := 0; i < b.N; i++ {
		<-ch
	}
}

// BenchmarkContextOperations benchmarks context operations
func BenchmarkContextOperations(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		select {
		case <-ctx.Done():
		default:
		}
		cancel()
	}
}
