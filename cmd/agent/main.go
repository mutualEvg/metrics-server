package main

import (
	"bytes"
	"compress/gzip"
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

	"github.com/mutualEvg/metrics-server/internal/models"
)

const (
	defaultServerAddress  = "http://localhost:8080"
	defaultPollInterval   = 2
	defaultReportInterval = 10
	defaultBatchSize      = 10
)

var (
	serverAddress  string
	pollInterval   time.Duration
	reportInterval time.Duration
	batchSize      int
	pollCount      int64
)

// MetricsBatch holds a collection of metrics to send
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

// List of metrics to collect from runtime.MemStats
var gaugeMetrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
	"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
	"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
	"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
	"StackInuse", "StackSys", "Sys", "TotalAlloc",
}

func main() {
	// Read flags
	flagAddress := flag.String("a", "", "HTTP server address (default: http://localhost:8080)")
	flagReport := flag.Int("r", 0, "Report interval in seconds (default: 10)")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds (default: 2)")
	flagBatchSize := flag.Int("b", 0, "Batch size for metrics (default: 10, 0 = disable batching)")
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

	log.Printf("Agent starting with server=%s, poll=%v, report=%v, batch_size=%d",
		serverAddress, pollInterval, reportInterval, batchSize)

	// --- Main program starts
	var gauges sync.Map
	batch := NewMetricsBatch()

	tickerPoll := time.NewTicker(pollInterval)
	defer tickerPoll.Stop()
	tickerReport := time.NewTicker(reportInterval)
	defer tickerReport.Stop()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for {
		select {
		case <-tickerPoll.C:
			pollMetrics(&gauges)

		case <-tickerReport.C:
			if batchSize > 0 {
				reportMetricsBatch(&gauges, batch)
			} else {
				reportMetricsIndividual(&gauges)
			}

		case <-signalChan:
			fmt.Println("Received shutdown signal. Sending final metrics...")
			if batchSize > 0 {
				reportMetricsBatch(&gauges, batch)
			} else {
				reportMetricsIndividual(&gauges)
			}
			return
		}
	}
}

func pollMetrics(gauges *sync.Map) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Update runtime metrics
	for _, metric := range gaugeMetrics {
		switch metric {
		case "Alloc":
			gauges.Store(metric, float64(memStats.Alloc))
		case "BuckHashSys":
			gauges.Store(metric, float64(memStats.BuckHashSys))
		case "Frees":
			gauges.Store(metric, float64(memStats.Frees))
		case "GCCPUFraction":
			gauges.Store(metric, memStats.GCCPUFraction)
		case "GCSys":
			gauges.Store(metric, float64(memStats.GCSys))
		case "HeapAlloc":
			gauges.Store(metric, float64(memStats.HeapAlloc))
		case "HeapIdle":
			gauges.Store(metric, float64(memStats.HeapIdle))
		case "HeapInuse":
			gauges.Store(metric, float64(memStats.HeapInuse))
		case "HeapObjects":
			gauges.Store(metric, float64(memStats.HeapObjects))
		case "HeapReleased":
			gauges.Store(metric, float64(memStats.HeapReleased))
		case "HeapSys":
			gauges.Store(metric, float64(memStats.HeapSys))
		case "LastGC":
			gauges.Store(metric, float64(memStats.LastGC))
		case "Lookups":
			gauges.Store(metric, float64(memStats.Lookups))
		case "MCacheInuse":
			gauges.Store(metric, float64(memStats.MCacheInuse))
		case "MCacheSys":
			gauges.Store(metric, float64(memStats.MCacheSys))
		case "MSpanInuse":
			gauges.Store(metric, float64(memStats.MSpanInuse))
		case "MSpanSys":
			gauges.Store(metric, float64(memStats.MSpanSys))
		case "Mallocs":
			gauges.Store(metric, float64(memStats.Mallocs))
		case "NextGC":
			gauges.Store(metric, float64(memStats.NextGC))
		case "NumForcedGC":
			gauges.Store(metric, float64(memStats.NumForcedGC))
		case "NumGC":
			gauges.Store(metric, float64(memStats.NumGC))
		case "OtherSys":
			gauges.Store(metric, float64(memStats.OtherSys))
		case "PauseTotalNs":
			gauges.Store(metric, float64(memStats.PauseTotalNs))
		case "StackInuse":
			gauges.Store(metric, float64(memStats.StackInuse))
		case "StackSys":
			gauges.Store(metric, float64(memStats.StackSys))
		case "Sys":
			gauges.Store(metric, float64(memStats.Sys))
		case "TotalAlloc":
			gauges.Store(metric, float64(memStats.TotalAlloc))
		}
	}

	// Add random metric
	gauges.Store("RandomValue", rand.Float64())

	// Increment poll count
	pollCount++
}

func reportMetricsBatch(gauges *sync.Map, batch *MetricsBatch) {
	// Add all gauge metrics to batch
	gauges.Range(func(key, value any) bool {
		name, _ := key.(string)
		val, _ := value.(float64)
		batch.AddGauge(name, val)
		return true
	})

	// Add counter metric
	batch.AddCounter("PollCount", pollCount)

	// Get all metrics and send as batch
	metrics := batch.GetAndClear()
	if metrics != nil && len(metrics) > 0 {
		if err := sendBatch(metrics); err != nil {
			log.Printf("Failed to send batch: %v", err)
			// Fallback to individual sending
			client := &http.Client{}
			for _, metric := range metrics {
				if metric.MType == "gauge" && metric.Value != nil {
					sendMetricJSON(client, "gauge", metric.ID, *metric.Value, 0)
				} else if metric.MType == "counter" && metric.Delta != nil {
					sendMetricJSON(client, "counter", metric.ID, 0, *metric.Delta)
				}
			}
		} else {
			log.Printf("Successfully sent batch of %d metrics", len(metrics))
		}
	}
}

// sendBatch sends a batch of metrics using the /updates/ endpoint
func sendBatch(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil // Don't send empty batches
	}

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
}

func reportMetricsIndividual(gauges *sync.Map) {
	client := &http.Client{}

	gauges.Range(func(key, value any) bool {
		name, _ := key.(string)
		val, _ := value.(float64)
		sendMetricJSON(client, "gauge", name, val, 0)
		return true
	})

	sendMetricJSON(client, "counter", "PollCount", 0, pollCount)
}

func sendMetric(client *http.Client, metricType, metricName, metricValue string) {
	url := fmt.Sprintf("%s/update/%s/%s/%s", serverAddress, metricType, metricName, metricValue)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(""))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send metric: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Server returned non-OK status: %s", resp.Status)
	}
}

// sendMetricJSON sends metrics using the new JSON API with gzip compression
func sendMetricJSON(client *http.Client, metricType, metricName string, gaugeValue float64, counterValue int64) {
	var metric models.Metrics
	metric.ID = metricName
	metric.MType = metricType

	switch metricType {
	case "gauge":
		metric.Value = &gaugeValue
	case "counter":
		metric.Delta = &counterValue
	default:
		log.Printf("Unknown metric type: %s", metricType)
		return
	}

	jsonData, err := json.Marshal(metric)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return
	}

	// Compress the JSON data
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err = gzipWriter.Write(jsonData)
	if err != nil {
		log.Printf("Failed to compress data: %v", err)
		return
	}
	err = gzipWriter.Close()
	if err != nil {
		log.Printf("Failed to close gzip writer: %v", err)
		return
	}

	url := fmt.Sprintf("%s/update/", serverAddress)
	req, err := http.NewRequest(http.MethodPost, url, &compressedData)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send metric: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Server returned non-OK status: %s", resp.Status)
	}
}
