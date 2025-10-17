package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

func TestUpdateHandler(t *testing.T) {
	store := storage.NewMemStorage()
	handler := UpdateHandler(store)

	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid gauge update",
			method:         "POST",
			url:            "/update/gauge/cpu_usage/75.5",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "valid counter update",
			method:         "POST",
			url:            "/update/counter/requests/100",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "invalid gauge value",
			method:         "POST",
			url:            "/update/gauge/cpu_usage/invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid gauge value",
		},
		{
			name:           "invalid counter value",
			method:         "POST",
			url:            "/update/counter/requests/invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid counter value",
		},
		{
			name:           "unknown metric type",
			method:         "POST",
			url:            "/update/unknown/metric/100",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "unknown metric type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router to simulate chi URL params
			router := chi.NewRouter()
			router.Post("/update/{type}/{name}/{value}", handler)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestValueHandler(t *testing.T) {
	store := storage.NewMemStorage()
	store.UpdateGauge("cpu_usage", 75.5)
	store.UpdateCounter("requests", 100)

	handler := ValueHandler(store)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "get gauge value",
			url:            "/value/gauge/cpu_usage",
			expectedStatus: http.StatusOK,
			expectedBody:   "75.5",
		},
		{
			name:           "get counter value",
			url:            "/value/counter/requests",
			expectedStatus: http.StatusOK,
			expectedBody:   "100",
		},
		{
			name:           "gauge not found",
			url:            "/value/gauge/nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "metric not found",
		},
		{
			name:           "counter not found",
			url:            "/value/counter/nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "metric not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Get("/value/{type}/{name}", handler)

			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestRootHandler(t *testing.T) {
	store := storage.NewMemStorage()
	store.UpdateGauge("cpu", 45.5)
	store.UpdateCounter("requests", 123)

	handler := RootHandler(store)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Header().Get("Content-Type") != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if !strings.Contains(body, "cpu") || !strings.Contains(body, "requests") {
		t.Errorf("Expected body to contain metrics, got %s", body)
	}
}

func TestUpdateJSONHandler(t *testing.T) {
	store := storage.NewMemStorage()
	handler := UpdateJSONHandler(store, nil)

	tests := []struct {
		name           string
		metric         models.Metrics
		expectedStatus int
		expectError    bool
	}{
		{
			name: "valid gauge update",
			metric: models.Metrics{
				ID:    "cpu_usage",
				MType: "gauge",
				Value: func() *float64 { v := 75.5; return &v }(),
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "valid counter update",
			metric: models.Metrics{
				ID:    "requests",
				MType: "counter",
				Delta: func() *int64 { v := int64(100); return &v }(),
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "missing ID",
			metric: models.Metrics{
				MType: "gauge",
				Value: func() *float64 { v := 75.5; return &v }(),
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "missing Type",
			metric: models.Metrics{
				ID:    "cpu_usage",
				Value: func() *float64 { v := 75.5; return &v }(),
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "gauge missing value",
			metric: models.Metrics{
				ID:    "cpu_usage",
				MType: "gauge",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "counter missing delta",
			metric: models.Metrics{
				ID:    "requests",
				MType: "counter",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "unknown metric type",
			metric: models.Metrics{
				ID:    "test",
				MType: "unknown",
				Value: func() *float64 { v := 75.5; return &v }(),
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.metric)
			req := httptest.NewRequest("POST", "/update/", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.expectError && w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json for successful requests")
			}
		})
	}
}

func TestValueJSONHandler(t *testing.T) {
	store := storage.NewMemStorage()
	store.UpdateGauge("cpu_usage", 75.5)
	store.UpdateCounter("requests", 100)
	
	handler := ValueJSONHandler(store, nil)

	tests := []struct {
		name           string
		metric         models.Metrics
		expectedStatus int
		expectError    bool
	}{
		{
			name: "get gauge value",
			metric: models.Metrics{
				ID:    "cpu_usage",
				MType: "gauge",
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "get counter value",
			metric: models.Metrics{
				ID:    "requests",
				MType: "counter",
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "gauge not found",
			metric: models.Metrics{
				ID:    "nonexistent",
				MType: "gauge",
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name: "counter not found",
			metric: models.Metrics{
				ID:    "nonexistent",
				MType: "counter",
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name: "missing ID",
			metric: models.Metrics{
				MType: "gauge",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "unknown metric type",
			metric: models.Metrics{
				ID:    "test",
				MType: "unknown",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.metric)
			req := httptest.NewRequest("POST", "/value/", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.expectError && w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json for successful requests")
			}
		})
	}
}

func TestUpdateBatchHandler(t *testing.T) {
	store := storage.NewMemStorage()
	handler := UpdateBatchHandler(store, nil)

	tests := []struct {
		name           string
		metrics        []models.Metrics
		expectedStatus int
		expectError    bool
	}{
		{
			name: "valid batch update",
			metrics: []models.Metrics{
				{
					ID:    "cpu_usage",
					MType: "gauge",
					Value: func() *float64 { v := 75.5; return &v }(),
				},
				{
					ID:    "requests",
					MType: "counter",
					Delta: func() *int64 { v := int64(100); return &v }(),
				},
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "empty batch",
			metrics:        []models.Metrics{},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "invalid metric in batch",
			metrics: []models.Metrics{
				{
					ID:    "cpu_usage",
					MType: "gauge",
					Value: func() *float64 { v := 75.5; return &v }(),
				},
				{
					// Missing ID
					MType: "counter",
					Delta: func() *int64 { v := int64(100); return &v }(),
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.metrics)
			req := httptest.NewRequest("POST", "/updates/", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.expectError && w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json for successful requests")
			}
		})
	}
}
