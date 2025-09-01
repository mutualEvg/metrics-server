#!/bin/bash

set -e

echo "🚀 Starting Metrics Server Benchmarks and Memory Profiling"
echo "=========================================================="

# Create profiles directory
mkdir -p profiles

# Function to run benchmarks
run_benchmarks() {
    echo "📊 Running benchmarks for key components..."
    
    echo "  - Storage benchmarks..."
    go test -bench=BenchmarkUpdateGauge -benchtime=5s -cpuprofile=profiles/cpu_storage.prof ./internal/storage/... -v
    
    echo "  - Hash benchmarks..."
    go test -bench=BenchmarkCalculateHash -benchtime=5s ./internal/hash/... -v
    
    echo "  - Batch benchmarks..."
    go test -bench=BenchmarkBatchAdd -benchtime=5s ./internal/batch/... -v
    
    echo "  - Handler benchmarks..."
    go test -bench=BenchmarkUpdateJSONHandler -benchtime=3s ./internal/handlers/... -v
    
    echo "  - Worker benchmarks..."
    go test -bench=BenchmarkJSONMarshaling -benchtime=3s ./internal/worker/... -v
    
    echo "  - Collector benchmarks..."
    go test -bench=BenchmarkRuntimeMetricCollection -benchtime=3s ./internal/collector/... -v
    
    echo "  - Middleware benchmarks..."
    go test -bench=BenchmarkGzipMiddleware -benchtime=3s ./internal/middleware/... -v
}

# Function to generate memory profile using benchmark tool
generate_base_profile() {
    echo "📈 Generating base memory profile..."
    
    echo "Building benchmark tool..."
    go build -o cmd/benchmark/benchmark cmd/benchmark/main.go
    
    echo "Running profiling workload for 30 seconds..."
    timeout 35s ./cmd/benchmark/benchmark -profile-duration=30s -profile-type=mem -output=profiles/base.pprof &
    BENCHMARK_PID=$!
    
    # Give it time to start
    sleep 2
    
    # Run additional memory-intensive operations while profiling
    echo "Running additional workload..."
    go test -bench=BenchmarkMixedOperations -benchtime=10s ./internal/storage/... &
    go test -bench=BenchmarkBatchMixedOperations -benchtime=10s ./internal/batch/... &
    go test -bench=BenchmarkGzipMiddlewareLargePayload -benchtime=10s ./internal/middleware/... &
    
    # Wait for profiling to complete
    wait $BENCHMARK_PID 2>/dev/null || true
    
    # Wait for benchmark tests to complete
    wait
    
    echo "✅ Base profile generated: profiles/base.pprof"
}

# Function to analyze profile
analyze_profile() {
    if [ ! -f "profiles/base.pprof" ]; then
        echo "❌ Base profile not found!"
        return 1
    fi
    
    echo "🔍 Analyzing base memory profile..."
    echo "Top memory consumers:"
    go tool pprof -top -flat profiles/base.pprof | head -15
    
    echo ""
    echo "Memory allocation sources:"
    go tool pprof -list . profiles/base.pprof | head -20
}

# Main execution
main() {
    echo "Starting benchmarks and profiling process..."
    
    # Step 1: Run initial benchmarks
    run_benchmarks
    
    echo ""
    echo "=========================================================="
    
    # Step 2: Generate base profile
    generate_base_profile
    
    echo ""
    echo "=========================================================="
    
    # Step 3: Analyze the profile
    analyze_profile
    
    echo ""
    echo "✅ Base profiling complete!"
    echo "📁 Profile saved to: profiles/base.pprof"
    echo ""
    echo "🔧 Next steps:"
    echo "   1. Analyze the profile output above"
    echo "   2. Optimize identified inefficient code"
    echo "   3. Run this script again to generate result.pprof"
    echo "   4. Compare profiles with: go tool pprof -diff_base=profiles/base.pprof profiles/result.pprof"
}

# Check if this is a result profile generation
if [ "$1" = "result" ]; then
    echo "🔄 Generating result profile after optimizations..."
    generate_base_profile
    mv profiles/base.pprof profiles/result.pprof
    echo "✅ Result profile generated: profiles/result.pprof"
    
    if [ -f "profiles/base.pprof" ]; then
        echo ""
        echo "🔍 Comparing profiles..."
        echo "Difference between base and result profiles:"
        go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof | head -15
    fi
else
    main
fi
