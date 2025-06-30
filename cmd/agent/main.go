package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/mutualEvg/metrics-server/internal/hash"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

const (
	defaultServerAddress  = "http://localhost:8080"
	defaultPollInterval   = 2
	defaultReportInterval = 10
	defaultBatchSize      = 0  // Default to individual sending for backward compatibility
	defaultRateLimit      = 10 // Default rate limit for concurrent requests
)

var (
	serverAddress  string
	pollInterval   time.Duration
	reportInterval time.Duration
	batchSize      int
	pollCount      int64
	retryConfig    retry.RetryConfig
	key            string // Key for SHA256 signature
	rateLimit      int    // Maximum concurrent requests
)

// MetricData represents a single metric to be sent
type MetricData struct {
	Metric models.Metrics
	Type   string // "runtime" or "system"
}

// MetricsWorkerPool manages concurrent metric sending
type MetricsWorkerPool struct {
	jobs       chan MetricData
	wg         sync.WaitGroup
	rateLimit  int
	httpClient *http.Client
}

// NewMetricsWorkerPool creates a new worker pool
func NewMetricsWorkerPool(rateLimit int) *MetricsWorkerPool {
	return &MetricsWorkerPool{
		jobs:       make(chan MetricData, rateLimit*2), // Buffer to prevent blocking
		rateLimit:  rateLimit,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Start initializes the worker pool
func (p *MetricsWorkerPool) Start() {
	for i := 0; i < p.rateLimit; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Started worker pool with %d workers", p.rateLimit)
}

// Stop gracefully shuts down the worker pool
func (p *MetricsWorkerPool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	log.Printf("Worker pool stopped")
}

// SubmitMetric adds a metric to the sending queue
func (p *MetricsWorkerPool) SubmitMetric(metric MetricData) {
	// Recover from panic if channel is closed
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker pool channel closed, dropping metric: %s", metric.Metric.ID)
		}
	}()

	select {
	case p.jobs <- metric:
		// Metric submitted successfully
	default:
		log.Printf("Worker pool queue full, dropping metric: %s", metric.Metric.ID)
	}
}

// worker processes metrics from the queue
func (p *MetricsWorkerPool) worker(id int) {
	defer p.wg.Done()
	log.Printf("Worker %d started", id)

	for metric := range p.jobs {
		p.sendMetric(metric)
	}

	log.Printf("Worker %d stopped", id)
}

// sendMetric sends a single metric to the server
func (p *MetricsWorkerPool) sendMetric(metricData MetricData) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := retry.Do(ctx, retryConfig, func() error {
		jsonData, err := json.Marshal(metricData.Metric)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Compress the JSON data
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		_, err = gzipWriter.Write(jsonData)
		if err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		err = gzipWriter.Close()
		if err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		url := fmt.Sprintf("%s/update/", serverAddress)
		req, err := http.NewRequest(http.MethodPost, url, &compressedData)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		// Add hash header if key is configured
		if key != "" {
			hashValue := hash.CalculateHash(compressedData.Bytes(), key)
			req.Header.Set("HashSHA256", hashValue)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send metric: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned non-OK status: %s", resp.Status)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to send %s metric %s after retries: %v", metricData.Type, metricData.Metric.ID, err)
	}
}

// MetricsBatch holds a collection of metrics to send as batch
type MetricsBatch struct {
	metrics []models.Metrics
	mu      sync.Mutex
}

// NewMetricsBatch creates a new batch
func NewMetricsBatch() *MetricsBatch {
	return &MetricsBatch{
		metrics: make([]models.Metrics, 0),
	}
}

// AddGauge adds a gauge metric to the batch
func (mb *MetricsBatch) AddGauge(name string, value float64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.metrics = append(mb.metrics, models.Metrics{
		ID:    name,
		MType: "gauge",
		Value: &value,
	})
}

// AddCounter adds a counter metric to the batch
func (mb *MetricsBatch) AddCounter(name string, delta int64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.metrics = append(mb.metrics, models.Metrics{
		ID:    name,
		MType: "counter",
		Delta: &delta,
	})
}

