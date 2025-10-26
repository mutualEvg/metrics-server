package pool

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// This file demonstrates practical usage scenarios for the Pool.

// RequestContext is a heavy object that can be pooled and reused across HTTP requests
type RequestContext struct {
	RequestID string
	Headers   map[string]string
	Body      []byte
	Metadata  map[string]interface{}
}

func (rc *RequestContext) Reset() {
	rc.RequestID = ""
	clear(rc.Headers)
	rc.Body = rc.Body[:0]
	clear(rc.Metadata)
}

// Example: HTTP Handler with Pooled Request Context
//
// This demonstrates how to use a pool in an HTTP handler to reduce allocations.
func ExampleHTTPHandler() {
	// Create a pool for request contexts
	contextPool := New(func() *RequestContext {
		return &RequestContext{
			Headers:  make(map[string]string, 10),
			Body:     make([]byte, 0, 4096),
			Metadata: make(map[string]interface{}, 10),
		}
	})

	// HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get a request context from the pool
		ctx := contextPool.Get()
		defer contextPool.Put(ctx) // Ensure it's returned to the pool

		// Use the context
		ctx.RequestID = r.Header.Get("X-Request-ID")
		for key, values := range r.Header {
			if len(values) > 0 {
				ctx.Headers[key] = values[0]
			}
		}

		// Process request...
		fmt.Fprintf(w, "Request %s processed", ctx.RequestID)
	})

	// Use the handler
	_ = handler
}

// BatchBuffer is a buffer for batch operations
type BatchBuffer struct {
	Items   []string
	Results map[string]bool
	Errors  []error
}

func (bb *BatchBuffer) Reset() {
	bb.Items = bb.Items[:0]
	clear(bb.Results)
	bb.Errors = bb.Errors[:0]
}

// Example: Batch Processing with Pooled Buffers
//
// This demonstrates using a pool for batch processing operations.
func ExampleBatchProcessing() {
	// Create pool
	bufferPool := New(func() *BatchBuffer {
		return &BatchBuffer{
			Items:   make([]string, 0, 100),
			Results: make(map[string]bool, 100),
			Errors:  make([]error, 0, 10),
		}
	})

	// Process batches
	processBatch := func(items []string) error {
		buf := bufferPool.Get()
		defer bufferPool.Put(buf)

		buf.Items = append(buf.Items, items...)
		// Process items...
		return nil
	}

	_ = processBatch
}

// JSONBuffer is a buffer for JSON encoding
type JSONBuffer struct {
	Buffer []byte
}

func (jb *JSONBuffer) Reset() {
	jb.Buffer = jb.Buffer[:0]
}

// Example: JSON Encoding with Pooled Buffers
//
// This demonstrates using a pool for JSON encoding to reduce allocations.
func ExampleJSONEncoding() {
	// Create pool
	jsonPool := New(func() *JSONBuffer {
		return &JSONBuffer{
			Buffer: make([]byte, 0, 8192),
		}
	})

	// Encode function
	encodeJSON := func(data interface{}) ([]byte, error) {
		buf := jsonPool.Get()
		defer jsonPool.Put(buf)

		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		buf.Buffer = append(buf.Buffer, encoded...)
		
		// Return a copy since we're returning the buffer to the pool
		result := make([]byte, len(buf.Buffer))
		copy(result, buf.Buffer)
		return result, nil
	}

	_ = encodeJSON
}

// WorkItem represents a work item in a worker pool
type WorkItem struct {
	ID      int
	Payload []byte
	Result  interface{}
}

func (wi *WorkItem) Reset() {
	wi.ID = 0
	wi.Payload = wi.Payload[:0]
	wi.Result = nil
}

// Example: Worker Pool Pattern
//
// This demonstrates using a pool with a worker pool pattern.
func ExampleWorkerPool() {

	// Create pool
	workItemPool := New(func() *WorkItem {
		return &WorkItem{
			Payload: make([]byte, 0, 1024),
		}
	})

	// Worker function
	worker := func(jobs <-chan []byte, results chan<- interface{}) {
		for payload := range jobs {
			item := workItemPool.Get()
			
			item.Payload = append(item.Payload, payload...)
			// Process item...
			item.Result = "processed"
			
			results <- item.Result
			workItemPool.Put(item)
		}
	}

	_ = worker
}

// CacheEntry represents a cache entry
type CacheEntry struct {
	Key       string
	Value     []byte
	Headers   map[string]string
	Timestamp int64
}

func (ce *CacheEntry) Reset() {
	ce.Key = ""
	ce.Value = ce.Value[:0]
	clear(ce.Headers)
	ce.Timestamp = 0
}

// Example: Response Caching with Pooled Cache Entries
//
// This demonstrates using a pool for cache entries.
func ExampleCacheWithPool() {

	// Create pool
	cachePool := New(func() *CacheEntry {
		return &CacheEntry{
			Value:   make([]byte, 0, 4096),
			Headers: make(map[string]string, 10),
		}
	})

	// Cache storage
	cache := make(map[string]*CacheEntry)

	// Set cache entry
	setCache := func(key string, value []byte, headers map[string]string) {
		entry := cachePool.Get()
		entry.Key = key
		entry.Value = append(entry.Value, value...)
		for k, v := range headers {
			entry.Headers[k] = v
		}
		cache[key] = entry
	}

	// Remove cache entry
	deleteCache := func(key string) {
		if entry, ok := cache[key]; ok {
			delete(cache, key)
			cachePool.Put(entry) // Return to pool
		}
	}

	_ = setCache
	_ = deleteCache
}

