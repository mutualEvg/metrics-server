package collector

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/mutualEvg/metrics-server/internal/batch"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/mutualEvg/metrics-server/internal/worker"
)

// List of runtime metrics to collect
var runtimeGaugeMetrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
	"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
	"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
	"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
	"StackInuse", "StackSys", "Sys", "TotalAlloc",
}

// Collector handles metric collection and transmission via channels
type Collector struct {
	runtimeChan    chan worker.MetricData
	systemChan     chan worker.MetricData
	workerPool     *worker.Pool
	pollInterval   time.Duration
	reportInterval time.Duration
	batchSize      int
	serverAddr     string
	key            string
	publicKey      *rsa.PublicKey // Public key for encryption
	retryConfig    retry.RetryConfig
	pollCount      *int64
}

// New creates a new metric collector
func New(workerPool *worker.Pool, pollInterval, reportInterval time.Duration, batchSize int, serverAddr, key string, retryConfig retry.RetryConfig, pollCount *int64) *Collector {
	return &Collector{
		runtimeChan:    make(chan worker.MetricData, 100), // Buffered channel
		systemChan:     make(chan worker.MetricData, 100), // Buffered channel
		workerPool:     workerPool,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		batchSize:      batchSize,
		serverAddr:     serverAddr,
		key:            key,
		publicKey:      nil,
		retryConfig:    retryConfig,
		pollCount:      pollCount,
	}
}

// SetPublicKey sets the public key for encryption
func (c *Collector) SetPublicKey(publicKey *rsa.PublicKey) {
	c.publicKey = publicKey
}

// Start begins metric collection and forwarding
func (c *Collector) Start(ctx context.Context) {
	// Start runtime metrics collection
	go c.collectRuntimeMetrics(ctx)

	// Start system metrics collection
	go c.collectSystemMetrics(ctx)

	// Start metric forwarding to worker pool
	go c.forwardMetrics(ctx)
}

// collectRuntimeMetrics collects Go runtime metrics and sends via channel
func (c *Collector) collectRuntimeMetrics(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
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
				case c.runtimeChan <- worker.MetricData{
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
			case c.runtimeChan <- worker.MetricData{
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
		atomic.AddInt64(c.pollCount, 1)
		}
	}
}

// collectSystemMetrics collects system metrics using gopsutil and sends via channel
func (c *Collector) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
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
				case c.systemChan <- worker.MetricData{
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
				case c.systemChan <- worker.MetricData{
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
					case c.systemChan <- worker.MetricData{
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
func (c *Collector) forwardMetrics(ctx context.Context) {
	ticker := time.NewTicker(c.reportInterval)
	defer ticker.Stop()

	var runtimeMetrics []worker.MetricData
	var systemMetrics []worker.MetricData

	for {
		select {
		case <-ctx.Done():
			// Only send final metrics if not in test mode (when worker pool might be stopping)
			if os.Getenv("TEST_MODE") != "true" {
				c.sendCollectedMetrics(runtimeMetrics, systemMetrics)
			}
			return

		case metric := <-c.runtimeChan:
			runtimeMetrics = append(runtimeMetrics, metric)

		case metric := <-c.systemChan:
			systemMetrics = append(systemMetrics, metric)

		case <-ticker.C:
			// Send collected metrics
			c.sendCollectedMetrics(runtimeMetrics, systemMetrics)

			// Clear collected metrics
			runtimeMetrics = runtimeMetrics[:0]
			systemMetrics = systemMetrics[:0]
		}
	}
}

// sendCollectedMetrics sends the collected metrics via worker pool or batch
func (c *Collector) sendCollectedMetrics(runtimeMetrics, systemMetrics []worker.MetricData) {
	if c.batchSize > 0 {
		c.sendMetricsBatch(runtimeMetrics, systemMetrics)
	} else {
		c.sendMetricsIndividual(runtimeMetrics, systemMetrics)
	}
}

// sendMetricsIndividual sends each metric individually using the worker pool
func (c *Collector) sendMetricsIndividual(runtimeMetrics, systemMetrics []worker.MetricData) {
	// Send runtime metrics
	for _, metric := range runtimeMetrics {
		c.workerPool.SubmitMetric(metric)
	}

	// Send system metrics
	for _, metric := range systemMetrics {
		c.workerPool.SubmitMetric(metric)
	}

	// Send counter metric
	counter := worker.MetricData{
		Metric: models.Metrics{
			ID:    "PollCount",
			MType: "counter",
			Delta: c.pollCount,
		},
		Type: "runtime",
	}
	c.workerPool.SubmitMetric(counter)
}

// sendMetricsBatch sends metrics in batches
func (c *Collector) sendMetricsBatch(runtimeMetrics, systemMetrics []worker.MetricData) {
	batchInstance := batch.New()

	// Add runtime metrics to batch
	for _, metricData := range runtimeMetrics {
		if metricData.Metric.Value != nil {
			batchInstance.AddGauge(metricData.Metric.ID, *metricData.Metric.Value)
		}
	}

	// Add system metrics to batch
	for _, metricData := range systemMetrics {
		if metricData.Metric.Value != nil {
			batchInstance.AddGauge(metricData.Metric.ID, *metricData.Metric.Value)
		}
	}

	// Add counter metric
	batchInstance.AddCounter("PollCount", *c.pollCount)

	// Get all metrics and send as batch
	metrics := batchInstance.GetAndClear()
	if len(metrics) > 0 {
		if err := batch.SendWithEncryption(metrics, c.serverAddr, c.key, c.publicKey, c.retryConfig); err != nil {
			log.Printf("Failed to send batch: %v", err)
			// Fallback to individual sending via worker pool
			for _, metric := range metrics {
				var metricData worker.MetricData
				if metric.Value != nil {
					metricData = worker.MetricData{
						Metric: metric,
						Type:   "batch_fallback",
					}
				} else if metric.Delta != nil {
					metricData = worker.MetricData{
						Metric: metric,
						Type:   "batch_fallback",
					}
				}
				c.workerPool.SubmitMetric(metricData)
			}
		} else {
			log.Printf("Successfully sent batch of %d metrics", len(metrics))
		}
	}
}

// GetRuntimeChan returns the runtime metrics channel for testing
func (c *Collector) GetRuntimeChan() <-chan worker.MetricData {
	return c.runtimeChan
}

// GetSystemChan returns the system metrics channel for testing
func (c *Collector) GetSystemChan() <-chan worker.MetricData {
	return c.systemChan
}
