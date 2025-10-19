package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/internal/handlers"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/storage"
)

// BenchmarkUpdateHandler benchmarks the legacy URL-based update handler
func BenchmarkUpdateHandler(b *testing.B) {
	s := storage.NewMemStorage()
	handler := handlers.UpdateHandler(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update/gauge/test_metric/123.45", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("type", "gauge")
		rctx.URLParams.Add("name", "test_metric")
		rctx.URLParams.Add("value", "123.45")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkValueHandler benchmarks the legacy URL-based value handler
func BenchmarkValueHandler(b *testing.B) {
	s := storage.NewMemStorage()
	s.UpdateGauge("test_metric", 123.45)
	handler := handlers.ValueHandler(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/value/gauge/test_metric", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("type", "gauge")
		rctx.URLParams.Add("name", "test_metric")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkUpdateJSONHandler benchmarks the JSON-based update handler
func BenchmarkUpdateJSONHandler(b *testing.B) {
	s := storage.NewMemStorage()
	handler := handlers.UpdateJSONHandler(s, nil)

	value := 123.45
	metric := models.Metrics{
		ID:    "test_gauge",
		MType: "gauge",
		Value: &value,
	}

	jsonData, _ := json.Marshal(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkValueJSONHandler benchmarks the JSON-based value handler
func BenchmarkValueJSONHandler(b *testing.B) {
	s := storage.NewMemStorage()
	s.UpdateGauge("test_gauge", 123.45)
	handler := handlers.ValueJSONHandler(s, nil)

	metric := models.Metrics{
		ID:    "test_gauge",
		MType: "gauge",
	}

	jsonData, _ := json.Marshal(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkUpdateBatchHandler benchmarks the batch update handler
func BenchmarkUpdateBatchHandler(b *testing.B) {
	s := storage.NewMemStorage()
	handler := handlers.UpdateBatchHandler(s, nil)

	// Create batch of 10 metrics
	metrics := make([]models.Metrics, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			value := float64(i)
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("gauge_%d", i),
				MType: "gauge",
				Value: &value,
			}
		} else {
			delta := int64(i)
			metrics[i] = models.Metrics{
				ID:    fmt.Sprintf("counter_%d", i),
				MType: "counter",
				Delta: &delta,
			}
		}
	}

	jsonData, _ := json.Marshal(metrics)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkRootHandler benchmarks the root handler that shows all metrics
func BenchmarkRootHandler(b *testing.B) {
	s := storage.NewMemStorage()
	// Pre-populate with data
	for i := 0; i < 50; i++ {
		s.UpdateGauge(fmt.Sprintf("gauge_%d", i), float64(i))
		s.UpdateCounter(fmt.Sprintf("counter_%d", i), int64(i))
	}

	handler := handlers.RootHandler(s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkHandlerWithManyMetrics benchmarks handlers performance with many metrics
func BenchmarkHandlerWithManyMetrics(b *testing.B) {
	s := storage.NewMemStorage()
	// Pre-populate with lots of data
	for i := 0; i < 1000; i++ {
		s.UpdateGauge(fmt.Sprintf("gauge_%d", i), float64(i))
		s.UpdateCounter(fmt.Sprintf("counter_%d", i), int64(i))
	}

	handler := handlers.ValueJSONHandler(s, nil)

	metric := models.Metrics{
		ID:    "gauge_500", // Metric that exists
		MType: "gauge",
	}

	jsonData, _ := json.Marshal(metric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler(w, req)
	}
}

// BenchmarkJSONParsing benchmarks JSON parsing performance
func BenchmarkJSONParsing(b *testing.B) {
	value := 123.45
	metric := models.Metrics{
		ID:    "test_gauge_with_long_name_for_testing",
		MType: "gauge",
		Value: &value,
	}

	jsonData, _ := json.Marshal(metric)
	jsonStr := string(jsonData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m models.Metrics
		json.Unmarshal([]byte(jsonStr), &m)
	}
}
