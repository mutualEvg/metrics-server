package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateHandler(t *testing.T) {
	storage := NewMemStorage()
	handler := updateHandler(storage)

	tests := []struct {
		name       string
		method     string
		url        string
		wantStatus int
	}{
		{
			name:       "Valid Gauge Update",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/123.45",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Valid Counter Update",
			method:     http.MethodPost,
			url:        "/update/counter/testCounter/42",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid URL Format",
			method:     http.MethodPost,
			url:        "/update/gauge/onlyname",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Invalid Metric Type",
			method:     http.MethodPost,
			url:        "/update/unknown/test/10",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid Value",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/notANumber",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Wrong Method",
			method:     http.MethodGet,
			url:        "/update/gauge/testGauge/123",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}
