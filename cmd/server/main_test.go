package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

func TestUpdateHandler(t *testing.T) {
	storage := storage.NewMemStorage()
	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", updateHandler(storage))

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
	router.Post("/update/", updateJSONHandler(storage))

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
	router.Post("/value/", valueJSONHandler(storage))

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
