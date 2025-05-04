// config/config.go
package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

const (
	defaultServerAddress = "http://localhost:8080"
	defaultPollSeconds   = 2
	defaultReportSeconds = 10
)

func Load() *Config {
	// Flags
	flagAddress := flag.String("a", "", "HTTP server address")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds")
	flagReport := flag.Int("r", 0, "Report interval in seconds")
	flag.Parse()

	addr := resolveString("ADDRESS", *flagAddress, defaultServerAddress)
	poll := resolveInt("POLL_INTERVAL", *flagPoll, defaultPollSeconds)
	report := resolveInt("REPORT_INTERVAL", *flagReport, defaultReportSeconds)

	return &Config{
		ServerAddress:  addr,
		PollInterval:   time.Duration(poll) * time.Second,
		ReportInterval: time.Duration(report) * time.Second,
	}
}

func resolveString(envVar, flagVal, def string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	if flagVal != "" {
		return flagVal
	}
	return def
}

func resolveInt(envVar string, flagVal, def int) int {
	if val := os.Getenv(envVar); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("Invalid %s: %v", envVar, err)
		}
		return i
	}
	if flagVal != 0 {
		return flagVal
	}
	return def
}
