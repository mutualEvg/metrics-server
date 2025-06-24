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

func TestAgentRetryModes(t *testing.T) {
	basePort := 18080

	t.Run("Default (fast) retry mode", func(t *testing.T) {
		port := basePort + 1
		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-p", "1", "-r", "1")
		agent.Env = os.Environ()
		output, err := runWithTimeout(agent, 3*time.Second)
		if err == nil {
			t.Errorf("Agent should fail to connect in default mode, got no error. Output: %s", output)
		}
		if !strings.Contains(output, "Failed") && !strings.Contains(output, "retry") {
			t.Errorf("Expected retry/failure output, got: %s", output)
		}
	})

	t.Run("Disable retry mode", func(t *testing.T) {
		port := basePort + 2
		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-p", "1", "-r", "1")
		agent.Env = append(os.Environ(), "DISABLE_RETRY=true")
		output, err := runWithTimeout(agent, 3*time.Second)
		if err == nil {
			t.Errorf("Agent should fail to connect in no-retry mode, got no error. Output: %s", output)
		}
		if !strings.Contains(output, "Failed") {
			t.Errorf("Expected immediate failure output, got: %s", output)
		}
	})

	t.Run("Full retry mode", func(t *testing.T) {
		port := basePort + 3
		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-p", "1", "-r", "1")
		agent.Env = append(os.Environ(), "ENABLE_FULL_RETRY=true")
		output, err := runWithTimeout(agent, 5*time.Second)
		if err == nil {
			t.Errorf("Agent should fail to connect in full-retry mode, got no error. Output: %s", output)
		}
		if !strings.Contains(output, "Failed to send batch") && !strings.Contains(output, "Failed to send metric") {
			t.Errorf("Expected retry/failure output, got: %s", output)
		}
	})

	t.Run("Agent succeeds when server is running", func(t *testing.T) {
		port := basePort + 4
		server := exec.Command("../../server", "-a", "localhost:"+strconv.Itoa(port))
		server.Stdout = os.Stdout
		server.Stderr = os.Stderr
		if err := server.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = server.Process.Kill()
			_ = server.Wait()
		}()
		time.Sleep(2 * time.Second)

		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-p", "1", "-r", "1")
		agent.Env = os.Environ()
		output, err := runWithTimeout(agent, 3*time.Second)
		if err != nil && !strings.Contains(output, "Successfully") {
			t.Errorf("Agent should succeed when server is running. Output: %s, err: %v", output, err)
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
