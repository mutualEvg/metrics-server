// Package audit implements the Observer pattern for audit logging.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Event represents an audit event for metrics collection.
type Event struct {
	// Timestamp is the Unix timestamp of the event
	Timestamp int64 `json:"ts"`

	// Metrics contains the names of the received metrics
	Metrics []string `json:"metrics"`

	// IPAddress is the IP address of the incoming request
	IPAddress string `json:"ip_address"`
}

// Observer defines the interface for audit observers.
// Observers are notified when an audit event occurs.
type Observer interface {
	// Notify sends an audit event to the observer
	Notify(event Event) error
}

// Subject manages a collection of observers and notifies them of events.
type Subject struct {
	observers []Observer
	mu        sync.RWMutex
}

// NewSubject creates a new audit subject.
func NewSubject() *Subject {
	return &Subject{
		observers: make([]Observer, 0),
	}
}

// Attach adds an observer to the subject.
func (s *Subject) Attach(observer Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, observer)
}

// Notify sends an event to all attached observers.
// Errors from individual observers are logged but don't stop notification of other observers.
func (s *Subject) Notify(event Event) {
	s.mu.RLock()
	observers := make([]Observer, len(s.observers))
	copy(observers, s.observers)
	s.mu.RUnlock()

	for _, observer := range observers {
		if err := observer.Notify(event); err != nil {
			log.Error().Err(err).Msg("Failed to notify audit observer")
		}
	}
}

// HasObservers returns true if there are any observers attached.
func (s *Subject) HasObservers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observers) > 0
}

// FileAuditor writes audit events to a file.
type FileAuditor struct {
	filePath string
	mu       sync.Mutex
}

// NewFileAuditor creates a new file-based audit observer.
func NewFileAuditor(filePath string) (*FileAuditor, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Test if we can write to the file
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	file.Close()

	return &FileAuditor{
		filePath: filePath,
	}, nil
}

// Notify writes the audit event to the file as a JSON line.
func (f *FileAuditor) Notify(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(f.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer file.Close()

	// Write JSON line
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to audit file: %w", err)
	}

	log.Debug().
		Str("file", f.filePath).
		Int("metrics_count", len(event.Metrics)).
		Msg("Audit event written to file")

	return nil
}

// RemoteAuditor sends audit events to a remote server via HTTP POST.
type RemoteAuditor struct {
	url        string
	httpClient *http.Client
}

// NewRemoteAuditor creates a new remote server audit observer.
func NewRemoteAuditor(url string) (*RemoteAuditor, error) {
	if url == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	return &RemoteAuditor{
		url: url,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// Notify sends the audit event to the remote server via HTTP POST.
func (r *RemoteAuditor) Notify(event Event) error {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", r.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create audit request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote audit server returned status %d", resp.StatusCode)
	}

	log.Debug().
		Str("url", r.url).
		Int("status", resp.StatusCode).
		Int("metrics_count", len(event.Metrics)).
		Msg("Audit event sent to remote server")

	return nil
}
