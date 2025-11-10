package agent

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mutualEvg/metrics-server/internal/retry"
)

const (
	DefaultServerAddress  = "http://localhost:8080"
	DefaultPollInterval   = 2
	DefaultReportInterval = 10
	DefaultBatchSize      = 0  // Default to individual sending for backward compatibility
	DefaultRateLimit      = 10 // Default rate limit for concurrent requests
)

// Config holds all agent configuration
type Config struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
	BatchSize      int
	RateLimit      int
	Key            string
	CryptoKey      string // Path to public key file for encryption
	RetryConfig    retry.RetryConfig
}

// ParseConfig parses command line flags and environment variables
func ParseConfig() *Config {
	// Read flags
	flagAddress := flag.String("a", "", "HTTP server address (default: http://localhost:8080)")
	flagReport := flag.Int("r", 0, "Report interval in seconds (default: 10)")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds (default: 2)")
	flagBatchSize := flag.Int("b", 0, "Batch size for metrics (default: 10, 0 = disable batching)")
	flagDisableRetry := flag.Bool("disable-retry", false, "Disable retry logic for testing")
	flagKey := flag.String("k", "", "Key for SHA256 signature")
	flagCryptoKey := flag.String("crypto-key", "", "Path to public key file for encryption")
	flagRateLimit := flag.Int("l", 0, "Rate limit for concurrent requests (default: 10)")
	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatalf("Unknown flags: %v", flag.Args())
	}

	config := &Config{}

	// --- Address
	address := os.Getenv("ADDRESS")
	if address == "" {
		if *flagAddress != "" {
			address = *flagAddress
		} else {
			address = DefaultServerAddress
		}
	}
	config.ServerAddress = address

	if !strings.HasPrefix(config.ServerAddress, "http://") && !strings.HasPrefix(config.ServerAddress, "https://") {
		config.ServerAddress = "http://" + config.ServerAddress
	}

	// --- Key
	keyEnv := os.Getenv("KEY")
	if keyEnv != "" {
		config.Key = keyEnv
	} else if *flagKey != "" {
		config.Key = *flagKey
	}

	if config.Key != "" {
		log.Printf("SHA256 signature enabled")
	}

	// --- Crypto Key
	cryptoKeyEnv := os.Getenv("CRYPTO_KEY")
	if cryptoKeyEnv != "" {
		config.CryptoKey = cryptoKeyEnv
	} else if *flagCryptoKey != "" {
		config.CryptoKey = *flagCryptoKey
	}

	if config.CryptoKey != "" {
		log.Printf("Asymmetric encryption enabled with public key: %s", config.CryptoKey)
	}

	// --- Rate Limit
	rateLimitEnv := os.Getenv("RATE_LIMIT")
	if rateLimitEnv != "" {
		val, err := strconv.Atoi(rateLimitEnv)
		if err != nil {
			log.Fatalf("Invalid RATE_LIMIT: %v", err)
		}
		config.RateLimit = val
	} else if *flagRateLimit != 0 {
		config.RateLimit = *flagRateLimit
	} else {
		config.RateLimit = DefaultRateLimit
	}

	// --- Report Interval
	reportEnv := os.Getenv("REPORT_INTERVAL")
	var reportSeconds int
	if reportEnv != "" {
		val, err := strconv.Atoi(reportEnv)
		if err != nil {
			log.Fatalf("Invalid REPORT_INTERVAL: %v", err)
		}
		reportSeconds = val
	} else if *flagReport != 0 {
		reportSeconds = *flagReport
	} else {
		reportSeconds = DefaultReportInterval
	}
	config.ReportInterval = time.Duration(reportSeconds) * time.Second

	// --- Poll Interval
	pollEnv := os.Getenv("POLL_INTERVAL")
	var pollSeconds int
	if pollEnv != "" {
		val, err := strconv.Atoi(pollEnv)
		if err != nil {
			log.Fatalf("Invalid POLL_INTERVAL: %v", err)
		}
		pollSeconds = val
	} else if *flagPoll != 0 {
		pollSeconds = *flagPoll
	} else {
		pollSeconds = DefaultPollInterval
	}
	config.PollInterval = time.Duration(pollSeconds) * time.Second

	// --- Batch Size
	batchEnv := os.Getenv("BATCH_SIZE")
	if batchEnv != "" {
		val, err := strconv.Atoi(batchEnv)
		if err != nil {
			log.Fatalf("Invalid BATCH_SIZE: %v", err)
		}
		config.BatchSize = val
	} else if *flagBatchSize != 0 {
		config.BatchSize = *flagBatchSize
	} else {
		config.BatchSize = DefaultBatchSize
	}

	// --- Retry Configuration
	if os.Getenv("ENABLE_FULL_RETRY") == "true" {
		config.RetryConfig = retry.DefaultConfig()
	} else {
		config.RetryConfig = retry.FastConfig()
	}

	if *flagDisableRetry || os.Getenv("DISABLE_RETRY") == "true" {
		config.RetryConfig = retry.NoRetryConfig()
	} else if os.Getenv("TEST_MODE") == "true" {
		config.RetryConfig.MaxAttempts = 1
		config.RetryConfig.Intervals = []time.Duration{}
	}

	cryptoStatus := "disabled"
	if config.CryptoKey != "" {
		cryptoStatus = "enabled"
	}
	log.Printf("Agent starting with server=%s, poll=%v, report=%v, batch_size=%d, rate_limit=%d, crypto=%s",
		config.ServerAddress, config.PollInterval, config.ReportInterval, config.BatchSize, config.RateLimit, cryptoStatus)

	return config
}