// GetAndClear returns all metrics and clears the batch
func (mb *MetricsBatch) GetAndClear() []models.Metrics {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if len(mb.metrics) == 0 {
		return nil
	}

	result := make([]models.Metrics, len(mb.metrics))
	copy(result, mb.metrics)
	mb.metrics = mb.metrics[:0] // Clear the slice
	return result
}

// List of runtime metrics to collect
var runtimeGaugeMetrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
	"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
	"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
	"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
	"StackInuse", "StackSys", "Sys", "TotalAlloc",
}

// MetricCollector handles metric collection and transmission via channels
type MetricCollector struct {
	runtimeChan chan MetricData
	systemChan  chan MetricData
	workerPool  *MetricsWorkerPool
}

// NewMetricCollector creates a new metric collector
func NewMetricCollector(workerPool *MetricsWorkerPool) *MetricCollector {
	return &MetricCollector{
		runtimeChan: make(chan MetricData, 100), // Buffered channel
		systemChan:  make(chan MetricData, 100), // Buffered channel
		workerPool:  workerPool,
	}
}

// Start begins metric collection and forwarding
func (mc *MetricCollector) Start(ctx context.Context) {
	// Start runtime metrics collection
	go mc.collectRuntimeMetrics(ctx)

	// Start system metrics collection
	go mc.collectSystemMetrics(ctx)

	// Start metric forwarding to worker pool
	go mc.forwardMetrics(ctx)
}

// collectRuntimeMetrics collects Go runtime metrics and sends via channel
func (mc *MetricCollector) collectRuntimeMetrics(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// Send runtime metrics via channel
			for _, metric := range runtimeGaugeMetrics {
				var value float64
				switch metric {
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

				select {
				case mc.runtimeChan <- MetricData{
					Metric: models.Metrics{
						ID:    metric,
						MType: "gauge",
						Value: &value,
					},
					Type: "runtime",
				}:
				case <-ctx.Done():
					return
				default:
					// Channel full, skip this metric
					log.Printf("Runtime channel full, dropping metric: %s", metric)
				}
			}

			// Send random metric
			randomValue := rand.Float64()
			select {
			case mc.runtimeChan <- MetricData{
				Metric: models.Metrics{
					ID:    "RandomValue",
					MType: "gauge",
					Value: &randomValue,
				},
				Type: "runtime",
			}:
			case <-ctx.Done():
				return
			default:
				log.Printf("Runtime channel full, dropping RandomValue metric")
			}

			// Increment poll count
			pollCount++
		}
	}
}

// collectSystemMetrics collects system metrics using gopsutil and sends via channel
func (mc *MetricCollector) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect memory metrics
			if memInfo, err := mem.VirtualMemory(); err == nil {
				totalMem := float64(memInfo.Total)
				freeMem := float64(memInfo.Free)

				select {
				case mc.systemChan <- MetricData{
					Metric: models.Metrics{
						ID:    "TotalMemory",
						MType: "gauge",
						Value: &totalMem,
					},
					Type: "system",
				}:
				case <-ctx.Done():
					return
				default:
					log.Printf("System channel full, dropping TotalMemory metric")
				}

				select {
				case mc.systemChan <- MetricData{
					Metric: models.Metrics{
						ID:    "FreeMemory",
						MType: "gauge",
						Value: &freeMem,
					},
					Type: "system",
				}:
				case <-ctx.Done():
					return
				default:
					log.Printf("System channel full, dropping FreeMemory metric")
				}
			}

			// Collect CPU utilization for each CPU
			if cpuPercents, err := cpu.Percent(time.Second, true); err == nil {
				for i, percent := range cpuPercents {
					metricName := fmt.Sprintf("CPUutilization%d", i+1)
					cpuValue := percent

					select {
					case mc.systemChan <- MetricData{
						Metric: models.Metrics{
							ID:    metricName,
							MType: "gauge",
							Value: &cpuValue,
						},
						Type: "system",
					}:
					case <-ctx.Done():
						return
					default:
						log.Printf("System channel full, dropping %s metric", metricName)
					}
				}
			}
		}
	}
}

