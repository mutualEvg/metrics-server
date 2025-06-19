package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/handlers"
	gzipmw "github.com/mutualEvg/metrics-server/internal/middleware"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

func TestUpdateHandler(t *testing.T) {
	storage := storage.NewMemStorage()
	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", handlers.UpdateHandler(storage))

	tests := []struct {
		name       string
		method     string
		url        string
		wantStatus int
	}{
		{
			name:       "Valid_Gauge_Update",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/123.45",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Valid_Counter_Update",
			method:     http.MethodPost,
			url:        "/update/counter/testCounter/42",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid_URL_Format",
			method:     http.MethodPost,
			url:        "/update/gauge/onlyname",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Invalid_Metric_Type",
			method:     http.MethodPost,
			url:        "/update/unknown/test/10",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid_Value",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/notANumber",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Wrong_Method",
			method:     http.MethodGet,
			url:        "/update/gauge/testGauge/123.45",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, expected %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestUpdateJSONHandler(t *testing.T) {
	storage := storage.NewMemStorage()
	router := chi.NewRouter()
	router.Post("/update/", handlers.UpdateJSONHandler(storage))

	tests := []struct {
		name       string
		metric     models.Metrics
		wantStatus int
	}{
		{
			name: "Valid_Gauge_Update",
			metric: models.Metrics{
				ID:    "testGauge",
				MType: "gauge",
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Valid_Counter_Update",
			metric: models.Metrics{
				ID:    "testCounter",
				MType: "counter",
				Delta: func() *int64 { v := int64(42); return &v }(),
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Missing_ID",
			metric: models.Metrics{
				MType: "gauge",
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "Missing_Value_For_Gauge",
			metric: models.Metrics{
				ID:    "testGauge",
				MType: "gauge",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid_Metric_Type",
			metric: models.Metrics{
				ID:    "test",
				MType: "unknown",
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.metric)
			req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, expected %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestValueJSONHandler(t *testing.T) {
	storage := storage.NewMemStorage()
	// Pre-populate storage
	storage.UpdateGauge("testGauge", 123.45)
	storage.UpdateCounter("testCounter", 42)

	router := chi.NewRouter()
	router.Post("/value/", handlers.ValueJSONHandler(storage))

	tests := []struct {
		name       string
		metric     models.Metrics
		wantStatus int
	}{
		{
			name: "Get_Existing_Gauge",
			metric: models.Metrics{
				ID:    "testGauge",
				MType: "gauge",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Get_Existing_Counter",
			metric: models.Metrics{
				ID:    "testCounter",
				MType: "counter",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Get_Nonexistent_Metric",
			metric: models.Metrics{
				ID:    "nonexistent",
				MType: "gauge",
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.metric)
			req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, expected %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestGzipCompression(t *testing.T) {
	storage := storage.NewMemStorage()
	router := chi.NewRouter()
	router.Use(gzipmw.GzipMiddleware)
	router.Post("/update/", handlers.UpdateJSONHandler(storage))

	metric := models.Metrics{
		ID:    "testGauge",
		MType: "gauge",
		Value: func() *float64 { v := 123.45; return &v }(),
	}

	jsonData, _ := json.Marshal(metric)
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Check that response is compressed
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected Content-Encoding: gzip header")
	}

	// Decompress and verify response
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	var response models.Metrics
	if err := json.Unmarshal(decompressed, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != "testGauge" || response.MType != "gauge" {
		t.Errorf("Unexpected response: %+v", response)
	}
}

func TestGzipDecompression(t *testing.T) {
	storage := storage.NewMemStorage()
	router := chi.NewRouter()
	router.Use(gzipmw.GzipMiddleware)
	router.Post("/update/", handlers.UpdateJSONHandler(storage))

	metric := models.Metrics{
		ID:    "testGauge",
		MType: "gauge",
		Value: func() *float64 { v := 123.45; return &v }(),
	}

	jsonData, _ := json.Marshal(metric)

	// Compress the request data
	var compressedData bytes.Buffer
	gz := gzip.NewWriter(&compressedData)
	gz.Write(jsonData)
	gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/update/", &compressedData)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify the metric was stored
	if value, ok := storage.GetGauge("testGauge"); !ok || value != 123.45 {
		t.Errorf("Expected gauge value 123.45, got %f", value)
	}
}
