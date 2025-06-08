package retry

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"syscall"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxAttempts int
	Intervals   []time.Duration
}

// DefaultConfig returns the default retry configuration
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 4, // 1 initial + 3 retries
		Intervals:   []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// Do executes a function with retry logic
func Do(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry (skip wait on first attempt)
			intervalIndex := attempt - 1
			if intervalIndex >= len(config.Intervals) {
				intervalIndex = len(config.Intervals) - 1
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(config.Intervals[intervalIndex]):
			}

			log.Info().
				Int("attempt", attempt+1).
				Int("max_attempts", config.MaxAttempts).
				Dur("waited", config.Intervals[intervalIndex]).
				Msg("Retrying operation")
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				log.Info().
					Int("attempt", attempt+1).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retriable
		if !IsRetriable(err) {
			log.Debug().
				Err(err).
				Int("attempt", attempt+1).
				Msg("Error is not retriable, stopping")
			return err
		}

		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", config.MaxAttempts).
			Msg("Retriable error occurred")
	}

	log.Error().
		Err(lastErr).
		Int("max_attempts", config.MaxAttempts).
		Msg("All retry attempts exhausted")

	return lastErr
}

// IsRetriable determines if an error can be retried
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors
	if isNetworkError(err) {
		return true
	}

	// PostgreSQL connection errors (Class 08 - Connection Exception)
	if isPostgreSQLConnectionError(err) {
		return true
	}

	// File system errors
	if isFileSystemError(err) {
		return true
	}

	// Context timeout/deadline errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	return false
}

// isNetworkError checks if the error is a network-related error
func isNetworkError(err error) bool {
	// URL errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}

	// Network operation errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Connection refused, timeout, etc.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// System call errors
	var syscallErr *syscall.Errno
	if errors.As(err, &syscallErr) {
		switch *syscallErr {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ETIMEDOUT, syscall.EHOSTUNREACH:
			return true
		}
	}

	return false
}

// isPostgreSQLConnectionError checks if the error is a PostgreSQL connection error
func isPostgreSQLConnectionError(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		// Class 08 - Connection Exception
		switch pqErr.Code {
		case pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
			pgerrcode.TransactionResolutionUnknown,
			pgerrcode.ProtocolViolation:
			return true
		}
	}

	// Generic database connection errors
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}

	return false
}

// isFileSystemError checks if the error is a file system related error
func isFileSystemError(err error) bool {
	var syscallErr *syscall.Errno
	if errors.As(err, &syscallErr) {
		switch *syscallErr {
		case syscall.EACCES, syscall.EAGAIN, syscall.EBUSY, syscall.EMFILE, syscall.ENFILE:
			return true
		}
	}

	return false
}
