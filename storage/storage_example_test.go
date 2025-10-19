package storage_test

import (
	"fmt"

	"github.com/mutualEvg/metrics-server/storage"
)

// ExampleNewMemStorage demonstrates how to create and use a new in-memory storage
func ExampleNewMemStorage() {
	// Create a new in-memory storage instance
	store := storage.NewMemStorage()

	// Update some gauge metrics (floating-point values)
	store.UpdateGauge("cpu_usage", 45.5)
	store.UpdateGauge("memory_usage", 78.2)

	// Update some counter metrics (integer values that accumulate)
	store.UpdateCounter("http_requests", 100)
	store.UpdateCounter("http_requests", 50) // This adds to the previous value

	// Retrieve individual metrics
	if cpuUsage, exists := store.GetGauge("cpu_usage"); exists {
		fmt.Printf("CPU Usage: %.1f%%\n", cpuUsage)
	}

	if requestCount, exists := store.GetCounter("http_requests"); exists {
		fmt.Printf("HTTP Requests: %d\n", requestCount)
	}

	// Get all metrics at once
	gauges, counters := store.GetAll()
	fmt.Printf("Total gauges: %d, Total counters: %d\n", len(gauges), len(counters))

	// Output:
	// CPU Usage: 45.5%
	// HTTP Requests: 150
	// Total gauges: 2, Total counters: 1
}

// ExampleMemStorage_UpdateGauge demonstrates gauge metric operations
func ExampleMemStorage_UpdateGauge() {
	store := storage.NewMemStorage()

	// Set gauge values (they replace previous values)
	store.UpdateGauge("temperature", 25.5)
	store.UpdateGauge("temperature", 26.8) // Replaces the previous value

	if temp, exists := store.GetGauge("temperature"); exists {
		fmt.Printf("Current temperature: %.1f°C\n", temp)
	}

	// Output:
	// Current temperature: 26.8°C
}

// ExampleMemStorage_UpdateCounter demonstrates counter metric operations
func ExampleMemStorage_UpdateCounter() {
	store := storage.NewMemStorage()

	// Add to counter values (they accumulate)
	store.UpdateCounter("page_views", 10)
	store.UpdateCounter("page_views", 25)
	store.UpdateCounter("page_views", 5)

	if views, exists := store.GetCounter("page_views"); exists {
		fmt.Printf("Total page views: %d\n", views)
	}

	// Output:
	// Total page views: 40
}

// ExampleMemStorage_GetAll demonstrates retrieving all metrics at once
func ExampleMemStorage_GetAll() {
	store := storage.NewMemStorage()

	// Add various metrics
	store.UpdateGauge("cpu", 45.2)
	store.UpdateGauge("memory", 67.8)
	store.UpdateCounter("requests", 100)
	store.UpdateCounter("errors", 5)

	// Get all metrics
	gauges, counters := store.GetAll()

	fmt.Printf("Total gauge metrics: %d\n", len(gauges))
	fmt.Printf("Total counter metrics: %d\n", len(counters))

	// Check specific values
	if cpuValue, exists := gauges["cpu"]; exists {
		fmt.Printf("CPU usage: %.1f\n", cpuValue)
	}

	if requestCount, exists := counters["requests"]; exists {
		fmt.Printf("Request count: %d\n", requestCount)
	}

	// Output:
	// Total gauge metrics: 2
	// Total counter metrics: 2
	// CPU usage: 45.2
	// Request count: 100
}
