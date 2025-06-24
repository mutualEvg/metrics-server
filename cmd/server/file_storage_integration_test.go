//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

func TestFileStorageIntegration(t *testing.T) {
	port := 18120
	storageFile := "/tmp/demo-metrics.json"
	legacyFile := "/tmp/legacy-test.json"
	periodicFile := "/tmp/test-metrics.json"

	// Clean up files
	_ = os.Remove(storageFile)
	_ = os.Remove(legacyFile)
	_ = os.Remove(periodicFile)

	t.Run("Synchronous file storage and retrieval", func(t *testing.T) {
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		cmd.Env = append(os.Environ(),
			"STORE_INTERVAL=0",
			"FILE_STORAGE_PATH="+storageFile,
			"RESTORE=true",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		time.Sleep(2 * time.Second)

		// Send gauge
		sendMetricJSON(t, port, "test_gauge", "gauge", 123.45, 0)
		// Send counter
		sendMetricJSON(t, port, "test_counter", "counter", 0, 42)
		// Send another counter increment
		sendMetricJSON(t, port, "test_counter", "counter", 0, 8)
		time.Sleep(1 * time.Second)

		// Retrieve gauge
		val := getMetricJSON(t, port, "test_gauge", "gauge")
		if val == "" {
			t.Errorf("Gauge not retrieved")
		}
		// Retrieve counter
		val = getMetricJSON(t, port, "test_counter", "counter")
		if val == "" {
			t.Errorf("Counter not retrieved")
		}

		// Check file exists
		if _, err := os.Stat(storageFile); err != nil {
			t.Errorf("Expected file %s to exist", storageFile)
		}
	})

	t.Run("Periodic saving", func(t *testing.T) {
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		cmd.Env = append(os.Environ(),
			"STORE_INTERVAL=2",
			"FILE_STORAGE_PATH="+periodicFile,
			"RESTORE=false",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		time.Sleep(2 * time.Second)

		sendMetricJSON(t, port, "periodic_gauge", "gauge", 77.77, 0)
		sendMetricJSON(t, port, "periodic_counter", "counter", 0, 5)
		time.Sleep(3 * time.Second)

		if _, err := os.Stat(periodicFile); err != nil {
			t.Errorf("Expected periodic file %s to exist", periodicFile)
		}
	})

	t.Run("Restoration", func(t *testing.T) {
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		cmd.Env = append(os.Environ(),
			"STORE_INTERVAL=300",
			"FILE_STORAGE_PATH="+periodicFile,
			"RESTORE=true",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		time.Sleep(2 * time.Second)

		val := getMetricJSON(t, port, "periodic_gauge", "gauge")
		if val == "" {
			t.Errorf("Restored gauge not found")
		}
		val = getMetricJSON(t, port, "periodic_counter", "counter")
		if val == "" {
			t.Errorf("Restored counter not found")
		}
	})

	t.Run("Legacy API with file storage", func(t *testing.T) {
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		cmd.Env = append(os.Environ(),
			"STORE_INTERVAL=0",
			"FILE_STORAGE_PATH="+legacyFile,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()
		time.Sleep(2 * time.Second)

		// Send gauge via legacy API
		http.Post("http://localhost:"+strconv.Itoa(port)+"/update/gauge/legacy_gauge/99.99", "text/plain", nil)
		// Send counter via legacy API
		http.Post("http://localhost:"+strconv.Itoa(port)+"/update/counter/legacy_counter/10", "text/plain", nil)
		time.Sleep(1 * time.Second)

		// Get values via legacy API
		resp, _ := http.Get("http://localhost:" + strconv.Itoa(port) + "/value/gauge/legacy_gauge")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(strings.TrimSpace(string(body))) == 0 {
			t.Errorf("Legacy gauge not found")
		}
		resp, _ = http.Get("http://localhost:" + strconv.Itoa(port) + "/value/counter/legacy_counter")
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(strings.TrimSpace(string(body))) == 0 {
			t.Errorf("Legacy counter not found")
		}
		if _, err := os.Stat(legacyFile); err != nil {
			t.Errorf("Expected legacy file %s to exist", legacyFile)
		}
	})
}

func sendMetricJSON(t *testing.T, port int, id, mtype string, value float64, delta int64) {
	url := "http://localhost:" + strconv.Itoa(port) + "/update/"
	payload := map[string]interface{}{
		"id":   id,
		"type": mtype,
	}
	if mtype == "gauge" {
		payload["value"] = value
	} else if mtype == "counter" {
		payload["delta"] = delta
	}
	body, _ := json.Marshal(payload)
	t.Logf("Sending to %s: %s", url, string(body))
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Errorf("Failed to send metric: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200, got %d. Response: %s", resp.StatusCode, string(respBody))
	}
}

func getMetricJSON(t *testing.T, port int, id, mtype string) string {
	url := "http://localhost:" + strconv.Itoa(port) + "/value/"
	payload := map[string]interface{}{
		"id":   id,
		"type": mtype,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Errorf("Failed to get metric: %v", err)
		return ""
	}
	if resp == nil {
		t.Errorf("Response is nil for %s", url)
		return ""
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
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
