package agent

import (
	"encoding/json"
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
	DefaultBatchSize      = 10 // Default batch size for metrics
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
	GRPCAddress    string // gRPC server address (optional)
}

// JSONConfig represents the JSON configuration file structure for agent
type JSONConfig struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
	GRPCAddress    string `json:"grpc_address"`
}

// agentFlags holds all command-line flag values for the agent
type agentFlags struct {
	address        *string
	reportInterval *int
	pollInterval   *int
	batchSize      *int
	disableRetry   *bool
	key            *string
	cryptoKey      *string
	rateLimit      *int
	grpcAddress    *string
	configPath     *string
	configPathLong *string
}

// ParseConfig parses command line flags and environment variables
func ParseConfig() *Config {
	flags := parseAgentFlags()
	validateAgentFlags()
	jsonConfig := loadAgentJSONConfig(resolveAgentConfigPath(flags))

	config := &Config{
		ServerAddress:  resolveAgentServerAddress(flags, jsonConfig),
		PollInterval:   resolveAgentPollInterval(flags, jsonConfig),
		ReportInterval: resolveAgentReportInterval(flags, jsonConfig),
		BatchSize:      resolveAgentBatchSize(flags),
		RateLimit:      resolveAgentRateLimit(flags),
		Key:            resolveAgentKey(flags),
		CryptoKey:      resolveAgentCryptoKey(flags, jsonConfig),
		RetryConfig:    resolveAgentRetryConfig(flags),
		GRPCAddress:    resolveAgentGRPCAddress(flags, jsonConfig),
	}

	logAgentConfig(config)
	return config
}

// parseAgentFlags parses all command-line flags
func parseAgentFlags() *agentFlags {
	flags := &agentFlags{
		address:        flag.String("a", "", "HTTP server address (default: http://localhost:8080)"),
		reportInterval: flag.Int("r", 0, "Report interval in seconds (default: 10)"),
		pollInterval:   flag.Int("p", 0, "Poll interval in seconds (default: 2)"),
		batchSize:      flag.Int("b", 0, "Batch size for metrics (default: 10, 0 = disable batching)"),
		disableRetry:   flag.Bool("disable-retry", false, "Disable retry logic for testing"),
		key:            flag.String("k", "", "Key for SHA256 signature"),
		cryptoKey:      flag.String("crypto-key", "", "Path to public key file for encryption"),
		rateLimit:      flag.Int("l", 0, "Rate limit for concurrent requests (default: 10)"),
		grpcAddress:    flag.String("g", "", "gRPC server address"),
		configPath:     flag.String("c", "", "Path to JSON configuration file"),
		configPathLong: flag.String("config", "", "Path to JSON configuration file"),
	}
	flag.Parse()
	return flags
}

// validateAgentFlags validates that no unknown flags are provided
func validateAgentFlags() {
	if len(flag.Args()) > 0 {
		log.Fatalf("Unknown flags: %v", flag.Args())
	}
}

// resolveAgentConfigPath resolves the path to the JSON config file
func resolveAgentConfigPath(flags *agentFlags) string {
	if *flags.configPath != "" {
		return *flags.configPath
	}
	if *flags.configPathLong != "" {
		return *flags.configPathLong
	}
	return os.Getenv("CONFIG")
}

// loadAgentJSONConfig loads the agent JSON config file
func loadAgentJSONConfig(path string) *JSONConfig {
	if path == "" {
		return nil
	}

	config, err := loadJSONConfig(path)
	if err != nil {
		log.Printf("Warning: Failed to load config file %s: %v", path, err)
		return nil
	}

	log.Printf("Loaded configuration from %s", path)
	return config
}

