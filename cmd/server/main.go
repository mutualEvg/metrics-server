// cmd/server/main.go
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mutualEvg/metrics-server/config"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	GaugeType   = "gauge"
	CounterType = "counter"
)

func main() {
	cfg := config.Load()
	memStorage := storage.NewMemStorage()

	// Setup zerolog
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	r := chi.NewRouter()

	// Add middleware
	r.Use(loggingMiddleware)
	r.Use(gzipmw.GzipMiddleware)

	// Legacy URL-based API
	r.Post("/update/{type}/{name}/{value}", updateHandler(memStorage))
	r.Get("/value/{type}/{name}", valueHandler(memStorage))

	// New JSON API
	r.Post("/update/", updateJSONHandler(memStorage))
	r.Post("/value/", valueJSONHandler(memStorage))

	r.Get("/", rootHandler(memStorage))

	addr := strings.TrimPrefix(cfg.ServerAddress, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	fmt.Printf("Server running at %s\n", cfg.ServerAddress)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status and size
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		log.Info().
			Str("method", r.Method).
			Str("uri", r.RequestURI).
			Int("status", ww.Status()).
			Int("size", ww.BytesWritten()).
			Dur("duration", duration).
			Msg("handled request")
	})
}

func updateHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")
		value := chi.URLParam(r, "value")

		switch typ {
		case GaugeType:
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				http.Error(w, "invalid gauge value", http.StatusBadRequest)
				return
			}
			s.UpdateGauge(name, v)
		case CounterType:
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				http.Error(w, "invalid counter value", http.StatusBadRequest)
				return
			}
			s.UpdateCounter(name, v)
		default:
			http.Error(w, "unknown metric type", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func valueHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")

		switch typ {
		case GaugeType:
			if v, ok := s.GetGauge(name); ok {
				w.Write([]byte(strconv.FormatFloat(v, 'f', -1, 64)))
				return
			}
		case CounterType:
			if v, ok := s.GetCounter(name); ok {
				w.Write([]byte(strconv.FormatInt(v, 10)))
				return
			}
		}

		http.Error(w, "metric not found", http.StatusNotFound)
	}
}

func rootHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, c := s.GetAll()
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Metrics</h1><ul>"))
		for k, v := range g {
			fmt.Fprintf(w, "<li>%s (gauge): %f</li>", k, v)
		}
		for k, v := range c {
			fmt.Fprintf(w, "<li>%s (counter): %d</li>", k, v)
		}
		w.Write([]byte("</ul></body></html>"))
	}
}

// updateJSONHandler handles POST /update/ with JSON body
func updateJSONHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var metric models.Metrics
		if err := json.Unmarshal(body, &metric); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if metric.ID == "" || metric.MType == "" {
			http.Error(w, "ID and MType are required", http.StatusBadRequest)
			return
		}

		switch metric.MType {
		case GaugeType:
			if metric.Value == nil {
				http.Error(w, "Value is required for gauge metrics", http.StatusBadRequest)
				return
			}
			s.UpdateGauge(metric.ID, *metric.Value)
			// Return the updated metric
			response := models.Metrics{
				ID:    metric.ID,
				MType: metric.MType,
				Value: metric.Value,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case CounterType:
			if metric.Delta == nil {
				http.Error(w, "Delta is required for counter metrics", http.StatusBadRequest)
				return
			}
			s.UpdateCounter(metric.ID, *metric.Delta)
			// Get the updated value from storage
			if updatedValue, ok := s.GetCounter(metric.ID); ok {
				response := models.Metrics{
					ID:    metric.ID,
					MType: metric.MType,
					Delta: &updatedValue,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			} else {
				http.Error(w, "Failed to retrieve updated counter value", http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Unknown metric type", http.StatusBadRequest)
			return
		}
	}
}

// valueJSONHandler handles POST /value/ with JSON body
func valueJSONHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var metric models.Metrics
		if err := json.Unmarshal(body, &metric); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if metric.ID == "" || metric.MType == "" {
			http.Error(w, "ID and MType are required", http.StatusBadRequest)
			return
		}

		switch metric.MType {
		case GaugeType:
			if value, ok := s.GetGauge(metric.ID); ok {
				response := models.Metrics{
					ID:    metric.ID,
					MType: metric.MType,
					Value: &value,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			} else {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}

		case CounterType:
			if value, ok := s.GetCounter(metric.ID); ok {
				response := models.Metrics{
					ID:    metric.ID,
					MType: metric.MType,
					Delta: &value,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			} else {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}

		default:
			http.Error(w, "Unknown metric type", http.StatusBadRequest)
			return
		}
	}
}
