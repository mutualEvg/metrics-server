package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/mutualEvg/metrics-server/internal/agent"
	"github.com/mutualEvg/metrics-server/internal/collector"
	"github.com/mutualEvg/metrics-server/internal/crypto"
	"github.com/mutualEvg/metrics-server/internal/grpcclient"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"

	pollCount int64
)

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func main() {
	printBuildInfo()

	// Parse configuration
	config := agent.ParseConfig()

	// Determine if we should use gRPC or HTTP
	if config.GRPCAddress != "" {
		// Run gRPC-based agent
		runGRPCAgent(config)
	} else {
		// Run HTTP-based agent (original behavior)
		runHTTPAgent(config)
	}
}

func runGRPCAgent(config *agent.Config) {
	log.Println("Starting agent with gRPC protocol")

	// Create gRPC client
	grpcClient, err := grpcclient.NewMetricsClient(config.GRPCAddress)
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer grpcClient.Close()

	// Setup graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Start metric collection and sending via gRPC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to collect and send metrics
	go collectAndSendGRPC(ctx, grpcClient, config)

	// Wait for shutdown signal
	sig := <-signalChan
	log.Printf("Shutdown signal received: %v", sig)
	log.Println("Stopping gRPC agent gracefully...")

	// Cancel metric collection
	cancel()

	// Give time to send final batch
	log.Println("Flushing final metrics...")
	time.Sleep(2 * time.Second)

	log.Println("gRPC agent shutdown complete")
}

func runHTTPAgent(config *agent.Config) {
	log.Println("Starting agent with HTTP protocol")

	// Load public key for encryption if configured
	var publicKey *rsa.PublicKey
	if config.CryptoKey != "" {
		var err error
		publicKey, err = crypto.LoadPublicKeyFromFile(config.CryptoKey)
		if err != nil {
			log.Fatalf("Failed to load public key from %s: %v", config.CryptoKey, err)
		}
		log.Printf("Public key loaded from %s", config.CryptoKey)
	}

	// Initialize worker pool
	workerPool := worker.NewPool(config.RateLimit, config.ServerAddress, config.Key, config.RetryConfig)
	workerPool.SetPublicKey(publicKey)
	workerPool.Start()

	// Setup graceful shutdown - handle SIGTERM, SIGINT, SIGQUIT
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Start metric collection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize metric collector with channel-based communication
	metricCollector := collector.New(
		workerPool,
		config.PollInterval,
		config.ReportInterval,
		config.BatchSize,
		config.ServerAddress,
		config.Key,
		config.RetryConfig,
		&pollCount,
	)
	metricCollector.SetPublicKey(publicKey)

	metricCollector.Start(ctx)

	// Wait for shutdown signal
	sig := <-signalChan
	log.Printf("Shutdown signal received: %v", sig)
	log.Println("Stopping HTTP agent gracefully...")

	// Cancel metric collection
	cancel()

	// Give collector time to send final batch of metrics
	log.Println("Flushing final metrics...")
	time.Sleep(2 * time.Second)

	// Stop worker pool (waits for in-flight requests)
	log.Println("Stopping worker pool...")
	workerPool.Stop()

	log.Println("HTTP agent shutdown complete")
}

