package main

import (
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
)

const (
	defaultServerAddress  = "http://localhost:8080"
	defaultPollInterval   = 2
	defaultReportInterval = 10
)

var (
	serverAddress  string
	pollInterval   time.Duration
	reportInterval time.Duration
	pollCount      int64
)

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

	// --- Main program starts
	var gauges sync.Map

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
			reportMetrics(&gauges)

		case <-signalChan:
			fmt.Println("Received shutdown signal. Exiting...")
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

func reportMetrics(gauges *sync.Map) {
	client := &http.Client{}

	gauges.Range(func(key, value any) bool {
		name, _ := key.(string)
		val, _ := value.(float64)
		sendMetric(client, "gauge", name, fmt.Sprintf("%f", val))
		return true
	})

	sendMetric(client, "counter", "PollCount", strconv.FormatInt(pollCount, 10))
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
