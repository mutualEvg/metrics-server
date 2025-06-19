package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAgentBatchModes(t *testing.T) {
	basePort := 18090

	t.Run("Batch processing (default batch size 10)", func(t *testing.T) {
		port := basePort + 1
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

		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-r", "3", "-p", "1", "-b", "10")
		agent.Stdout = os.Stdout
		agent.Stderr = os.Stderr
		if err := agent.Start(); err != nil {
			t.Fatalf("Failed to start agent: %v", err)
		}
		time.Sleep(5 * time.Second)
		_ = agent.Process.Kill()
		_ = agent.Wait()

		checkMetricEndpoint(t, port, "gauge", "Alloc")
		checkMetricEndpoint(t, port, "counter", "PollCount")
	})

	t.Run("Individual sending (batch disabled)", func(t *testing.T) {
		port := basePort + 2
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

		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port), "-r", "3", "-p", "1", "-b", "0")
		agent.Stdout = os.Stdout
		agent.Stderr = os.Stderr
		if err := agent.Start(); err != nil {
			t.Fatalf("Failed to start agent: %v", err)
		}
		time.Sleep(5 * time.Second)
		_ = agent.Process.Kill()
		_ = agent.Wait()

		checkMetricEndpoint(t, port, "gauge", "Alloc")
		checkMetricEndpoint(t, port, "counter", "PollCount")
	})

	t.Run("Env var configuration (BATCH_SIZE, REPORT_INTERVAL)", func(t *testing.T) {
		port := basePort + 3
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

		agent := exec.Command("../../agent", "-a", "localhost:"+strconv.Itoa(port))
		agent.Env = append(os.Environ(), "BATCH_SIZE=5", "REPORT_INTERVAL=2")
		agent.Stdout = os.Stdout
		agent.Stderr = os.Stderr
		if err := agent.Start(); err != nil {
			t.Fatalf("Failed to start agent: %v", err)
		}
		time.Sleep(4 * time.Second)
		_ = agent.Process.Kill()
		_ = agent.Wait()

		checkMetricEndpoint(t, port, "gauge", "Alloc")
	})
}

func checkMetricEndpoint(t *testing.T, port int, metricType, metricName string) {
	url := "http://localhost:" + strconv.Itoa(port) + "/value/" + metricType + "/" + metricName
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("Failed to GET %s: %v", url, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Non-200 status for %s: %d", url, resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if len(strings.TrimSpace(string(body))) == 0 {
		t.Errorf("Empty response for %s", url)
	}
}
