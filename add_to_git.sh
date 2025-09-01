cd /Users/ar11/Downloads/10_golang/metrics-server

echo "📊 Adding benchmark files..."
git add internal/storage/storage_bench_test.go
git add internal/hash/hash_bench_test.go  
git add internal/batch/batch_bench_test.go
git add internal/handlers/handlers_bench_test.go
git add internal/worker/worker_bench_test.go
git add internal/collector/collector_bench_test.go
git add internal/middleware/middleware_bench_test.go

echo "🔧 Adding benchmark infrastructure..."
git add cmd/benchmark/main.go
git add run_benchmarks.sh

echo "⚡ Adding performance optimizations..."
git add internal/batch/batch.go
git add storage/storage.go

echo "📈 Adding memory profiles..."
git add profiles/

echo "✅ Committing all benchmark and optimization work..."
git commit -m "feat: Add comprehensive benchmarks and memory optimization"