// forwardMetrics reads from channels and forwards to worker pool or batch
func (mc *MetricCollector) forwardMetrics(ctx context.Context) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	var runtimeMetrics []MetricData
	var systemMetrics []MetricData

	for {
		select {
		case <-ctx.Done():
			// Only send final metrics if not in test mode (when worker pool might be stopping)
			if os.Getenv("TEST_MODE") != "true" {
				mc.sendCollectedMetrics(runtimeMetrics, systemMetrics)
			}
			return

		case metric := <-mc.runtimeChan:
			runtimeMetrics = append(runtimeMetrics, metric)

		case metric := <-mc.systemChan:
			systemMetrics = append(systemMetrics, metric)

		case <-ticker.C:
			// Send collected metrics
			mc.sendCollectedMetrics(runtimeMetrics, systemMetrics)

			// Clear collected metrics
			runtimeMetrics = runtimeMetrics[:0]
			systemMetrics = systemMetrics[:0]
		}
	}
}

// sendCollectedMetrics sends the collected metrics via worker pool or batch
func (mc *MetricCollector) sendCollectedMetrics(runtimeMetrics, systemMetrics []MetricData) {
	if batchSize > 0 {
		mc.sendMetricsBatch(runtimeMetrics, systemMetrics)
	} else {
		mc.sendMetricsIndividual(runtimeMetrics, systemMetrics)
	}
}

// sendMetricsIndividual sends each metric individually using the worker pool
func (mc *MetricCollector) sendMetricsIndividual(runtimeMetrics, systemMetrics []MetricData) {
	// Send runtime metrics
	for _, metric := range runtimeMetrics {
		mc.workerPool.SubmitMetric(metric)
	}

	// Send system metrics
	for _, metric := range systemMetrics {
		mc.workerPool.SubmitMetric(metric)
	}

	// Send counter metric
	counter := MetricData{
		Metric: models.Metrics{
			ID:    "PollCount",
			MType: "counter",
			Delta: &pollCount,
		},
		Type: "runtime",
	}
	mc.workerPool.SubmitMetric(counter)
}

// sendMetricsBatch sends metrics in batches
func (mc *MetricCollector) sendMetricsBatch(runtimeMetrics, systemMetrics []MetricData) {
	batch := NewMetricsBatch()

	// Add runtime metrics to batch
	for _, metricData := range runtimeMetrics {
		if metricData.Metric.Value != nil {
			batch.AddGauge(metricData.Metric.ID, *metricData.Metric.Value)
		}
	}

	// Add system metrics to batch
	for _, metricData := range systemMetrics {
		if metricData.Metric.Value != nil {
			batch.AddGauge(metricData.Metric.ID, *metricData.Metric.Value)
		}
	}

	// Add counter metric
	batch.AddCounter("PollCount", pollCount)

	// Get all metrics and send as batch
	metrics := batch.GetAndClear()
	if len(metrics) > 0 {
		if err := sendBatch(metrics); err != nil {
			log.Printf("Failed to send batch: %v", err)
			// Fallback to individual sending via worker pool
			for _, metric := range metrics {
				var metricData MetricData
				if metric.Value != nil {
					metricData = MetricData{
						Metric: metric,
						Type:   "batch_fallback",
					}
				} else if metric.Delta != nil {
					metricData = MetricData{
						Metric: metric,
						Type:   "batch_fallback",
					}
				}
				mc.workerPool.SubmitMetric(metricData)
			}
		} else {
			log.Printf("Successfully sent batch of %d metrics", len(metrics))
		}
	}
}

