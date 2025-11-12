// config/config.go
package config

import (
	"encoding/json"
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
	DatabaseDSN     string
	UseFileStorage  bool   // Indicates if file storage was explicitly configured
	Key             string // Key for SHA256 signature verification
	CryptoKey       string // Path to private key file for decryption
	AuditFile       string // Path to audit log file (optional)
	AuditURL        string // URL for remote audit server (optional)
}

// JSONConfig represents the JSON configuration file structure for server
type JSONConfig struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore"` // pointer to distinguish between false and not set
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
}

// configFlags holds all command-line flag values
type configFlags struct {
	address         *string
	pollInterval    *int
	storeInterval   *int
	fileStoragePath *string
	restore         *bool
	databaseDSN     *string
	key             *string
	cryptoKey       *string
	auditFile       *string
	auditURL        *string
	configPath      *string
	configPathLong  *string
}

const (
	defaultServerAddress   = "http://localhost:8080"
	defaultPollSeconds     = 2
	defaultReportSeconds   = 10
	defaultStoreSeconds    = 300
	defaultFileStoragePath = "/tmp/metrics-db.json"
	defaultRestore         = true
	defaultDatabaseDSN     = ""
)

// Load loads configuration from flags, environment variables, and JSON file
func Load() *Config {
	flags := parseFlags()
	jsonConfig := loadJSONConfigFile(resolveConfigPath(flags))

	return &Config{
		ServerAddress:   resolveServerAddress(flags, jsonConfig),
		PollInterval:    resolvePollInterval(flags),
		ReportInterval:  resolveReportInterval(),
		StoreInterval:   resolveStoreInterval(flags, jsonConfig),
		FileStoragePath: resolveFileStoragePath(flags, jsonConfig),
		Restore:         resolveRestore(flags, jsonConfig),
		DatabaseDSN:     resolveDatabaseDSN(flags, jsonConfig),
		UseFileStorage:  shouldUseFileStorage(flags, jsonConfig),
		Key:             resolveKey(flags),
		CryptoKey:       resolveCryptoKey(flags, jsonConfig),
		AuditFile:       resolveAuditFile(flags),
		AuditURL:        resolveAuditURL(flags),
	}
}

// parseFlags parses all command-line flags
func parseFlags() *configFlags {
	flags := &configFlags{
		address:         flag.String("a", "", "HTTP server address"),
		pollInterval:    flag.Int("p", 0, "Poll interval in seconds"),
		storeInterval:   flag.Int("i", 0, "Store interval in seconds (0 for synchronous)"),
		fileStoragePath: flag.String("f", "", "File storage path"),
		restore:         flag.Bool("r", false, "Restore previously stored values"),
		databaseDSN:     flag.String("d", "", "Database connection string"),
		key:             flag.String("k", "", "Key for SHA256 signature"),
		cryptoKey:       flag.String("crypto-key", "", "Path to private key file for decryption"),
		auditFile:       flag.String("audit-file", "", "Path to audit log file"),
		auditURL:        flag.String("audit-url", "", "URL for remote audit server"),
		configPath:      flag.String("c", "", "Path to JSON configuration file"),
		configPathLong:  flag.String("config", "", "Path to JSON configuration file"),
	}
	flag.Parse()
	return flags
}

// resolveConfigPath resolves the path to the JSON config file
func resolveConfigPath(flags *configFlags) string {
	if *flags.configPath != "" {
		return *flags.configPath
	}
	if *flags.configPathLong != "" {
		return *flags.configPathLong
	}
	return os.Getenv("CONFIG")
}

// loadJSONConfigFile loads the JSON config file if path is provided
func loadJSONConfigFile(path string) *JSONConfig {
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

// resolveServerAddress resolves the server address
func resolveServerAddress(flags *configFlags, jsonConfig *JSONConfig) string {
	return resolveStringWithJSON("ADDRESS", *flags.address, func() string {
		if jsonConfig != nil {
			return jsonConfig.Address
		}
		return ""
	}, defaultServerAddress)
}

// resolvePollInterval resolves the poll interval
func resolvePollInterval(flags *configFlags) time.Duration {
	seconds := resolveInt("POLL_INTERVAL", *flags.pollInterval, defaultPollSeconds)
	return time.Duration(seconds) * time.Second
}

// resolveReportInterval resolves the report interval
func resolveReportInterval() time.Duration {
	seconds := resolveInt("REPORT_INTERVAL", 0, defaultReportSeconds)
	return time.Duration(seconds) * time.Second
}

// resolveStoreInterval resolves the store interval
func resolveStoreInterval(flags *configFlags, jsonConfig *JSONConfig) time.Duration {
	seconds := resolveIntWithJSON("STORE_INTERVAL", *flags.storeInterval, func() int {
		if jsonConfig != nil && jsonConfig.StoreInterval != "" {
			return parseStoreIntervalFromJSON(jsonConfig.StoreInterval)
		}
		return 0
	}, defaultStoreSeconds)
	return time.Duration(seconds) * time.Second
}

// parseStoreIntervalFromJSON parses the store interval from JSON string
func parseStoreIntervalFromJSON(interval string) int {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.Printf("Warning: Invalid store_interval in config file: %v", err)
		return 0
	}
	return int(duration.Seconds())
}

