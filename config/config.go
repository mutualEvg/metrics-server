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
	ServerAddress   string
	PollInterval    time.Duration
	ReportInterval  time.Duration
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
}

const (
	defaultServerAddress   = "http://localhost:8080"
	defaultPollSeconds     = 2
	defaultReportSeconds   = 10
	defaultStoreSeconds    = 300
	defaultFileStoragePath = "/tmp/metrics-db.json"
	defaultRestore         = true
)

func Load() *Config {
	// Flags
	flagAddress := flag.String("a", "", "HTTP server address")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds")
	flagReport := flag.Int("r", 0, "Report interval in seconds")
	flagStoreInterval := flag.Int("i", 0, "Store interval in seconds (0 for synchronous)")
	flagFileStoragePath := flag.String("f", "", "File storage path")
	flagRestore := flag.Bool("restore", false, "Restore previously stored values")
	flag.Parse()

	addr := resolveString("ADDRESS", *flagAddress, defaultServerAddress)
	poll := resolveInt("POLL_INTERVAL", *flagPoll, defaultPollSeconds)
	report := resolveInt("REPORT_INTERVAL", *flagReport, defaultReportSeconds)
	storeInterval := resolveInt("STORE_INTERVAL", *flagStoreInterval, defaultStoreSeconds)
	fileStoragePath := resolveString("FILE_STORAGE_PATH", *flagFileStoragePath, defaultFileStoragePath)
	restore := resolveBool("RESTORE", *flagRestore, defaultRestore)

	return &Config{
		ServerAddress:   addr,
		PollInterval:    time.Duration(poll) * time.Second,
		ReportInterval:  time.Duration(report) * time.Second,
		StoreInterval:   time.Duration(storeInterval) * time.Second,
		FileStoragePath: fileStoragePath,
		Restore:         restore,
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

func resolveBool(envVar string, flagVal, def bool) bool {
	if val := os.Getenv(envVar); val != "" {
		b, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Invalid %s: %v", envVar, err)
		}
		return b
	}
	if flagVal {
		return flagVal
	}
	return def
}
