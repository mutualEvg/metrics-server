package retry

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

func TestRetryLogic(t *testing.T) {
	config := RetryConfig{
		MaxAttempts: 3,
		Intervals:   []time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
	}

	t.Run("Success_on_first_attempt", func(t *testing.T) {
		attempts := 0
		err := Do(context.Background(), config, func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Success_after_retries", func(t *testing.T) {
		attempts := 0
		err := Do(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Non-retriable_error", func(t *testing.T) {
		attempts := 0
		err := Do(context.Background(), config, func() error {
			attempts++
			return errors.New("non-retriable error")
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Max_attempts_exhausted", func(t *testing.T) {
		attempts := 0
		err := Do(context.Background(), config, func() error {
			attempts++
			return &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})
}

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "network error",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
			expected: true,
		},
		{
			name:     "PostgreSQL connection error",
			err:      &pq.Error{Code: pgerrcode.ConnectionFailure},
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "file system error",
			err:      syscall.EACCES,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetriable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetriable(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}
