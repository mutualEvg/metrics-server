package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

func TestFileStorageIntegration(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "integration_test.json")

	// Create storage with file manager
	memStorage := storage.NewMemStorage()
	fileManager := storage.NewFileManager(filePath, memStorage)
	memStorage.SetFileManager(fileManager, false) // Async saving

	// Setup router
	router := chi.NewRouter()
	router.Use(gzipmw.GzipMiddleware)
	router.Post("/update/", updateJSONHandler(memStorage))
	router.Post("/value/", valueJSONHandler(memStorage))

	// Add some metrics via API
	testMetrics := []models.Metrics{
		{
			ID:    "test_gauge",
			MType: "gauge",
			Value: func() *float64 { v := 123.45; return &v }(),
		},
		{
			ID:    "test_counter",
			MType: "counter",
			Delta: func() *int64 { v := int64(42); return &v }(),
		},
	}

	for _, metric := range testMetrics {
		jsonData, _ := json.Marshal(metric)
		req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	}

	// Manually save to file
	err := fileManager.SaveToFile()
	if err != nil {
		t.Fatalf("Failed to save to file: %v", err)
	}

	// Verify file exists
	if !fileManager.FileExists() {
		t.Error("File should exist after saving")
	}

	// Create new storage and load from file
	newStorage := storage.NewMemStorage()
	err = fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	// Verify loaded data via API
	router2 := chi.NewRouter()
	router2.Use(gzipmw.GzipMiddleware)
	router2.Post("/value/", valueJSONHandler(newStorage))

	// Test gauge retrieval
	gaugeReq := models.Metrics{
		ID:    "test_gauge",
		MType: "gauge",
	}
	jsonData, _ := json.Marshal(gaugeReq)
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router2.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for gauge retrieval, got %d", rec.Code)
	}

	var gaugeResp models.Metrics
	err = json.Unmarshal(rec.Body.Bytes(), &gaugeResp)
	if err != nil {
		t.Fatalf("Failed to unmarshal gauge response: %v", err)
	}

	if gaugeResp.Value == nil || *gaugeResp.Value != 123.45 {
		t.Errorf("Expected gauge value 123.45, got %v", gaugeResp.Value)
	}

	// Test counter retrieval
	counterReq := models.Metrics{
		ID:    "test_counter",
		MType: "counter",
	}
	jsonData, _ = json.Marshal(counterReq)
	req = httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	router2.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for counter retrieval, got %d", rec.Code)
	}

	var counterResp models.Metrics
	err = json.Unmarshal(rec.Body.Bytes(), &counterResp)
	if err != nil {
		t.Fatalf("Failed to unmarshal counter response: %v", err)
	}

	if counterResp.Delta == nil || *counterResp.Delta != 42 {
		t.Errorf("Expected counter value 42, got %v", counterResp.Delta)
	}
}

func TestSynchronousFileStorage(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sync_integration_test.json")

	// Create storage with synchronous file saving
	memStorage := storage.NewMemStorage()
	fileManager := storage.NewFileManager(filePath, memStorage)
	memStorage.SetFileManager(fileManager, true) // Sync saving

	// Setup router
	router := chi.NewRouter()
	router.Post("/update/", updateJSONHandler(memStorage))

	// Add a metric - should save immediately
	metric := models.Metrics{
		ID:    "sync_gauge",
		MType: "gauge",
		Value: func() *float64 { v := 99.99; return &v }(),
	}

	jsonData, _ := json.Marshal(metric)
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// File should exist immediately due to synchronous saving
	if !fileManager.FileExists() {
		t.Error("File should exist immediately after synchronous save")
	}

	// Verify file content
	newStorage := storage.NewMemStorage()
	err := fileManager.LoadFromFile(newStorage)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	if gauge, ok := newStorage.GetGauge("sync_gauge"); !ok || gauge != 99.99 {
		t.Errorf("Expected gauge value 99.99, got %f", gauge)
	}
}
