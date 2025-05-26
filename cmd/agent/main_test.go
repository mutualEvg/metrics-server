package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestPollMetrics(t *testing.T) {
	var gauges sync.Map
	pollMetrics(&gauges)

	foundRandom := false
	gauges.Range(func(key, value any) bool {
		if key == "RandomValue" {
			foundRandom = true
		}
		return true
	})

	if !foundRandom {
		t.Error("RandomValue should be present in gauges")
	}

	if pollCount == 0 {
		t.Error("pollCount should be incremented after polling metrics")
	}
}

func TestSendMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}
	}))
	defer server.Close()

	client := server.Client()

	// Temporarily change the global serverAddress for the test
	oldAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = oldAddress }()

	sendMetric(client, "gauge", "TestMetric", "123.45")
}

func TestSendMetricJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("Expected Content-Encoding gzip, got %s", r.Header.Get("Content-Encoding"))
		}

		if r.Header.Get("Accept-Encoding") != "gzip" {
			t.Errorf("Expected Accept-Encoding gzip, got %s", r.Header.Get("Accept-Encoding"))
		}

		// Verify the body is compressed
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("Failed to create gzip reader: %v", err)
			return
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			t.Errorf("Failed to read compressed body: %v", err)
			return
		}

		// Verify it's valid JSON
		expectedJSON := `{"id":"TestMetric","type":"gauge","value":123.45}`
		if string(body) != expectedJSON {
			t.Errorf("Expected JSON %s, got %s", expectedJSON, string(body))
		}
	}))
	defer server.Close()

	client := server.Client()

	// Temporarily change the global serverAddress for the test
	oldAddress := serverAddress
	serverAddress = server.URL
	defer func() { serverAddress = oldAddress }()

	sendMetricJSON(client, "gauge", "TestMetric", 123.45, 0)
}