// loadJSONConfig reads and parses the JSON config file
func loadJSONConfig(path string) (*JSONConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config JSONConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// resolveAgentServerAddress resolves the server address
func resolveAgentServerAddress(flags *agentFlags, jsonConfig *JSONConfig) string {
	address := os.Getenv("ADDRESS")
	if address == "" {
		if *flags.address != "" {
			address = *flags.address
		} else if jsonConfig != nil && jsonConfig.Address != "" {
			address = jsonConfig.Address
		} else {
			address = DefaultServerAddress
		}
	}

	// Ensure address has http:// or https:// prefix
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	return address
}

// resolveAgentKey resolves the signature key
func resolveAgentKey(flags *agentFlags) string {
	if key := os.Getenv("KEY"); key != "" {
		log.Printf("SHA256 signature enabled")
		return key
	}
	if *flags.key != "" {
		log.Printf("SHA256 signature enabled")
		return *flags.key
	}
	return ""
}

// resolveAgentCryptoKey resolves the crypto key path
func resolveAgentCryptoKey(flags *agentFlags, jsonConfig *JSONConfig) string {
	if cryptoKey := os.Getenv("CRYPTO_KEY"); cryptoKey != "" {
		log.Printf("Asymmetric encryption enabled with public key: %s", cryptoKey)
		return cryptoKey
	}
	if *flags.cryptoKey != "" {
		log.Printf("Asymmetric encryption enabled with public key: %s", *flags.cryptoKey)
		return *flags.cryptoKey
	}
	if jsonConfig != nil && jsonConfig.CryptoKey != "" {
		log.Printf("Asymmetric encryption enabled with public key: %s", jsonConfig.CryptoKey)
		return jsonConfig.CryptoKey
	}
	return ""
}

// resolveAgentRateLimit resolves the rate limit
func resolveAgentRateLimit(flags *agentFlags) int {
	if rateLimitEnv := os.Getenv("RATE_LIMIT"); rateLimitEnv != "" {
		val, err := strconv.Atoi(rateLimitEnv)
		if err != nil {
			log.Fatalf("Invalid RATE_LIMIT: %v", err)
		}
		return val
	}
	if *flags.rateLimit != 0 {
		return *flags.rateLimit
	}
	return DefaultRateLimit
}

// resolveAgentReportInterval resolves the report interval
func resolveAgentReportInterval(flags *agentFlags, jsonConfig *JSONConfig) time.Duration {
	if reportEnv := os.Getenv("REPORT_INTERVAL"); reportEnv != "" {
		val, err := strconv.Atoi(reportEnv)
		if err != nil {
			log.Fatalf("Invalid REPORT_INTERVAL: %v", err)
		}
		return time.Duration(val) * time.Second
	}
	if *flags.reportInterval != 0 {
		return time.Duration(*flags.reportInterval) * time.Second
	}
	if jsonConfig != nil && jsonConfig.ReportInterval != "" {
		return parseAgentIntervalFromJSON("report_interval", jsonConfig.ReportInterval)
	}
	return time.Duration(DefaultReportInterval) * time.Second
}

// resolveAgentPollInterval resolves the poll interval
func resolveAgentPollInterval(flags *agentFlags, jsonConfig *JSONConfig) time.Duration {
	if pollEnv := os.Getenv("POLL_INTERVAL"); pollEnv != "" {
		val, err := strconv.Atoi(pollEnv)
		if err != nil {
			log.Fatalf("Invalid POLL_INTERVAL: %v", err)
		}
		return time.Duration(val) * time.Second
	}
	if *flags.pollInterval != 0 {
		return time.Duration(*flags.pollInterval) * time.Second
	}
	if jsonConfig != nil && jsonConfig.PollInterval != "" {
		return parseAgentIntervalFromJSON("poll_interval", jsonConfig.PollInterval)
	}
	return time.Duration(DefaultPollInterval) * time.Second
}

// parseAgentIntervalFromJSON parses a time interval from JSON string
func parseAgentIntervalFromJSON(name, interval string) time.Duration {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.Fatalf("Invalid %s in config file: %v", name, err)
	}
	return duration
}

// resolveAgentBatchSize resolves the batch size
func resolveAgentBatchSize(flags *agentFlags) int {
	if batchEnv := os.Getenv("BATCH_SIZE"); batchEnv != "" {
		val, err := strconv.Atoi(batchEnv)
		if err != nil {
			log.Fatalf("Invalid BATCH_SIZE: %v", err)
		}
		return val
	}
	if *flags.batchSize != 0 {
		return *flags.batchSize
	}
	return DefaultBatchSize
}

// resolveAgentRetryConfig resolves the retry configuration
func resolveAgentRetryConfig(flags *agentFlags) retry.RetryConfig {
	// Check for disabled retry first
	if *flags.disableRetry || os.Getenv("DISABLE_RETRY") == "true" {
		return retry.NoRetryConfig()
	}

	// Test mode with minimal retry
	if os.Getenv("TEST_MODE") == "true" {
		config := retry.FastConfig()
		config.MaxAttempts = 1
		config.Intervals = []time.Duration{}
		return config
	}

	// Full retry or fast config
	if os.Getenv("ENABLE_FULL_RETRY") == "true" {
		return retry.DefaultConfig()
	}

	return retry.FastConfig()
}

// resolveAgentGRPCAddress resolves the gRPC server address
func resolveAgentGRPCAddress(flags *agentFlags, jsonConfig *JSONConfig) string {
	if grpcAddr := os.Getenv("GRPC_ADDRESS"); grpcAddr != "" {
		log.Printf("gRPC enabled: %s", grpcAddr)
		return grpcAddr
	}
	if *flags.grpcAddress != "" {
		log.Printf("gRPC enabled: %s", *flags.grpcAddress)
		return *flags.grpcAddress
	}
	if jsonConfig != nil && jsonConfig.GRPCAddress != "" {
		log.Printf("gRPC enabled: %s", jsonConfig.GRPCAddress)
		return jsonConfig.GRPCAddress
	}
	return ""
}

// logAgentConfig logs the final configuration
func logAgentConfig(config *Config) {
	cryptoStatus := "disabled"
	if config.CryptoKey != "" {
		cryptoStatus = "enabled"
	}
	grpcStatus := "disabled"
	if config.GRPCAddress != "" {
		grpcStatus = config.GRPCAddress
	}
	log.Printf("Agent starting with server=%s, poll=%v, report=%v, batch_size=%d, rate_limit=%d, crypto=%s, grpc=%s",
		config.ServerAddress, config.PollInterval, config.ReportInterval, config.BatchSize, config.RateLimit, cryptoStatus, grpcStatus)
}