func main() {
	// Read flags
	flagAddress := flag.String("a", "", "HTTP server address (default: http://localhost:8080)")
	flagReport := flag.Int("r", 0, "Report interval in seconds (default: 10)")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds (default: 2)")
	flagBatchSize := flag.Int("b", 0, "Batch size for metrics (default: 10, 0 = disable batching)")
	flagDisableRetry := flag.Bool("disable-retry", false, "Disable retry logic for testing")
	flagKey := flag.String("k", "", "Key for SHA256 signature")
	flagRateLimit := flag.Int("l", 0, "Rate limit for concurrent requests (default: 10)")
	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatalf("Unknown flags: %v", flag.Args())
	}

	// --- Address
	address := os.Getenv("ADDRESS")
	if address == "" {
		if *flagAddress != "" {
			address = *flagAddress
		} else {
			address = defaultServerAddress
		}
	}
	serverAddress = address

	if !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	// --- Key
	keyEnv := os.Getenv("KEY")
	if keyEnv != "" {
		key = keyEnv
	} else if *flagKey != "" {
		key = *flagKey
	}

	if key != "" {
		log.Printf("SHA256 signature enabled")
	}

	// --- Rate Limit
	rateLimitEnv := os.Getenv("RATE_LIMIT")
	if rateLimitEnv != "" {
		val, err := strconv.Atoi(rateLimitEnv)
		if err != nil {
			log.Fatalf("Invalid RATE_LIMIT: %v", err)
		}
		rateLimit = val
	} else if *flagRateLimit != 0 {
		rateLimit = *flagRateLimit
	} else {
		rateLimit = defaultRateLimit
	}

	// --- Report Interval
	reportEnv := os.Getenv("REPORT_INTERVAL")
	var reportSeconds int
	if reportEnv != "" {
		val, err := strconv.Atoi(reportEnv)
		if err != nil {
			log.Fatalf("Invalid REPORT_INTERVAL: %v", err)
		}
		reportSeconds = val
	} else if *flagReport != 0 {
		reportSeconds = *flagReport
	} else {
		reportSeconds = defaultReportInterval
	}
	reportInterval = time.Duration(reportSeconds) * time.Second

	// --- Poll Interval
	pollEnv := os.Getenv("POLL_INTERVAL")
	var pollSeconds int
	if pollEnv != "" {
		val, err := strconv.Atoi(pollEnv)
		if err != nil {
			log.Fatalf("Invalid POLL_INTERVAL: %v", err)
		}
		pollSeconds = val
	} else if *flagPoll != 0 {
		pollSeconds = *flagPoll
	} else {
		pollSeconds = defaultPollInterval
	}
	pollInterval = time.Duration(pollSeconds) * time.Second

	// --- Batch Size
	batchEnv := os.Getenv("BATCH_SIZE")
	if batchEnv != "" {
		val, err := strconv.Atoi(batchEnv)
		if err != nil {
			log.Fatalf("Invalid BATCH_SIZE: %v", err)
		}
		batchSize = val
	} else if *flagBatchSize != 0 {
		batchSize = *flagBatchSize
	} else {
		batchSize = defaultBatchSize
	}

	log.Printf("Agent starting with server=%s, poll=%v, report=%v, batch_size=%d, rate_limit=%d",
		serverAddress, pollInterval, reportInterval, batchSize, rateLimit)

	// Initialize retry configuration
	if os.Getenv("ENABLE_FULL_RETRY") == "true" {
		retryConfig = retry.DefaultConfig()
	} else {
		retryConfig = retry.FastConfig()
	}

	if *flagDisableRetry || os.Getenv("DISABLE_RETRY") == "true" {
		retryConfig = retry.NoRetryConfig()
	} else if os.Getenv("TEST_MODE") == "true" {
		retryConfig.MaxAttempts = 1
		retryConfig.Intervals = []time.Duration{}
	}

	// --- Main program starts
	workerPool := NewMetricsWorkerPool(rateLimit)
	workerPool.Start()
	defer workerPool.Stop()

	// Setup graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	// Start metric collection goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize metric collector with channel-based communication
	metricCollector := NewMetricCollector(workerPool)
	metricCollector.Start(ctx)

	// Wait for shutdown signal
	<-signalChan
	fmt.Println("Received shutdown signal. Stopping agent...")
	cancel() // Cancel all goroutines

	// Give some time for final metrics to be processed
	log.Println("Sending final metrics...")
	time.Sleep(1 * time.Second)
}

// sendBatch sends a batch of metrics using the /updates/ endpoint (unchanged)
func sendBatch(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil // Don't send empty batches
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return retry.Do(ctx, retryConfig, func() error {
		// Marshal to JSON
		jsonData, err := json.Marshal(metrics)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}

		// Compress with gzip
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		if _, err := gzipWriter.Write(jsonData); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		// Create HTTP request
		url := fmt.Sprintf("%s/updates/", serverAddress)
		req, err := http.NewRequest("POST", url, &compressedData)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		// Add hash header if key is configured
		if key != "" {
			hashValue := hash.CalculateHash(compressedData.Bytes(), key)
			req.Header.Set("HashSHA256", hashValue)
		}

		// Send request
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status %d", resp.StatusCode)
		}

		return nil
	})
}
