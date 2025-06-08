// storage/db_storage.go
package storage

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type DBStorage struct {
	db       *sqlx.DB
	mu       sync.RWMutex
	fallback *MemStorage // Fallback to memory storage if DB is unavailable
}

// NewDBStorage creates a new database storage instance
func NewDBStorage(dsn string) (*DBStorage, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	storage := &DBStorage{
		db:       db,
		fallback: NewMemStorage(),
	}

	// Create tables if they don't exist
	if err := storage.createTables(); err != nil {
		db.Close()
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

	for _, query := range queries {
		if _, err := ds.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// UpdateGauge updates or inserts a gauge metric
func (ds *DBStorage) UpdateGauge(name string, value float64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// If database is not available, use fallback
	if ds.db == nil {
		ds.fallback.UpdateGauge(name, value)
		return
	}

	query := `INSERT INTO gauges (name, value, updated_at) 
			  VALUES ($1, $2, CURRENT_TIMESTAMP) 
			  ON CONFLICT (name) 
			  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

	if _, err := ds.db.Exec(query, name, value); err != nil {
		log.Error().Err(err).Str("name", name).Float64("value", value).Msg("Failed to update gauge in database, using fallback")
		ds.fallback.UpdateGauge(name, value)
		return
	}

	log.Debug().Str("name", name).Float64("value", value).Msg("Updated gauge in database")
}

// UpdateCounter updates or inserts a counter metric (adds to existing value)
func (ds *DBStorage) UpdateCounter(name string, value int64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// If database is not available, use fallback
	if ds.db == nil {
		ds.fallback.UpdateCounter(name, value)
		return
	}

	// First try to get existing value
	var currentValue int64
	err := ds.db.Get(&currentValue, "SELECT value FROM counters WHERE name = $1", name)
	if err != nil && err != sql.ErrNoRows {
		log.Error().Err(err).Str("name", name).Int64("value", value).Msg("Failed to get counter from database, using fallback")
		ds.fallback.UpdateCounter(name, value)
		return
	}

	newValue := currentValue + value

	query := `INSERT INTO counters (name, value, updated_at) 
			  VALUES ($1, $2, CURRENT_TIMESTAMP) 
			  ON CONFLICT (name) 
			  DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`

	if _, err := ds.db.Exec(query, name, newValue); err != nil {
		log.Error().Err(err).Str("name", name).Int64("value", value).Msg("Failed to update counter in database, using fallback")
		ds.fallback.UpdateCounter(name, value)
		return
	}

	log.Debug().Str("name", name).Int64("value", newValue).Msg("Updated counter in database")
}

// GetGauge retrieves a gauge metric
func (ds *DBStorage) GetGauge(name string) (float64, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// If database is not available, use fallback
	if ds.db == nil {
		return ds.fallback.GetGauge(name)
	}

	var value float64
	err := ds.db.Get(&value, "SELECT value FROM gauges WHERE name = $1", name)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Error().Err(err).Str("name", name).Msg("Failed to get gauge from database, trying fallback")
		return ds.fallback.GetGauge(name)
	}

	return value, true
}

// GetCounter retrieves a counter metric
func (ds *DBStorage) GetCounter(name string) (int64, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// If database is not available, use fallback
	if ds.db == nil {
		return ds.fallback.GetCounter(name)
	}

	var value int64
	err := ds.db.Get(&value, "SELECT value FROM counters WHERE name = $1", name)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		log.Error().Err(err).Str("name", name).Msg("Failed to get counter from database, trying fallback")
		return ds.fallback.GetCounter(name)
	}

	return value, true
}

// GetAll retrieves all metrics
func (ds *DBStorage) GetAll() (map[string]float64, map[string]int64) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// If database is not available, use fallback
	if ds.db == nil {
		return ds.fallback.GetAll()
	}

	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	// Get all gauges
	rows, err := ds.db.Query("SELECT name, value FROM gauges")
	if err != nil {
		log.Error().Err(err).Msg("Failed to get gauges from database, using fallback")
		return ds.fallback.GetAll()
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

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error occurred during gauge rows iteration, using fallback")
		return ds.fallback.GetAll()
	}

	// Get all counters
	rows, err = ds.db.Query("SELECT name, value FROM counters")
	if err != nil {
		log.Error().Err(err).Msg("Failed to get counters from database, using fallback")
		fallbackGauges, fallbackCounters := ds.fallback.GetAll()
		// Merge with what we got from gauges
		for k, v := range fallbackGauges {
			if _, exists := gauges[k]; !exists {
				gauges[k] = v
			}
		}
		return gauges, fallbackCounters
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

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error occurred during counter rows iteration, using fallback")
		fallbackGauges, fallbackCounters := ds.fallback.GetAll()
		// Merge with what we got from gauges and counters so far
		for k, v := range fallbackGauges {
			if _, exists := gauges[k]; !exists {
				gauges[k] = v
			}
		}
		for k, v := range fallbackCounters {
			if _, exists := counters[k]; !exists {
				counters[k] = v
			}
		}
		return gauges, counters
	}

	return gauges, counters
}

// Ping checks the database connection
func (ds *DBStorage) Ping() error {
	if ds.db == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	return ds.db.Ping()
}

// Close closes the database connection
func (ds *DBStorage) Close() error {
	if ds.db != nil {
		return ds.db.Close()
	}
	return nil
}
