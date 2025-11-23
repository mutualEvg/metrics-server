// cmd/server/main.go
package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mutualEvg/metrics-server/config"
	"github.com/mutualEvg/metrics-server/internal/audit"
	"github.com/mutualEvg/metrics-server/internal/crypto"
	"github.com/mutualEvg/metrics-server/internal/handlers"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func main() {
	printBuildInfo()

	cfg := config.Load()

	// Setup zerolog
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Initialize storage based on configuration priority:
	// 1. Database storage (if DATABASE_DSN is provided)
	// 2. File storage (if file storage is explicitly configured)
	// 3. Memory storage (fallback)
	var mainStorage storage.Storage
	var dbStorage *storage.DBStorage
	var periodicSaver *storage.PeriodicSaver
	var fileManager *storage.FileManager
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
		fileManager = storage.NewFileManager(cfg.FileStoragePath, memStorage)

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
		if !syncSave {
			periodicSaver = storage.NewPeriodicSaver(fileManager, memStorage, cfg.StoreInterval)
			periodicSaver.Start()
			log.Info().Dur("interval", cfg.StoreInterval).Msg("Started periodic saving")
		} else {
			log.Info().Msg("Synchronous saving enabled")
		}

		log.Info().Str("file", cfg.FileStoragePath).Msg("Using file storage")
	} else {
		// Priority 3: Use pure memory storage
		mainStorage = storage.NewMemStorage()
		log.Info().Msg("Using in-memory storage (no persistence)")
	}

	// Initialize audit system
	auditSubject := audit.NewSubject()

	// Configure file auditor if specified
	if cfg.AuditFile != "" {
		fileAuditor, err := audit.NewFileAuditor(cfg.AuditFile)
		if err != nil {
			log.Error().Err(err).Str("file", cfg.AuditFile).Msg("Failed to initialize file auditor")
		} else {
			auditSubject.Attach(fileAuditor)
			log.Info().Str("file", cfg.AuditFile).Msg("File audit logging enabled")
		}
	}

	// Configure remote auditor if specified
	if cfg.AuditURL != "" {
		remoteAuditor, err := audit.NewRemoteAuditor(cfg.AuditURL)
		if err != nil {
			log.Error().Err(err).Str("url", cfg.AuditURL).Msg("Failed to initialize remote auditor")
		} else {
			auditSubject.Attach(remoteAuditor)
			log.Info().Str("url", cfg.AuditURL).Msg("Remote audit logging enabled")
		}
	}

	if !auditSubject.HasObservers() {
		log.Info().Msg("Audit logging is disabled (no audit-file or audit-url configured)")
	}

	r := chi.NewRouter()

	// Add middleware
	r.Use(loggingMiddleware)

	// Add trusted subnet middleware if configured
	if cfg.TrustedSubnet != "" {
		r.Use(gzipmw.TrustedSubnetMiddleware(cfg.TrustedSubnet))
		log.Info().Str("trusted_subnet", cfg.TrustedSubnet).Msg("Trusted subnet validation enabled")
	} else {
		log.Info().Msg("Trusted subnet validation disabled (all IPs allowed)")
	}

	// Add decryption middleware if crypto key is configured
	if cfg.CryptoKey != "" {
		privateKey, err := loadPrivateKey(cfg.CryptoKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load private key for decryption")
		}
		r.Use(gzipmw.DecryptionMiddleware(privateKey))
		log.Info().Str("key_path", cfg.CryptoKey).Msg("Asymmetric decryption enabled")
	}

	// Add hash middleware BEFORE gzip middleware so it can verify compressed data
	if cfg.Key != "" {
		log.Info().Msg("SHA256 hash verification enabled")
		r.Use(gzipmw.HashVerification(cfg.Key))
		r.Use(gzipmw.ResponseHash(cfg.Key))
	}

	r.Use(gzipmw.GzipMiddleware)

	// Database ping handler
	r.Get("/ping", handlers.PingHandler(dbStorage))

	// Legacy URL-based API
	r.Post("/update/{type}/{name}/{value}", handlers.UpdateHandler(mainStorage))
	r.Get("/value/{type}/{name}", handlers.ValueHandler(mainStorage))

	// New JSON API with Content-Type middleware - use exact paths to avoid conflicts
	r.With(gzipmw.RequireContentType("application/json")).Post("/update/", handlers.UpdateJSONHandler(mainStorage, auditSubject))
	r.With(gzipmw.RequireContentType("application/json")).Post("/value/", handlers.ValueJSONHandler(mainStorage, auditSubject))
	r.With(gzipmw.RequireContentType("application/json")).Post("/updates/", handlers.UpdateBatchHandler(mainStorage, auditSubject))

	r.Get("/", handlers.RootHandler(mainStorage))

	addr := strings.TrimPrefix(cfg.ServerAddress, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	// Setup graceful shutdown - handle SIGTERM, SIGINT, SIGQUIT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

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
	sig := <-sigChan
	log.Info().Msgf("Shutdown signal received: %v", sig)

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server gracefully (waits for in-flight requests to complete)
	log.Info().Msg("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	} else {
		log.Info().Msg("HTTP server stopped gracefully")
	}

	// Save final state if using file storage with periodic saver
	if periodicSaver != nil {
		log.Info().Msg("Stopping periodic saver...")
		periodicSaver.Stop()
		log.Info().Msg("Saving final state...")
		if err := fileManager.SaveToFile(); err != nil {
			log.Error().Err(err).Msg("Failed to save final state")
		} else {
			log.Info().Str("file", cfg.FileStoragePath).Msg("Final state saved")
		}
	}

	// Close database connection if using database storage
	if dbStorage != nil {
		log.Info().Msg("Closing database connection...")
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

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	privateKey, err := crypto.LoadPrivateKeyFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key from %s: %w", path, err)
	}
	return privateKey, nil
}
