package worker

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mutualEvg/metrics-server/internal/crypto"
	"github.com/mutualEvg/metrics-server/internal/hash"
	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/retry"
)

// MetricData represents a single metric to be sent
type MetricData struct {
	Metric models.Metrics
	Type   string // "runtime", "system", or other types
}

// Pool manages concurrent metric sending
type Pool struct {
	jobs        chan MetricData
	wg          sync.WaitGroup
	rateLimit   int
	httpClient  *http.Client
	serverAddr  string
	key         string         // Key for SHA256 signature
	publicKey   *rsa.PublicKey // Public key for encryption
	retryConfig retry.RetryConfig
}

// NewPool creates a new worker pool
func NewPool(rateLimit int, serverAddr, key string, retryConfig retry.RetryConfig) *Pool {
	return &Pool{
		jobs:        make(chan MetricData, rateLimit*10), // Buffer to handle burst metrics
		rateLimit:   rateLimit,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		serverAddr:  serverAddr,
		key:         key,
		publicKey:   nil,
		retryConfig: retryConfig,
	}
}

// SetPublicKey sets the public key for encryption
func (p *Pool) SetPublicKey(publicKey *rsa.PublicKey) {
	p.publicKey = publicKey
	if publicKey != nil {
		log.Printf("Public key configured for encryption")
	}
}

// Start initializes the worker pool
func (p *Pool) Start() {
	for i := 0; i < p.rateLimit; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Started worker pool with %d workers", p.rateLimit)
}

// Stop gracefully shuts down the worker pool
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	log.Printf("Worker pool stopped")
}

// SubmitMetric adds a metric to the sending queue
func (p *Pool) SubmitMetric(metric MetricData) {
	// Recover from panic if channel is closed
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker pool channel closed, dropping metric: %s", metric.Metric.ID)
		}
	}()

	select {
	case p.jobs <- metric:
		// Metric submitted successfully
	default:
		log.Printf("Worker pool queue full, dropping metric: %s", metric.Metric.ID)
	}
}

// worker processes metrics from the queue
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	log.Printf("Worker %d started", id)

	for metric := range p.jobs {
		p.sendMetric(metric)
	}

	log.Printf("Worker %d stopped", id)
}

// getOutboundIP gets the preferred outbound IP address of this machine
func getOutboundIP() string {
	// Try to get the outbound IP by connecting to a public DNS server
	// This doesn't actually send any data, just establishes which interface would be used
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1" // Fallback to localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// sendMetric sends a single metric to the server
func (p *Pool) sendMetric(metricData MetricData) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := retry.Do(ctx, p.retryConfig, func() error {
		jsonData, err := json.Marshal(metricData.Metric)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Compress the JSON data
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		_, err = gzipWriter.Write(jsonData)
		if err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		err = gzipWriter.Close()
		if err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}

		// Prepare body data (may be encrypted)
		bodyData := compressedData.Bytes()

		// Encrypt if public key is configured
		if p.publicKey != nil {
			encryptedData, err := crypto.EncryptRSAChunked(bodyData, p.publicKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt data: %w", err)
			}
			bodyData = encryptedData
		}

		url := fmt.Sprintf("%s/update/", p.serverAddr)
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		// Add X-Real-IP header with the agent's IP address
		req.Header.Set("X-Real-IP", getOutboundIP())

		// Add encryption header if data is encrypted
		if p.publicKey != nil {
			req.Header.Set("X-Encrypted", "true")
		}

		// Add hash header if key is configured (hash is computed before encryption)
		if p.key != "" {
			hashValue := hash.CalculateHash(compressedData.Bytes(), p.key)
			req.Header.Set("HashSHA256", hashValue)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send metric: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned non-OK status: %s", resp.Status)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to send %s metric %s after retries: %v", metricData.Type, metricData.Metric.ID, err)
	}
}
