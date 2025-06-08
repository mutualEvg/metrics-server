// storage/db_storage_test.go
package storage

import (
	"testing"

	_ "github.com/lib/pq"
)

// TestDBStorageBasicOperations tests basic database operations
// Note: This test requires a PostgreSQL database to be running
// Skip this test if no database is available
func TestDBStorageBasicOperations(t *testing.T) {
	// This is a basic test that can be run if a test database is available
	// For now, we'll test the fallback functionality
	dsn := "postgres://invalid:invalid@localhost/invalid?sslmode=disable"

	// This should fail to connect, which is expected for this test
	_, err := NewDBStorage(dsn)
	if err == nil {
		t.Error("Expected error when connecting to invalid database")
	}
}

// TestDBStorageInterface verifies that DBStorage implements the Storage interface
func TestDBStorageInterface(t *testing.T) {
	// Create a mock DBStorage to test interface compliance
	dbStorage := &DBStorage{
		db:       nil, // We won't actually use the db for this test
		fallback: NewMemStorage(),
	}

	// Test that it implements the Storage interface
	var _ Storage = dbStorage

	// Test fallback operations when db is nil
	dbStorage.UpdateGauge("test_gauge", 42.5)
	dbStorage.UpdateCounter("test_counter", 10)

	// These should work through the fallback
	if val, ok := dbStorage.fallback.GetGauge("test_gauge"); !ok || val != 42.5 {
		t.Errorf("Expected gauge value 42.5, got %f", val)
	}

	if val, ok := dbStorage.fallback.GetCounter("test_counter"); !ok || val != 10 {
		t.Errorf("Expected counter value 10, got %d", val)
	}
}

// TestPingWithoutDB tests the Ping method when no database is connected
func TestPingWithoutDB(t *testing.T) {
	dbStorage := &DBStorage{
		db:       nil,
		fallback: NewMemStorage(),
	}

	err := dbStorage.Ping()
	if err == nil {
		t.Error("Expected error when pinging without database connection")
	}
}

// TestCloseWithoutDB tests the Close method when no database is connected
func TestCloseWithoutDB(t *testing.T) {
	dbStorage := &DBStorage{
		db:       nil,
		fallback: NewMemStorage(),
	}

	err := dbStorage.Close()
	if err != nil {
		t.Errorf("Expected no error when closing without database connection, got: %v", err)
	}
}
