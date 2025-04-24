package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// --- Metric Types ---
const (
	GaugeType   = "gauge"
	CounterType = "counter"
)

// --- Metric Storage ---
type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (ms *MemStorage) UpdateGauge(name string, value float64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.gauges[name] = value
}

func (ms *MemStorage) UpdateCounter(name string, value int64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.counters[name] += value
}

func (ms *MemStorage) GetGauge(name string) (float64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.gauges[name]
	return val, ok
}

func (ms *MemStorage) GetCounter(name string) (int64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.counters[name]
	return val, ok
}

func (ms *MemStorage) GetAll() (map[string]float64, map[string]int64) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	gCopy := make(map[string]float64)
	cCopy := make(map[string]int64)
	for k, v := range ms.gauges {
		gCopy[k] = v
	}
	for k, v := range ms.counters {
		cCopy[k] = v
	}
	return gCopy, cCopy
}

// --- Handlers ---

func updateHandler(storage *MemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/update/"), "/")
		if len(parts) != 3 {
			http.Error(w, "Invalid URL format", http.StatusNotFound)
			return
		}

		metricType := parts[0]
		metricName := parts[1]
		metricValue := parts[2]

		switch metricType {
		case GaugeType:
			val, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				http.Error(w, "Invalid gauge value", http.StatusBadRequest)
				return
			}
			storage.UpdateGauge(metricName, val)

		case CounterType:
			val, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				http.Error(w, "Invalid counter value", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(metricName, val)

		default:
			http.Error(w, "Unknown metric type", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}
}

func getValueHandler(storage *MemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metricType := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")

		switch metricType {
		case GaugeType:
			if val, ok := storage.GetGauge(name); ok {
				fmt.Fprintf(w, "%f", val)
				return
			}
		case CounterType:
			if val, ok := storage.GetCounter(name); ok {
				fmt.Fprintf(w, "%d", val)
				return
			}
		}

		http.Error(w, "Metric not found", http.StatusNotFound)
	}
}

func rootHandler(storage *MemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gauges, counters := storage.GetAll()
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h1>Metrics</h1><ul>")
		for name, val := range gauges {
			fmt.Fprintf(w, "<li>%s (gauge): %f</li>", name, val)
		}
		for name, val := range counters {
			fmt.Fprintf(w, "<li>%s (counter): %d</li>", name, val)
		}
		fmt.Fprint(w, "</ul></body></html>")
	}
}

// --- Main Entry ---
func main() {
	storage := NewMemStorage()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", updateHandler(storage))
	r.Get("/value/{type}/{name}", getValueHandler(storage))
	r.Get("/", rootHandler(storage))

	fmt.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