// collectAndSendGRPC collects metrics and sends them via gRPC
func collectAndSendGRPC(ctx context.Context, grpcClient *grpcclient.MetricsClient, config *agent.Config) {
	pollTicker := time.NewTicker(config.PollInterval)
	reportTicker := time.NewTicker(config.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	var metrics []models.Metrics
	var pollCounter int64

	// List of runtime metrics to collect
	runtimeMetrics := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
		"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
		"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
		"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
		"StackInuse", "StackSys", "Sys", "TotalAlloc",
	}

	for {
		select {
		case <-ctx.Done():
			// Send final metrics before exiting
			if len(metrics) > 0 {
				if err := grpcClient.SendMetrics(context.Background(), metrics); err != nil {
					log.Printf("Failed to send final metrics: %v", err)
				}
			}
			return

		case <-pollTicker.C:
			// Collect runtime metrics
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			for _, metricName := range runtimeMetrics {
				var value float64
				switch metricName {
				case "Alloc":
					value = float64(memStats.Alloc)
				case "BuckHashSys":
					value = float64(memStats.BuckHashSys)
				case "Frees":
					value = float64(memStats.Frees)
				case "GCCPUFraction":
					value = memStats.GCCPUFraction
				case "GCSys":
					value = float64(memStats.GCSys)
				case "HeapAlloc":
					value = float64(memStats.HeapAlloc)
				case "HeapIdle":
					value = float64(memStats.HeapIdle)
				case "HeapInuse":
					value = float64(memStats.HeapInuse)
				case "HeapObjects":
					value = float64(memStats.HeapObjects)
				case "HeapReleased":
					value = float64(memStats.HeapReleased)
				case "HeapSys":
					value = float64(memStats.HeapSys)
				case "LastGC":
					value = float64(memStats.LastGC)
				case "Lookups":
					value = float64(memStats.Lookups)
				case "MCacheInuse":
					value = float64(memStats.MCacheInuse)
				case "MCacheSys":
					value = float64(memStats.MCacheSys)
				case "MSpanInuse":
					value = float64(memStats.MSpanInuse)
				case "MSpanSys":
					value = float64(memStats.MSpanSys)
				case "Mallocs":
					value = float64(memStats.Mallocs)
				case "NextGC":
					value = float64(memStats.NextGC)
				case "NumForcedGC":
					value = float64(memStats.NumForcedGC)
				case "NumGC":
					value = float64(memStats.NumGC)
				case "OtherSys":
					value = float64(memStats.OtherSys)
				case "PauseTotalNs":
					value = float64(memStats.PauseTotalNs)
				case "StackInuse":
					value = float64(memStats.StackInuse)
				case "StackSys":
					value = float64(memStats.StackSys)
				case "Sys":
					value = float64(memStats.Sys)
				case "TotalAlloc":
					value = float64(memStats.TotalAlloc)
				}

				metrics = append(metrics, models.Metrics{
					ID:    metricName,
					MType: "gauge",
					Value: &value,
				})
			}

			// Add RandomValue
			randomValue := rand.Float64()
			metrics = append(metrics, models.Metrics{
				ID:    "RandomValue",
				MType: "gauge",
				Value: &randomValue,
			})

			// Collect system metrics
			if memInfo, err := mem.VirtualMemory(); err == nil {
				totalMem := float64(memInfo.Total)
				freeMem := float64(memInfo.Free)

				metrics = append(metrics, models.Metrics{
					ID:    "TotalMemory",
					MType: "gauge",
					Value: &totalMem,
				})

				metrics = append(metrics, models.Metrics{
					ID:    "FreeMemory",
					MType: "gauge",
					Value: &freeMem,
				})
			}

			// Collect CPU metrics
			if cpuPercents, err := cpu.Percent(time.Second, true); err == nil {
				for i, percent := range cpuPercents {
					metricName := fmt.Sprintf("CPUutilization%d", i+1)
					cpuValue := percent

					metrics = append(metrics, models.Metrics{
						ID:    metricName,
						MType: "gauge",
						Value: &cpuValue,
					})
				}
			}

			// Increment poll counter
			atomic.AddInt64(&pollCounter, 1)

		case <-reportTicker.C:
			// Add poll counter to metrics
			currentCount := atomic.LoadInt64(&pollCounter)
			metrics = append(metrics, models.Metrics{
				ID:    "PollCount",
				MType: "counter",
				Delta: &currentCount,
			})

			// Send metrics via gRPC
			if len(metrics) > 0 {
				log.Printf("Sending %d metrics via gRPC", len(metrics))
				if err := grpcClient.SendMetrics(ctx, metrics); err != nil {
					log.Printf("Failed to send metrics via gRPC: %v", err)
				}
				// Clear metrics after sending
				metrics = metrics[:0]
			}
		}
	}
}
