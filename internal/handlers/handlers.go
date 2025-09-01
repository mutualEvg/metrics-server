package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
	"github.com/rs/zerolog/log"
)

const (
	// GaugeType represents floating-point metrics that can be set to any value
	GaugeType = "gauge"

	// CounterType represents integer metrics that accumulate values over time
	CounterType = "counter"
)

// PingHandler handles the /ping endpoint to check database connectivity
func PingHandler(dbStorage *storage.DBStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dbStorage == nil {
			// No database configured
			http.Error(w, "Database not configured", http.StatusServiceUnavailable)
			return
		}

		if err := dbStorage.Ping(); err != nil {
			log.Error().Err(err).Msg("Database ping failed")
			http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// UpdateHandler handles legacy URL-based metric updates via POST requests.
// URL format: /update/{type}/{name}/{value}
// Supports both "gauge" and "counter" metric types.
func UpdateHandler(s storage.Storage) http.HandlerFunc {
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

// ValueHandler handles legacy URL-based metric retrieval via GET requests.
// URL format: /value/{type}/{name}
// Returns the metric value as plain text or 404 if not found.
func ValueHandler(s storage.Storage) http.HandlerFunc {
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

// RootHandler handles the root endpoint showing all metrics in HTML format.
// Returns an HTML page listing all gauge and counter metrics.
func RootHandler(s storage.Storage) http.HandlerFunc {
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

// UpdateJSONHandler handles JSON-based metric updates via POST /update/.
// Accepts a single metric in JSON format and returns the updated metric.
func UpdateJSONHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

// ValueJSONHandler handles JSON-based metric retrieval via POST /value/.
// Accepts a metric ID and type in JSON format and returns the current value.
func ValueJSONHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

// UpdateBatchHandler handles batch metric updates via POST /updates/.
// Accepts an array of metrics in JSON format and processes them atomically.
// Uses database transactions for DBStorage, sequential processing for others.
func UpdateBatchHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var metrics []models.Metrics
		if err := json.Unmarshal(body, &metrics); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Don't process empty batches
		if len(metrics) == 0 {
			http.Error(w, "Empty batch not allowed", http.StatusBadRequest)
			return
		}

		// Check if we have database storage for transaction support
		if dbStorage, ok := s.(*storage.DBStorage); ok {
			// Use database transaction for batch processing
			if err := dbStorage.UpdateBatch(metrics); err != nil {
				log.Error().Err(err).Msg("Failed to process batch update in database")
				http.Error(w, "Failed to process batch update", http.StatusInternalServerError)
				return
			}
		} else {
			// For memory/file storage, process sequentially with proper locking
			for _, metric := range metrics {
				// Validate required fields
				if metric.ID == "" || metric.MType == "" {
					http.Error(w, "ID and MType are required for all metrics", http.StatusBadRequest)
					return
				}

				switch metric.MType {
				case GaugeType:
					if metric.Value == nil {
						http.Error(w, "Value is required for gauge metrics", http.StatusBadRequest)
						return
					}
					s.UpdateGauge(metric.ID, *metric.Value)

				case CounterType:
					if metric.Delta == nil {
						http.Error(w, "Delta is required for counter metrics", http.StatusBadRequest)
						return
					}
					s.UpdateCounter(metric.ID, *metric.Delta)

				default:
					http.Error(w, "Unknown metric type: "+metric.MType, http.StatusBadRequest)
					return
				}
			}
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return the processed metrics (optional, for confirmation)
		response := make([]models.Metrics, 0, len(metrics))
		for _, metric := range metrics {
			switch metric.MType {
			case GaugeType:
				if value, ok := s.GetGauge(metric.ID); ok {
					response = append(response, models.Metrics{
						ID:    metric.ID,
						MType: metric.MType,
						Value: &value,
					})
				}
			case CounterType:
				if value, ok := s.GetCounter(metric.ID); ok {
					response = append(response, models.Metrics{
						ID:    metric.ID,
						MType: metric.MType,
						Delta: &value,
					})
				}
			}
		}

		json.NewEncoder(w).Encode(response)
	}
}
