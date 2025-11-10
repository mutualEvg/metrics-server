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

const (
	defaultServerAddress   = "http://localhost:8080"
	defaultPollSeconds     = 2
	defaultReportSeconds   = 10
	defaultStoreSeconds    = 300
	defaultFileStoragePath = "/tmp/metrics-db.json"
	defaultRestore         = true
	defaultDatabaseDSN     = ""
)

func Load() *Config {
	// Flags
	flagAddress := flag.String("a", "", "HTTP server address")
	flagPoll := flag.Int("p", 0, "Poll interval in seconds")
	flagStoreInterval := flag.Int("i", 0, "Store interval in seconds (0 for synchronous)")
	flagFileStoragePath := flag.String("f", "", "File storage path")
	flagRestore := flag.Bool("r", false, "Restore previously stored values")
	flagDatabaseDSN := flag.String("d", "", "Database connection string")
	flagKey := flag.String("k", "", "Key for SHA256 signature")
	flagCryptoKey := flag.String("crypto-key", "", "Path to private key file for decryption")
	flagAuditFile := flag.String("audit-file", "", "Path to audit log file")
	flagAuditURL := flag.String("audit-url", "", "URL for remote audit server")
	flagConfig := flag.String("c", "", "Path to JSON configuration file")
	flagConfigLong := flag.String("config", "", "Path to JSON configuration file")
	flag.Parse()

	// Determine config file path (flag or env variable)
	configPath := *flagConfig
	if configPath == "" {
		configPath = *flagConfigLong
	}
	if configPath == "" {
		configPath = os.Getenv("CONFIG")
	}

	// Load JSON config if specified
	var jsonConfig *JSONConfig
	if configPath != "" {
		var err error
		jsonConfig, err = loadJSONConfig(configPath)
		if err != nil {
			log.Printf("Warning: Failed to load config file %s: %v", configPath, err)
		} else {
			log.Printf("Loaded configuration from %s", configPath)
		}
	}

	// Resolve values with priority: flag > env > json > default
	addr := resolveStringWithJSON("ADDRESS", *flagAddress, func() string {
		if jsonConfig != nil {
			return jsonConfig.Address
		}
		return ""
	}, defaultServerAddress)

	poll := resolveInt("POLL_INTERVAL", *flagPoll, defaultPollSeconds)
	report := resolveInt("REPORT_INTERVAL", 0, defaultReportSeconds)

	storeInterval := resolveIntWithJSON("STORE_INTERVAL", *flagStoreInterval, func() int {
		if jsonConfig != nil && jsonConfig.StoreInterval != "" {
			duration, err := time.ParseDuration(jsonConfig.StoreInterval)
			if err != nil {
				log.Printf("Warning: Invalid store_interval in config file: %v", err)
				return 0
			}
			return int(duration.Seconds())
		}
		return 0
	}, defaultStoreSeconds)

	databaseDSN := resolveStringWithJSON("DATABASE_DSN", *flagDatabaseDSN, func() string {
		if jsonConfig != nil {
			return jsonConfig.DatabaseDSN
		}
		return ""
	}, defaultDatabaseDSN)

	restore := resolveBoolWithJSON("RESTORE", *flagRestore, func() *bool {
		if jsonConfig != nil {
			return jsonConfig.Restore
		}
		return nil
	}, defaultRestore)

	key := resolveString("KEY", *flagKey, "")

	cryptoKey := resolveStringWithJSON("CRYPTO_KEY", *flagCryptoKey, func() string {
		if jsonConfig != nil {
			return jsonConfig.CryptoKey
		}
		return ""
	}, "")

	auditFile := resolveString("AUDIT_FILE", *flagAuditFile, "")
	auditURL := resolveString("AUDIT_URL", *flagAuditURL, "")

	// Determine file storage path with JSON config support
	var fileStoragePath string
	var useFileStorage bool

	if *flagFileStoragePath != "" {
		// Flag has highest priority
		fileStoragePath = *flagFileStoragePath
		useFileStorage = true
	} else if envPath := os.Getenv("FILE_STORAGE_PATH"); envPath != "" {
		// Environment variable
		fileStoragePath = envPath
		useFileStorage = true
	} else if jsonConfig != nil && jsonConfig.StoreFile != "" {
		// JSON config
		fileStoragePath = jsonConfig.StoreFile
		useFileStorage = true
	} else {
		// Default
		fileStoragePath = defaultFileStoragePath
		useFileStorage = false
	}

	return &Config{
		ServerAddress:   addr,
		PollInterval:    time.Duration(poll) * time.Second,
		ReportInterval:  time.Duration(report) * time.Second,
		StoreInterval:   time.Duration(storeInterval) * time.Second,
		FileStoragePath: fileStoragePath,
		Restore:         restore,
		DatabaseDSN:     databaseDSN,
		UseFileStorage:  useFileStorage,
		Key:             key,
		CryptoKey:       cryptoKey,
		AuditFile:       auditFile,
		AuditURL:        auditURL,
	}
}

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
