// cmd/server/main.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mutualEvg/metrics-server/config"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"net/http"
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

	// Setup zerolog
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Initialize storage based on configuration priority:
	// 1. Database storage (if DATABASE_DSN is provided)
	// 2. File storage (if file storage is explicitly configured)
	// 3. Memory storage (fallback)
	var mainStorage storage.Storage
	var dbStorage *storage.DBStorage
	var err error

	if cfg.DatabaseDSN != "" {
		// Priority 1: Use database storage
		dbStorage, err = storage.NewDBStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize database storage")
		}
		mainStorage = dbStorage
		log.Info().Msg("Using PostgreSQL database storage")
	} else if cfg.UseFileStorage {
		// Priority 2: Use file storage
		memStorage := storage.NewMemStorage()
		mainStorage = memStorage

		// Setup file storage
		fileManager := storage.NewFileManager(cfg.FileStoragePath, memStorage)

		// Configure synchronous saving if store interval is 0
		syncSave := cfg.StoreInterval == 0
		memStorage.SetFileManager(fileManager, syncSave)

		// Restore data if configured
		if cfg.Restore {
			if err := fileManager.LoadFromFile(memStorage); err != nil {
				log.Error().Err(err).Msg("Failed to restore data from file")
			} else {
				log.Info().Str("file", cfg.FileStoragePath).Msg("Data restored from file")
			}
		}

		// Setup periodic saving if not synchronous
		var periodicSaver *storage.PeriodicSaver
		if !syncSave {
			periodicSaver = storage.NewPeriodicSaver(fileManager, memStorage, cfg.StoreInterval)
			periodicSaver.Start()
			log.Info().Dur("interval", cfg.StoreInterval).Msg("Started periodic saving")

			// Setup graceful shutdown for periodic saver
			defer func() {
				if periodicSaver != nil {
					periodicSaver.Stop()
					log.Info().Msg("Stopped periodic saving")
				}

				// Save final state
				if err := fileManager.SaveToFile(); err != nil {
					log.Error().Err(err).Msg("Failed to save final state")
				} else {
					log.Info().Str("file", cfg.FileStoragePath).Msg("Final state saved")
				}
			}()
		} else {
			log.Info().Msg("Synchronous saving enabled")
		}

		log.Info().Str("file", cfg.FileStoragePath).Msg("Using file storage")
	} else {
		// Priority 3: Use pure memory storage
		mainStorage = storage.NewMemStorage()
		log.Info().Msg("Using in-memory storage (no persistence)")
	}

	r := chi.NewRouter()

	// Add middleware
	r.Use(loggingMiddleware)
	r.Use(gzipmw.GzipMiddleware)

	// Database ping handler
	r.Get("/ping", pingHandler(dbStorage))

	// Legacy URL-based API
	r.Post("/update/{type}/{name}/{value}", updateHandler(mainStorage))
	r.Get("/value/{type}/{name}", valueHandler(mainStorage))

	// New JSON API
	r.Post("/update/", updateJSONHandler(mainStorage))
	r.Post("/value/", valueJSONHandler(mainStorage))

	r.Get("/", rootHandler(mainStorage))

	addr := strings.TrimPrefix(cfg.ServerAddress, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		fmt.Printf("Server running at %s\n", cfg.ServerAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Info().Msg("Shutdown signal received")

	// Close database connection if using database storage
	if dbStorage != nil {
		if err := dbStorage.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		} else {
			log.Info().Msg("Database connection closed")
		}
	}

	log.Info().Msg("Server shutdown complete")
}

// pingHandler handles the /ping endpoint to check database connectivity
func pingHandler(dbStorage *storage.DBStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dbStorage == nil {
			// No database configured
			http.Error(w, "Database not configured", http.StatusInternalServerError)
			return
		}

		if err := dbStorage.Ping(); err != nil {
			log.Error().Err(err).Msg("Database ping failed")
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
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
