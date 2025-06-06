// cmd/server/main.go
package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/mutualEvg/metrics-server/config"
	"github.com/mutualEvg/metrics-server/storage"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	GaugeType   = "gauge"
	CounterType = "counter"
)

func main() {
	cfg := config.Load()
	memStorage := storage.NewMemStorage()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", updateHandler(memStorage))
	r.Get("/value/{type}/{name}", valueHandler(memStorage))
	r.Get("/", rootHandler(memStorage))

	fmt.Printf("Server running at %s\n", cfg.ServerAddress)
	// this fixes => Server failed: listen tcp: address http://localhost:8080: too many colons in address
	if err := http.ListenAndServe(strings.TrimPrefix(strings.TrimPrefix(cfg.ServerAddress, "http://"), "https://"), r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func updateHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")
		value := chi.URLParam(r, "value")

		switch typ {
		case GaugeType:
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				http.Error(w, "invalid gauge value", http.StatusBadRequest)
				return
			}
			s.UpdateGauge(name, v)
		case CounterType:
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				http.Error(w, "invalid counter value", http.StatusBadRequest)
				return
			}
			s.UpdateCounter(name, v)
		default:
			http.Error(w, "unknown metric type", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func valueHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")

		switch typ {
		case GaugeType:
			if v, ok := s.GetGauge(name); ok {
				w.Write([]byte(strconv.FormatFloat(v, 'f', -1, 64)))
				return
			}
		case CounterType:
			if v, ok := s.GetCounter(name); ok {
				w.Write([]byte(strconv.FormatInt(v, 10)))
				return
			}
		}

		http.Error(w, "metric not found", http.StatusNotFound)
	}
}

func rootHandler(s storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, c := s.GetAll()
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Metrics</h1><ul>"))
		for k, v := range g {
			fmt.Fprintf(w, "<li>%s (gauge): %f</li>", k, v)
		}
		for k, v := range c {
			fmt.Fprintf(w, "<li>%s (counter): %d</li>", k, v)
		}
		w.Write([]byte("</ul></body></html>"))
	}
}