// resolveDatabaseDSN resolves the database DSN
func resolveDatabaseDSN(flags *configFlags, jsonConfig *JSONConfig) string {
	return resolveStringWithJSON("DATABASE_DSN", *flags.databaseDSN, func() string {
		if jsonConfig != nil {
			return jsonConfig.DatabaseDSN
		}
		return ""
	}, defaultDatabaseDSN)
}

// resolveRestore resolves the restore flag
func resolveRestore(flags *configFlags, jsonConfig *JSONConfig) bool {
	return resolveBoolWithJSON("RESTORE", *flags.restore, func() *bool {
		if jsonConfig != nil {
			return jsonConfig.Restore
		}
		return nil
	}, defaultRestore)
}

// resolveKey resolves the signature key
func resolveKey(flags *configFlags) string {
	return resolveString("KEY", *flags.key, "")
}

// resolveCryptoKey resolves the crypto key path
func resolveCryptoKey(flags *configFlags, jsonConfig *JSONConfig) string {
	return resolveStringWithJSON("CRYPTO_KEY", *flags.cryptoKey, func() string {
		if jsonConfig != nil {
			return jsonConfig.CryptoKey
		}
		return ""
	}, "")
}

// resolveAuditFile resolves the audit file path
func resolveAuditFile(flags *configFlags) string {
	return resolveString("AUDIT_FILE", *flags.auditFile, "")
}

// resolveAuditURL resolves the audit URL
func resolveAuditURL(flags *configFlags) string {
	return resolveString("AUDIT_URL", *flags.auditURL, "")
}

// resolveFileStoragePath resolves the file storage path
func resolveFileStoragePath(flags *configFlags, jsonConfig *JSONConfig) string {
	// Flag has highest priority
	if *flags.fileStoragePath != "" {
		return *flags.fileStoragePath
	}

	// Environment variable
	if envPath := os.Getenv("FILE_STORAGE_PATH"); envPath != "" {
		return envPath
	}

	// JSON config
	if jsonConfig != nil && jsonConfig.StoreFile != "" {
		return jsonConfig.StoreFile
	}

	// Default
	return defaultFileStoragePath
}

// shouldUseFileStorage determines if file storage should be used
func shouldUseFileStorage(flags *configFlags, jsonConfig *JSONConfig) bool {
	return *flags.fileStoragePath != "" ||
		os.Getenv("FILE_STORAGE_PATH") != "" ||
		(jsonConfig != nil && jsonConfig.StoreFile != "")
}

// Utility functions for resolving values with priority

// resolveString resolves value with priority: env > flag > default
func resolveString(envVar, flagVal, def string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	if flagVal != "" {
		return flagVal
	}
	return def
}

// resolveStringWithJSON resolves value with priority: env > flag > json > default
func resolveStringWithJSON(envVar, flagVal string, jsonGetter func() string, def string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	if flagVal != "" {
		return flagVal
	}
	if jsonVal := jsonGetter(); jsonVal != "" {
		return jsonVal
	}
	return def
}

// resolveInt resolves integer value with priority: env > flag > default
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

// resolveIntWithJSON resolves integer value with priority: env > flag > json > default
func resolveIntWithJSON(envVar string, flagVal int, jsonGetter func() int, def int) int {
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
	if jsonVal := jsonGetter(); jsonVal != 0 {
		return jsonVal
	}
	return def
}

// resolveBool resolves boolean value with priority: env > flag > default
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

// resolveBoolWithJSON resolves boolean value with priority: env > flag > json > default
func resolveBoolWithJSON(envVar string, flagVal bool, jsonGetter func() *bool, def bool) bool {
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
	if jsonVal := jsonGetter(); jsonVal != nil {
		return *jsonVal
	}
	return def
}
