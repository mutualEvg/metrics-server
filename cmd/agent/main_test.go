package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPollMetrics(t *testing.T) {
	gauges := make(map[string]float64)
	pollMetrics(gauges)

	if len(gauges) == 0 {
		t.Fatal("gauges map should not be empty after polling metrics")
	}

	if _, ok := gauges["RandomValue"]; !ok {
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
