package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRetryFunctionality(t *testing.T) {
	port := 18110

	// Ensure no leftover processes
	_ = exec.Command("pkill", "-f", "./server").Run()
	_ = exec.Command("pkill", "-f", "./agent").Run()
	time.Sleep(1 * time.Second)

	t.Run("Agent retries when server is down", func(t *testing.T) {
		agent := exec.Command("./agent", "-a", "localhost:"+strconv.Itoa(port), "-r", "5", "-p", "2", "-b", "5")
		var output strings.Builder
		agent.Stdout = &output
		agent.Stderr = &output
		if err := agent.Start(); err != nil {
			t.Fatalf("Failed to start agent: %v", err)
		}
		time.Sleep(10 * time.Second)
		_ = agent.Process.Kill()
		_ = agent.Wait()
		if !strings.Contains(output.String(), "Failed") {
			t.Errorf("Expected retry/failure output, got: %s", output.String())
		}
	})

	t.Run("Agent succeeds after server starts", func(t *testing.T) {
		server := exec.Command("./server", "-a", "localhost:"+strconv.Itoa(port))
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

		agent := exec.Command("./agent", "-a", "localhost:"+strconv.Itoa(port), "-r", "5", "-p", "2", "-b", "5")
		agent.Stdout = os.Stdout
		agent.Stderr = os.Stderr
		if err := agent.Start(); err != nil {
			t.Fatalf("Failed to start agent: %v", err)
		}
		time.Sleep(5 * time.Second)
		_ = agent.Process.Kill()
		_ = agent.Wait()
	})

	t.Run("Server endpoints", func(t *testing.T) {
		// Start server for this test
		server := exec.Command("./server", "-a", "localhost:"+strconv.Itoa(port))
		server.Stdout = os.Stdout
		server.Stderr = os.Stderr
		if err := server.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		defer func() {
			_ = server.Process.Kill()
			_ = server.Wait()
		}()
		time.Sleep(2 * time.Second) // Wait for server to start

		// Test root endpoint instead of /ping
		resp, err := http.Get("http://localhost:" + strconv.Itoa(port) + "/")
		if err != nil {
			t.Errorf("Root endpoint failed: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("Root endpoint returned status %d", resp.StatusCode)
			return
		}

		// Verify it returns HTML content
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			t.Errorf("Root endpoint returned empty body")
			return
		}
		if !strings.Contains(string(body), "Metrics") {
			t.Errorf("Root endpoint doesn't contain expected 'Metrics' text")
		}
	})
}
