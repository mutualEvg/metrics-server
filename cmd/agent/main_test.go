package main

import (
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
