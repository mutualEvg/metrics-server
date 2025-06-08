// storage/db_storage.go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
	"github.com/rs/zerolog/log"
)

type DBStorage struct {
	db          *sqlx.DB
	mu          sync.RWMutex
	retryConfig retry.RetryConfig
}

// NewDBStorage creates a new database storage instance
func NewDBStorage(dsn string) (*DBStorage, error) {
	storage := &DBStorage{
		retryConfig: retry.DefaultConfig(),
	}

	// Connect to database with retry logic
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := retry.Do(ctx, storage.retryConfig, func() error {
		db, err := sqlx.Connect("postgres", dsn)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		storage.db = db
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	if err := storage.createTables(); err != nil {
		storage.db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	log.Info().Msg("Database storage initialized successfully")
	return storage, nil
}

// createTables creates the necessary tables for storing metrics
func (ds *DBStorage) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS gauges (
			name VARCHAR(255) PRIMARY KEY,
			value DOUBLE PRECISION NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS counters (
			name VARCHAR(255) PRIMARY KEY,
			value BIGINT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, query := range queries {
		err := retry.Do(ctx, ds.retryConfig, func() error {
			_, err := ds.db.Exec(query)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// UpdateGauge updates or inserts a gauge metric
func (ds *DBStorage) UpdateGauge(name string, value float64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.db == nil {
		log.Error().Str("name", name).Float64("value", value).Msg("Database connection is nil, cannot update gauge")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO gauges (name, value, updated_at) 
			  VALUES ($1, $2, CURRENT_TIMESTAMP) 
			  ON CONFLICT (name) 
			  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

	err := retry.Do(ctx, ds.retryConfig, func() error {
		_, err := ds.db.Exec(query, name, value)
		return err
	})

	if err != nil {
		log.Error().Err(err).Str("name", name).Float64("value", value).Msg("Failed to update gauge in database after retries")
		return
	}

	log.Debug().Str("name", name).Float64("value", value).Msg("Updated gauge in database")
}

// UpdateCounter updates or inserts a counter metric (adds to existing value)
func (ds *DBStorage) UpdateCounter(name string, value int64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.db == nil {
		log.Error().Str("name", name).Int64("value", value).Msg("Database connection is nil, cannot update counter")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := retry.Do(ctx, ds.retryConfig, func() error {
		// First try to get existing value
		var currentValue int64
		err := ds.db.Get(&currentValue, "SELECT value FROM counters WHERE name = $1", name)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get counter from database: %w", err)
		}

		newValue := currentValue + value

		query := `INSERT INTO counters (name, value, updated_at) 
				  VALUES ($1, $2, CURRENT_TIMESTAMP) 
				  ON CONFLICT (name) 
				  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

		_, err = ds.db.Exec(query, name, newValue)
		return err
	})

	if err != nil {
		log.Error().Err(err).Str("name", name).Int64("value", value).Msg("Failed to update counter in database after retries")
		return
	}

	log.Debug().Str("name", name).Int64("value", value).Msg("Updated counter in database")
}

// GetGauge retrieves a gauge metric
func (ds *DBStorage) GetGauge(name string) (float64, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if ds.db == nil {
		log.Error().Str("name", name).Msg("Database connection is nil, cannot get gauge")
		return 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var value float64
	err := retry.Do(ctx, ds.retryConfig, func() error {
		return ds.db.Get(&value, "SELECT value FROM gauges WHERE name = $1", name)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Error().Err(err).Str("name", name).Msg("Failed to get gauge from database after retries")
		return 0, false
	}

	return value, true
}

// GetCounter retrieves a counter metric
func (ds *DBStorage) GetCounter(name string) (int64, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if ds.db == nil {
		log.Error().Str("name", name).Msg("Database connection is nil, cannot get counter")
		return 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var value int64
	err := retry.Do(ctx, ds.retryConfig, func() error {
		return ds.db.Get(&value, "SELECT value FROM counters WHERE name = $1", name)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Error().Err(err).Str("name", name).Msg("Failed to get counter from database after retries")
		return 0, false
	}

	return value, true
}

// GetAll retrieves all metrics
func (ds *DBStorage) GetAll() (map[string]float64, map[string]int64) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	if ds.db == nil {
		log.Error().Msg("Database connection is nil, cannot get all metrics")
		return gauges, counters
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all gauges with retry
	err := retry.Do(ctx, ds.retryConfig, func() error {
		rows, err := ds.db.Query("SELECT name, value FROM gauges")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			var value float64
			if err := rows.Scan(&name, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan gauge row")
				continue
			}
			gauges[name] = value
		}

		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get gauges from database after retries")
		return gauges, counters
	}

	// Get all counters with retry
	err = retry.Do(ctx, ds.retryConfig, func() error {
		rows, err := ds.db.Query("SELECT name, value FROM counters")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			var value int64
			if err := rows.Scan(&name, &value); err != nil {
				log.Error().Err(err).Msg("Failed to scan counter row")
				continue
			}
			counters[name] = value
		}

		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get counters from database after retries")
	}

	return gauges, counters
}

// Ping checks the database connection
func (ds *DBStorage) Ping() error {
	if ds.db == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return retry.Do(ctx, ds.retryConfig, func() error {
		return ds.db.Ping()
	})
}

// Close closes the database connection
func (ds *DBStorage) Close() error {
	if ds.db != nil {
		return ds.db.Close()
	}
	return nil
}

// UpdateBatch processes multiple metrics in a single database transaction
func (ds *DBStorage) UpdateBatch(metrics []models.Metrics) error {
	if ds.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return retry.Do(ctx, ds.retryConfig, func() error {
		// Start a transaction
		tx, err := ds.db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback() // Will be ignored if tx.Commit() succeeds

		ds.mu.Lock()
		defer ds.mu.Unlock()

		// Process each metric in the transaction
		for _, metric := range metrics {
			// Validate required fields
			if metric.ID == "" || metric.MType == "" {
				return fmt.Errorf("metric ID and type are required")
			}

			switch metric.MType {
			case "gauge":
				if metric.Value == nil {
					return fmt.Errorf("gauge value is required for metric %s", metric.ID)
				}

				query := `INSERT INTO gauges (name, value, updated_at) 
						  VALUES ($1, $2, CURRENT_TIMESTAMP) 
						  ON CONFLICT (name) 
						  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

				if _, err := tx.Exec(query, metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("failed to update gauge %s: %w", metric.ID, err)
				}

			case "counter":
				if metric.Delta == nil {
					return fmt.Errorf("counter delta is required for metric %s", metric.ID)
				}

				// Get current value within transaction
				var currentValue int64
				err := tx.Get(&currentValue, "SELECT value FROM counters WHERE name = $1", metric.ID)
				if err != nil && err != sql.ErrNoRows {
					return fmt.Errorf("failed to get current counter value for %s: %w", metric.ID, err)
				}

				newValue := currentValue + *metric.Delta

				query := `INSERT INTO counters (name, value, updated_at) 
						  VALUES ($1, $2, CURRENT_TIMESTAMP) 
						  ON CONFLICT (name) 
						  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

				if _, err := tx.Exec(query, metric.ID, newValue); err != nil {
					return fmt.Errorf("failed to update counter %s: %w", metric.ID, err)
				}

			default:
				return fmt.Errorf("unknown metric type: %s", metric.MType)
			}
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		return nil
	})
}
