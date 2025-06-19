// cmd/server/main.go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mutualEvg/metrics-server/config"
	"github.com/mutualEvg/metrics-server/internal/handlers"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"net/http"
	"strings"
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
	r.Get("/ping", handlers.PingHandler(dbStorage))

	// Legacy URL-based API
	r.Post("/update/{type}/{name}/{value}", handlers.UpdateHandler(mainStorage))
	r.Get("/value/{type}/{name}", handlers.ValueHandler(mainStorage))

	// New JSON API with Content-Type middleware - use exact paths to avoid conflicts
	r.With(gzipmw.RequireContentType("application/json")).Post("/update/", handlers.UpdateJSONHandler(mainStorage))
	r.With(gzipmw.RequireContentType("application/json")).Post("/value/", handlers.ValueJSONHandler(mainStorage))
	r.With(gzipmw.RequireContentType("application/json")).Post("/updates/", handlers.UpdateBatchHandler(mainStorage))

	r.Get("/", handlers.RootHandler(mainStorage))

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
