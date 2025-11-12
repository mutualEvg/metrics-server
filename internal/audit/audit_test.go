package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewSubject(t *testing.T) {
	subject := NewSubject()
	if subject == nil {
		t.Error("Expected non-nil subject")
	}

	if subject.HasObservers() {
		t.Error("New subject should have no observers")
	}
}

func TestSubjectAttach(t *testing.T) {
	subject := NewSubject()

	// Create a mock observer
	tempFile := "/tmp/test_audit_attach.json"
	defer os.Remove(tempFile)

	observer, err := NewFileAuditor(tempFile)
	if err != nil {
		t.Fatalf("Failed to create file auditor: %v", err)
	}

	subject.Attach(observer)

	if !subject.HasObservers() {
		t.Error("Subject should have observers after Attach")
	}
}

func TestFileAuditor(t *testing.T) {
	tempFile := "/tmp/test_audit_file.json"
	defer os.Remove(tempFile)

	auditor, err := NewFileAuditor(tempFile)
	if err != nil {
		t.Fatalf("Failed to create file auditor: %v", err)
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"cpu_usage", "memory_usage"},
		IPAddress: "192.168.1.100",
	}

	// Write event
	err = auditor.Notify(event)
	if err != nil {
		t.Fatalf("Failed to notify file auditor: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read audit file: %v", err)
	}

	var readEvent Event
	err = json.Unmarshal(data[:len(data)-1], &readEvent) // Remove trailing newline
	if err != nil {
		t.Fatalf("Failed to unmarshal audit event: %v", err)
	}

	if readEvent.Timestamp != event.Timestamp {
		t.Errorf("Expected timestamp %d, got %d", event.Timestamp, readEvent.Timestamp)
	}

	if len(readEvent.Metrics) != len(event.Metrics) {
		t.Errorf("Expected %d metrics, got %d", len(event.Metrics), len(readEvent.Metrics))
	}

	if readEvent.IPAddress != event.IPAddress {
		t.Errorf("Expected IP %s, got %s", event.IPAddress, readEvent.IPAddress)
	}
}

func TestFileAuditorMultipleEvents(t *testing.T) {
	tempFile := "/tmp/test_audit_multiple.json"
	defer os.Remove(tempFile)

	auditor, err := NewFileAuditor(tempFile)
	if err != nil {
		t.Fatalf("Failed to create file auditor: %v", err)
	}

	// Write multiple events with incrementing timestamps
	for i := 0; i < 3; i++ {
		event := Event{
			Timestamp: time.Now().Unix() + int64(i), // Increment timestamp to ensure uniqueness
			Metrics:   []string{"metric1", "metric2"},
			IPAddress: "192.168.1.100",
		}
		err = auditor.Notify(event)
		if err != nil {
			t.Fatalf("Failed to notify file auditor: %v", err)
		}
	}

	// File auditor writes synchronously, so we can verify immediately
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read audit file: %v", err)
	}

	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}

	if lines != 3 {
		t.Errorf("Expected 3 lines in audit file, got %d", lines)
	}
}

func TestRemoteAuditor(t *testing.T) {
	// Create a test server to receive audit events
	received := make(chan Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("Failed to decode event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- event
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	auditor, err := NewRemoteAuditor(server.URL)
	if err != nil {
		t.Fatalf("Failed to create remote auditor: %v", err)
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"cpu_usage", "memory_usage"},
		IPAddress: "192.168.1.100",
	}

	// Send event
	err = auditor.Notify(event)
	if err != nil {
		t.Fatalf("Failed to notify remote auditor: %v", err)
	}

	// Verify event was received
	select {
	case receivedEvent := <-received:
		if receivedEvent.Timestamp != event.Timestamp {
			t.Errorf("Expected timestamp %d, got %d", event.Timestamp, receivedEvent.Timestamp)
		}
		if len(receivedEvent.Metrics) != len(event.Metrics) {
			t.Errorf("Expected %d metrics, got %d", len(event.Metrics), len(receivedEvent.Metrics))
		}
		if receivedEvent.IPAddress != event.IPAddress {
			t.Errorf("Expected IP %s, got %s", event.IPAddress, receivedEvent.IPAddress)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}
}

func TestSubjectNotify(t *testing.T) {
	tempFile := "/tmp/test_audit_subject.json"
	defer os.Remove(tempFile)

	subject := NewSubject()
	fileAuditor, _ := NewFileAuditor(tempFile)
	subject.Attach(fileAuditor)

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"test_metric"},
		IPAddress: "127.0.0.1",
	}

	// Notify all observers
	subject.Notify(event)

	// File auditor writes synchronously, so verify immediately
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read audit file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Audit file should not be empty")
	}
}

func TestMultipleObservers(t *testing.T) {
	tempFile1 := "/tmp/test_audit_multi1.json"
	tempFile2 := "/tmp/test_audit_multi2.json"
	defer os.Remove(tempFile1)
	defer os.Remove(tempFile2)

	subject := NewSubject()

	// Attach two file auditors
	auditor1, _ := NewFileAuditor(tempFile1)
	auditor2, _ := NewFileAuditor(tempFile2)
	subject.Attach(auditor1)
	subject.Attach(auditor2)

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"test_metric"},
		IPAddress: "127.0.0.1",
	}

	// Notify all observers
	subject.Notify(event)

	// File auditor writes synchronously, so verify immediately
	data1, err1 := os.ReadFile(tempFile1)
	data2, err2 := os.ReadFile(tempFile2)

	if err1 != nil || err2 != nil {
		t.Fatal("Failed to read audit files")
	}

	if len(data1) == 0 || len(data2) == 0 {
		t.Error("Both audit files should have data")
	}
}

func TestNewFileAuditorError(t *testing.T) {
	// Try to create auditor with invalid path
	_, err := NewFileAuditor("")
	if err == nil {
		t.Error("Expected error for empty file path")
	}

	// Try to create auditor with non-writable path
	_, err = NewFileAuditor("/root/impossible_path.json")
	if err == nil {
		t.Error("Expected error for non-writable path")
	}
}

func TestNewRemoteAuditorError(t *testing.T) {
	// Try to create auditor with empty URL
	_, err := NewRemoteAuditor("")
	if err == nil {
		t.Error("Expected error for empty URL")
	}
}
