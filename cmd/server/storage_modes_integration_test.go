//go:build integration

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStorageModes(t *testing.T) {
	basePort := 18100

	t.Run("Memory storage (no flags)", func(t *testing.T) {
		port := basePort + 1
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		output, err := runWithTimeout(cmd, 3*time.Second)
		if err != nil && !strings.Contains(output, "Using in-memory storage") {
			t.Errorf("Expected in-memory storage, got: %s", output)
		}
	})

	t.Run("File storage (with -f flag)", func(t *testing.T) {
		port := basePort + 2
		file := "/tmp/test-metrics.json"
		_ = os.Remove(file)
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port), "-f", file)
		output, err := runWithTimeout(cmd, 3*time.Second)
		if err != nil && !strings.Contains(output, "Using file storage") {
			t.Errorf("Expected file storage, got: %s", output)
		}
	})

	t.Run("File storage (with FILE_STORAGE_PATH env)", func(t *testing.T) {
		port := basePort + 3
		file := "/tmp/test-metrics-env.json"
		_ = os.Remove(file)
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		cmd.Env = append(os.Environ(), "FILE_STORAGE_PATH="+file)
		output, err := runWithTimeout(cmd, 3*time.Second)
		if err != nil && !strings.Contains(output, "Using file storage") {
			t.Errorf("Expected file storage via env, got: %s", output)
		}
	})

	t.Run("Database storage (with invalid DSN)", func(t *testing.T) {
		port := basePort + 4
		cmd := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port), "-d", "postgres://invalid:invalid@localhost/invalid?sslmode=disable")
		output, _ := runWithTimeout(cmd, 3*time.Second)
		if !strings.Contains(output, "failed to connect") && !strings.Contains(output, "database") {
			t.Errorf("Expected database connection error, got: %s", output)
		}
	})
}

// runWithTimeout runs a command with a timeout and returns combined output.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Run()
	}()
	select {
	case err := <-errCh:
		return output.String(), err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return output.String(), os.ErrDeadlineExceeded
	}
}
