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

	for {
		select {
		case <-ctx.Done():
			sendFinalMetrics(grpcClient, metrics)
			return

		case <-pollTicker.C:
			// Collect all metrics
			metrics = append(metrics, collectRuntimeMetrics()...)
			metrics = append(metrics, collectSystemMetrics()...)
			atomic.AddInt64(&pollCounter, 1)

		case <-reportTicker.C:
			metrics = appendPollCount(metrics, &pollCounter)
			sendMetricsBatch(ctx, grpcClient, &metrics)
		}
	}
}

// collectRuntimeMetrics collects Go runtime and random metrics
func collectRuntimeMetrics() []models.Metrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := make([]models.Metrics, 0, 28)

	// Collect all runtime metrics using a map for cleaner code
	metricValues := map[string]float64{
		"Alloc":         float64(memStats.Alloc),
		"BuckHashSys":   float64(memStats.BuckHashSys),
		"Frees":         float64(memStats.Frees),
		"GCCPUFraction": memStats.GCCPUFraction,
		"GCSys":         float64(memStats.GCSys),
		"HeapAlloc":     float64(memStats.HeapAlloc),
		"HeapIdle":      float64(memStats.HeapIdle),
		"HeapInuse":     float64(memStats.HeapInuse),
		"HeapObjects":   float64(memStats.HeapObjects),
		"HeapReleased":  float64(memStats.HeapReleased),
		"HeapSys":       float64(memStats.HeapSys),
		"LastGC":        float64(memStats.LastGC),
		"Lookups":       float64(memStats.Lookups),
		"MCacheInuse":   float64(memStats.MCacheInuse),
		"MCacheSys":     float64(memStats.MCacheSys),
		"MSpanInuse":    float64(memStats.MSpanInuse),
		"MSpanSys":      float64(memStats.MSpanSys),
		"Mallocs":       float64(memStats.Mallocs),
		"NextGC":        float64(memStats.NextGC),
		"NumForcedGC":   float64(memStats.NumForcedGC),
		"NumGC":         float64(memStats.NumGC),
		"OtherSys":      float64(memStats.OtherSys),
		"PauseTotalNs":  float64(memStats.PauseTotalNs),
		"StackInuse":    float64(memStats.StackInuse),
		"StackSys":      float64(memStats.StackSys),
		"Sys":           float64(memStats.Sys),
		"TotalAlloc":    float64(memStats.TotalAlloc),
	}

	for name, value := range metricValues {
		v := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: "gauge",
			Value: &v,
		})
	}

	// Add RandomValue
	randomValue := rand.Float64()
	metrics = append(metrics, models.Metrics{
		ID:    "RandomValue",
		MType: "gauge",
		Value: &randomValue,
	})

	return metrics
}

// collectSystemMetrics collects memory and CPU metrics
func collectSystemMetrics() []models.Metrics {
	metrics := make([]models.Metrics, 0, 10)

	// Collect memory metrics
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
			cpuValue := percent
			metrics = append(metrics, models.Metrics{
				ID:    fmt.Sprintf("CPUutilization%d", i+1),
				MType: "gauge",
				Value: &cpuValue,
			})
		}
	}

	return metrics
}

// appendPollCount adds the poll counter metric to the metrics slice
func appendPollCount(metrics []models.Metrics, pollCounter *int64) []models.Metrics {
	currentCount := atomic.LoadInt64(pollCounter)
	return append(metrics, models.Metrics{
		ID:    "PollCount",
		MType: "counter",
		Delta: &currentCount,
	})
}

// sendMetricsBatch sends metrics via gRPC and clears the slice
func sendMetricsBatch(ctx context.Context, grpcClient *grpcclient.MetricsClient, metrics *[]models.Metrics) {
	if len(*metrics) > 0 {
		log.Printf("Sending %d metrics via gRPC", len(*metrics))
		if err := grpcClient.SendMetrics(ctx, *metrics); err != nil {
			log.Printf("Failed to send metrics via gRPC: %v", err)
		}
		*metrics = (*metrics)[:0]
	}
}

// sendFinalMetrics sends remaining metrics before shutdown
func sendFinalMetrics(grpcClient *grpcclient.MetricsClient, metrics []models.Metrics) {
	if len(metrics) > 0 {
		if err := grpcClient.SendMetrics(context.Background(), metrics); err != nil {
			log.Printf("Failed to send final metrics: %v", err)
		}
	}
}
