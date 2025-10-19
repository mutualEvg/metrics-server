package hash_test

import (
	"bytes"
	"testing"

	"github.com/mutualEvg/metrics-server/internal/hash"
)

var testKey = "test-secret-key"

// BenchmarkCalculateHash benchmarks hash calculation with different data sizes
func BenchmarkCalculateHash(b *testing.B) {
	data := bytes.Repeat([]byte("test data for hashing"), 50) // ~1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.CalculateHash(data, testKey)
	}
}

// BenchmarkCalculateHashSmall benchmarks hash calculation with small data
func BenchmarkCalculateHashSmall(b *testing.B) {
	data := []byte("small data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.CalculateHash(data, testKey)
	}
}

// BenchmarkCalculateHashLarge benchmarks hash calculation with large data
func BenchmarkCalculateHashLarge(b *testing.B) {
	data := bytes.Repeat([]byte("large data for hashing benchmark test"), 1000) // ~40KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.CalculateHash(data, testKey)
	}
}

// BenchmarkVerifyHash benchmarks hash verification
func BenchmarkVerifyHash(b *testing.B) {
	data := bytes.Repeat([]byte("test data for verification"), 50) // ~1KB
	correctHash := hash.CalculateHash(data, testKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.VerifyHash(data, testKey, correctHash)
	}
}

// BenchmarkHashReader benchmarks the HashReader function
func BenchmarkHashReader(b *testing.B) {
	data := bytes.Repeat([]byte("reader data for hashing"), 50) // ~1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		hash.HashReader(reader, testKey)
	}
}

// BenchmarkParallelHashCalculation benchmarks parallel hash calculation
func BenchmarkParallelHashCalculation(b *testing.B) {
	data := bytes.Repeat([]byte("parallel hash test data"), 20) // ~500B

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hash.CalculateHash(data, testKey)
		}
	})
}
