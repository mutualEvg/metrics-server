//go:build integration

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBatchAPI(t *testing.T) {
	port := 18095
	server := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
	server.Stdout = os.Stdout
	server.Stderr = os.Stderr
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = server.Process.Kill()
		_ = server.Wait()
	}()
	time.Sleep(2 * time.Second)

	client := &http.Client{}

	// Debug: Test if server is responding at all
	t.Run("Debug: Test server is responding", func(t *testing.T) {
		resp, err := client.Get("http://localhost:" + strconv.Itoa(port) + "/")
		if err != nil {
			t.Fatalf("Server not responding to root request: %v", err)
		}
		defer resp.Body.Close()
		t.Logf("Root endpoint status: %d", resp.StatusCode)
	})

	// Debug: Test /update/ endpoint works
	t.Run("Debug: Test /update/ endpoint", func(t *testing.T) {
		payload := `{"id":"debug_test","type":"gauge","value":1.0}`
		resp, err := client.Post("http://localhost:"+strconv.Itoa(port)+"/update/", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Single update request failed: %v", err)
		}
		defer resp.Body.Close()
		t.Logf("/update/ endpoint status: %d", resp.StatusCode)
		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("/update/ response: %s", string(respBody))
		}
	})

	// Debug: Manual test comparison
	t.Run("Debug: Manual test while server running", func(t *testing.T) {
		t.Logf("Server is running on port %d", port)
		t.Logf("You can manually test with:")
		t.Logf(`curl -X POST http://localhost:%d/updates/ -H "Content-Type: application/json" -d '[{"id":"manual_test","type":"gauge","value":99.0}]'`, port)

		// Let's also try a simple request using the exact same format as curl
		batch := `[{"id":"manual_test","type":"gauge","value":99.0}]`
		resp, err := http.Post("http://localhost:"+strconv.Itoa(port)+"/updates/", "application/json", strings.NewReader(batch))
		if err != nil {
			t.Errorf("Manual-style request failed: %v", err)
		} else {
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("Manual-style request: status %d, response: %s", resp.StatusCode, string(respBody))
		}

		// Give time to run manual test if needed
		// time.Sleep(10 * time.Second)
	})

	t.Run("Valid batch request", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"id": "cpu_usage", "type": "gauge", "value": 85.5},
			{"id": "requests_total", "type": "counter", "delta": 10},
			{"id": "memory_usage", "type": "gauge", "value": 67.2},
		}
		body, _ := json.Marshal(batch)
		t.Logf("Sending batch to /updates/: %s", string(body))
		resp, err := client.Post("http://localhost:"+strconv.Itoa(port)+"/updates/", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatalf("Valid batch request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 200, got %d. Response: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("Empty batch (should fail)", func(t *testing.T) {
		resp, err := client.Post("http://localhost:"+strconv.Itoa(port)+"/updates/", "application/json", strings.NewReader("[]"))
		if err != nil {
			t.Fatalf("Empty batch request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("Expected non-200 for empty batch, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid content type (should fail)", func(t *testing.T) {
		resp, err := client.Post("http://localhost:"+strconv.Itoa(port)+"/updates/", "text/plain", strings.NewReader(`[{"id":"test","type":"gauge","value":1.0}]`))
		if err != nil {
			t.Fatalf("Invalid content type request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("Expected non-200 for invalid content type, got %d", resp.StatusCode)
		}
	})

	t.Run("Verify metrics were stored", func(t *testing.T) {
		checkMetricValue(t, port, "gauge", "cpu_usage")
		checkMetricValue(t, port, "counter", "requests_total")
		checkMetricValue(t, port, "gauge", "memory_usage")
	})

	t.Run("Gzip compressed batch", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"id": "compressed_metric", "type": "gauge", "value": 99.9},
		}
		body, _ := json.Marshal(batch)
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write(body)
		_ = gz.Close()
		req, _ := http.NewRequest("POST", "http://localhost:"+strconv.Itoa(port)+"/updates/", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Gzip batch request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 200 for gzip batch, got %d. Response: %s", resp.StatusCode, string(respBody))
		}
	})
}

func checkMetricValue(t *testing.T, port int, metricType, metricName string) {
	url := "http://localhost:" + strconv.Itoa(port) + "/value/" + metricType + "/" + metricName
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("Failed to GET %s: %v", url, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Non-200 status for %s: %d", url, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(strings.TrimSpace(string(body))) == 0 {
		t.Errorf("Empty response for %s", url)
	}
}
