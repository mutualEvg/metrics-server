package storage_test

import (
	"strconv"
	"testing"

	"github.com/mutualEvg/metrics-server/storage"
)

// BenchmarkUpdateGauge benchmarks gauge update operations
func BenchmarkUpdateGauge(b *testing.B) {
	s := storage.NewMemStorage()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.UpdateGauge("test_gauge", float64(i%1000))
			i++
		}
	})
}

// BenchmarkUpdateCounter benchmarks counter update operations
func BenchmarkUpdateCounter(b *testing.B) {
	s := storage.NewMemStorage()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.UpdateCounter("test_counter", int64(i%100))
			i++
		}
	})
}

// BenchmarkGetGauge benchmarks gauge read operations
func BenchmarkGetGauge(b *testing.B) {
	s := storage.NewMemStorage()
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		s.UpdateGauge("gauge_"+string(rune(i)), float64(i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.GetGauge("gauge_" + string(rune(i%1000)))
			i++
		}
	})
}

// BenchmarkGetCounter benchmarks counter read operations
func BenchmarkGetCounter(b *testing.B) {
	s := storage.NewMemStorage()
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		s.UpdateCounter("counter_"+string(rune(i)), int64(i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.GetCounter("counter_" + string(rune(i%1000)))
			i++
		}
	})
}

// BenchmarkGetAll benchmarks getting all metrics
func BenchmarkGetAll(b *testing.B) {
	s := storage.NewMemStorage()
	// Pre-populate with data
	for i := 0; i < 100; i++ {
		s.UpdateGauge("gauge_"+string(rune(i)), float64(i))
		s.UpdateCounter("counter_"+string(rune(i)), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetAll()
	}
}

// BenchmarkMixedOperations benchmarks mixed read/write operations
func BenchmarkMixedOperations(b *testing.B) {
	s := storage.NewMemStorage()

	// Pre-compute metric names to avoid string concatenation during benchmark
	gaugeNames := make([]string, 100)
	counterNames := make([]string, 100)
	for i := 0; i < 100; i++ {
		gaugeNames[i] = "gauge_" + strconv.Itoa(i)
		counterNames[i] = "counter_" + strconv.Itoa(i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % 100
			if i%4 == 0 {
				s.UpdateGauge(gaugeNames[idx], float64(i))
			} else if i%4 == 1 {
				s.UpdateCounter(counterNames[idx], int64(i))
			} else if i%4 == 2 {
				s.GetGauge(gaugeNames[idx])
			} else {
				s.GetCounter(counterNames[idx])
			}
			i++
		}
	})
}
