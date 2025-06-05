package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileManager_SaveAndLoad(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_metrics.json")

	// Create storage and file manager
	storage := NewMemStorage()
	fileManager := NewFileManager(filePath)

	// Add some test data
	storage.UpdateGauge("test_gauge", 123.45)
	storage.UpdateCounter("test_counter", 42)
	storage.UpdateCounter("test_counter", 8) // Should be 50 total

	// Save to file
	err := fileManager.SaveToFile(storage)
	if err != nil {
		t.Fatalf("Failed to save to file: %v", err)
	}

	// Verify file exists
	if !fileManager.FileExists() {
		t.Error("File should exist after saving")
	}

	// Create new storage and load data
	newStorage := NewMemStorage()
	err = fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	// Verify loaded data
	if gauge, ok := newStorage.GetGauge("test_gauge"); !ok || gauge != 123.45 {
		t.Errorf("Expected gauge value 123.45, got %f", gauge)
	}

	if counter, ok := newStorage.GetCounter("test_counter"); !ok || counter != 50 {
		t.Errorf("Expected counter value 50, got %d", counter)
	}
}

func TestFileManager_LoadNonexistentFile(t *testing.T) {
	// Create file manager with non-existent file
	fileManager := NewFileManager("/nonexistent/path/file.json")
	storage := NewMemStorage()

	// Should not return error for non-existent file
	err := fileManager.LoadFromFile(storage)
	if err != nil {
		t.Errorf("Loading non-existent file should not return error, got: %v", err)
	}

	// Storage should be empty
	gauges, counters := storage.GetAll()
	if len(gauges) != 0 || len(counters) != 0 {
		t.Error("Storage should be empty when loading non-existent file")
	}
}

func TestMemStorage_SynchronousSaving(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sync_test.json")

	// Create storage with synchronous saving
	storage := NewMemStorage()
	fileManager := NewFileManager(filePath)
	storage.SetFileManager(fileManager, true) // Enable sync save

	// Update metrics - should save immediately
	storage.UpdateGauge("sync_gauge", 99.99)
	storage.UpdateCounter("sync_counter", 10)

	// Verify file was created and contains data
	if !fileManager.FileExists() {
		t.Error("File should exist after synchronous save")
	}

	// Load into new storage to verify
	newStorage := NewMemStorage()
	err := fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	if gauge, ok := newStorage.GetGauge("sync_gauge"); !ok || gauge != 99.99 {
		t.Errorf("Expected gauge value 99.99, got %f", gauge)
	}

	if counter, ok := newStorage.GetCounter("sync_counter"); !ok || counter != 10 {
		t.Errorf("Expected counter value 10, got %d", counter)
	}
}

func TestPeriodicSaver(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "periodic_test.json")

	// Create storage and file manager
	storage := NewMemStorage()
	fileManager := NewFileManager(filePath)

	// Create periodic saver with short interval
	saver := NewPeriodicSaver(fileManager, storage, 100*time.Millisecond)
	saver.Start()
	defer saver.Stop()

	// Add some data
	storage.UpdateGauge("periodic_gauge", 77.77)
	storage.UpdateCounter("periodic_counter", 5)

	// Wait for periodic save
	time.Sleep(200 * time.Millisecond)

	// Verify file was created
	if !fileManager.FileExists() {
		t.Error("File should exist after periodic save")
	}

	// Load and verify data
	newStorage := NewMemStorage()
	err := fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	if gauge, ok := newStorage.GetGauge("periodic_gauge"); !ok || gauge != 77.77 {
		t.Errorf("Expected gauge value 77.77, got %f", gauge)
	}

	if counter, ok := newStorage.GetCounter("periodic_counter"); !ok || counter != 5 {
		t.Errorf("Expected counter value 5, got %d", counter)
	}
}

func TestPeriodicSaver_SaveNow(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "save_now_test.json")

	// Create storage and file manager
	storage := NewMemStorage()
	fileManager := NewFileManager(filePath)

	// Create periodic saver (but don't start it)
	saver := NewPeriodicSaver(fileManager, storage, time.Hour) // Long interval

	// Add some data
	storage.UpdateGauge("immediate_gauge", 55.55)

	// Save immediately
	err := saver.SaveNow()
	if err != nil {
		t.Fatalf("SaveNow failed: %v", err)
	}

	// Verify file was created
	if !fileManager.FileExists() {
		t.Error("File should exist after SaveNow")
	}

	// Load and verify data
	newStorage := NewMemStorage()
	err = fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	if gauge, ok := newStorage.GetGauge("immediate_gauge"); !ok || gauge != 55.55 {
		t.Errorf("Expected gauge value 55.55, got %f", gauge)
	}
}

func TestFileStorage_JSONFormat(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "json_test.json")

	// Create storage and add data
	storage := NewMemStorage()
	storage.UpdateGauge("json_gauge", 123.456)
	storage.UpdateCounter("json_counter", 789)

	// Save to file
	fileManager := NewFileManager(filePath)
	err := fileManager.SaveToFile(storage)
	if err != nil {
		t.Fatalf("Failed to save to file: %v", err)
	}

	// Read file content and verify JSON structure
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Should contain expected JSON structure
	expectedSubstrings := []string{
		`"gauges"`,
		`"counters"`,
		`"json_gauge"`,
		`"json_counter"`,
		`123.456`,
		`789`,
	}

	contentStr := string(content)
	for _, substr := range expectedSubstrings {
		if !strings.Contains(contentStr, substr) {
			t.Errorf("File content should contain %s", substr)
		}
	}
}
