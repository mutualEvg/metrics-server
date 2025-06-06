package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/storage"
	"net/http"
	"net/http/httptest"
	"testing"
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
