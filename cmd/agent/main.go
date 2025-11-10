package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mutualEvg/metrics-server/internal/agent"
	"github.com/mutualEvg/metrics-server/internal/collector"
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

	// Initialize worker pool
	workerPool := worker.NewPool(config.RateLimit, config.ServerAddress, config.Key, config.RetryConfig)

	// Load public key for encryption if configured
	if config.CryptoKey != "" {
		if err := workerPool.SetPublicKey(config.CryptoKey); err != nil {
			log.Fatalf("Failed to load public key: %v", err)
		}
	}

	workerPool.Start()
	defer workerPool.Stop()

	// Setup graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

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

	// Set crypto key for batch sending
	if config.CryptoKey != "" {
		metricCollector.SetCryptoKey(config.CryptoKey)
	}

	metricCollector.Start(ctx)

	// Wait for shutdown signal
	<-signalChan
	fmt.Println("Received shutdown signal. Stopping agent...")
	cancel() // Cancel all goroutines

	// Give some time for final metrics to be processed
	log.Println("Sending final metrics...")
	time.Sleep(1 * time.Second)
}
