package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // Import pprof for profiling
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/mutualEvg/metrics-server/internal/batch"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
	"github.com/mutualEvg/metrics-server/storage"
)

var (
	profileDuration = flag.Duration("profile-duration", 30*time.Second, "Duration to run profiling")
	profileType     = flag.String("profile-type", "mem", "Profile type: mem, cpu, goroutine, heap")
	outputFile      = flag.String("output", "profiles/profile.pprof", "Output profile file")
	serverAddr      = flag.String("server", "http://localhost:8080", "Metrics server address")
)

func main() {
	flag.Parse()

	log.Printf("Starting benchmark with profiling for %v", *profileDuration)

	// Start pprof server in background
	go func() {
		log.Printf("pprof server listening on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	// Wait a moment for pprof server to start
	time.Sleep(100 * time.Millisecond)

	// Start profiling
	if err := startProfile(*profileType, *outputFile); err != nil {
		log.Fatalf("Failed to start profiling: %v", err)
	}

	// Run intensive operations that simulate real workload
	ctx, cancel := context.WithTimeout(context.Background(), *profileDuration)
	defer cancel()

	var wg sync.WaitGroup

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start multiple workload goroutines
	wg.Add(4)

	// 1. Storage intensive operations
	go func() {
		defer wg.Done()
		runStorageWorkload(ctx)
	}()

	// 2. Batch processing operations
	go func() {
		defer wg.Done()
		runBatchWorkload(ctx)
	}()

	// 3. Worker pool operations
	go func() {
		defer wg.Done()
		runWorkerWorkload(ctx)
	}()

	// 4. Memory allocation intensive operations
	go func() {
		defer wg.Done()
		runMemoryIntensiveWorkload(ctx)
	}()

	// Wait for completion or signal
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		log.Println("Workload completed")
	case <-sigChan:
		log.Println("Received signal, stopping...")
		cancel()
		<-done
	case <-ctx.Done():
		log.Println("Context timeout, stopping...")
	}

	// Stop profiling
	if err := stopProfile(); err != nil {
		log.Printf("Failed to stop profiling: %v", err)
	}

	log.Printf("Profile saved to %s", *outputFile)
}

func startProfile(profileType, outputFile string) error {
	var url string
	switch profileType {
	case "mem":
		url = "http://localhost:6060/debug/pprof/heap"
	case "cpu":
		url = "http://localhost:6060/debug/pprof/profile"
	case "goroutine":
		url = "http://localhost:6060/debug/pprof/goroutine"
	case "heap":
		url = "http://localhost:6060/debug/pprof/heap"
	default:
		return fmt.Errorf("unknown profile type: %s", profileType)
	}

	// Create profiles directory if it doesn't exist
	os.MkdirAll("profiles", 0755)

	log.Printf("Starting %s profiling to %s", profileType, outputFile)

	// Start background profiling collection
	go func() {
		time.Sleep(*profileDuration)

		cmd := exec.Command("curl", "-s", url, "-o", outputFile)
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to collect profile: %v", err)
		}
	}()

	return nil
}

func stopProfile() error {
	// Profile collection is handled by the goroutine in startProfile
	return nil
}

// runStorageWorkload simulates intensive storage operations
func runStorageWorkload(ctx context.Context) {
	log.Println("Starting storage workload")
	defer log.Println("Storage workload completed")

	s := storage.NewMemStorage()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Intensive storage operations
			for i := 0; i < 1000; i++ {
				s.UpdateGauge(fmt.Sprintf("gauge_%d", i%100), float64(i))
				s.UpdateCounter(fmt.Sprintf("counter_%d", i%100), int64(i))

				if i%10 == 0 {
					s.GetGauge(fmt.Sprintf("gauge_%d", i%100))
					s.GetCounter(fmt.Sprintf("counter_%d", i%100))
				}

				if i%50 == 0 {
					s.GetAll()
				}
			}

			// Small delay to prevent overwhelming
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// runBatchWorkload simulates batch processing operations
func runBatchWorkload(ctx context.Context) {
	log.Println("Starting batch workload")
	defer log.Println("Batch workload completed")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			batch := batch.New()

			// Add many metrics to batch
			for i := 0; i < 100; i++ {
				batch.AddGauge(fmt.Sprintf("batch_gauge_%d", i), float64(i))
				batch.AddCounter(fmt.Sprintf("batch_counter_%d", i), int64(i))
			}

			// Get and clear batch (simulates processing)
			metrics := batch.GetAndClear()
			if len(metrics) > 0 {
				// Simulate JSON marshaling and compression
				for _, metric := range metrics {
					_ = metric.ID
					_ = metric.MType
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	}
}

// runWorkerWorkload simulates worker pool operations
func runWorkerWorkload(ctx context.Context) {
	log.Println("Starting worker workload")
	defer log.Println("Worker workload completed")

	retryConfig := retry.RetryConfig{
		MaxAttempts: 1,                 // Minimal retries to reduce latency
		Intervals:   []time.Duration{}, // No retry intervals for minimal latency
	}

	pool := worker.NewPool(5, *serverAddr, "test-key", retryConfig)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Create and submit metrics
			for i := 0; i < 50; i++ {
				value := float64(i)
				metric := worker.MetricData{
					Metric: models.Metrics{
						ID:    fmt.Sprintf("worker_metric_%d", i%10),
						MType: "gauge",
						Value: &value,
					},
					Type: "runtime",
				}

				pool.SubmitMetric(metric)
			}

			time.Sleep(10 * time.Millisecond)
		}
	}
}

// runMemoryIntensiveWorkload creates memory pressure to test allocation patterns
func runMemoryIntensiveWorkload(ctx context.Context) {
	log.Println("Starting memory intensive workload")
	defer log.Println("Memory intensive workload completed")

	var memStats runtime.MemStats

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Create and destroy large slices
			largeSlices := make([][]byte, 100)
			for i := range largeSlices {
				largeSlices[i] = make([]byte, 1024*10) // 10KB each
			}

			// Create maps with many entries
			largeMap := make(map[string]interface{})
			for i := 0; i < 1000; i++ {
				largeMap[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
			}

			// Force garbage collection occasionally
			if len(largeMap)%5 == 0 {
				runtime.GC()
			}

			// Read memory stats (this allocates)
			runtime.ReadMemStats(&memStats)

			// Clear references
			largeSlices = nil
			largeMap = nil

			time.Sleep(2 * time.Millisecond)
		}
	}
}
