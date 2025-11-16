package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadJSONConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	testConfig := JSONConfig{
		Address:       "localhost:9090",
		Restore:       boolPtr(false),
		StoreInterval: "60s",
		StoreFile:     "/custom/path.json",
		DatabaseDSN:   "postgresql://localhost/test",
		CryptoKey:     "/path/to/key.pem",
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loaded, err := loadJSONConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Address != testConfig.Address {
		t.Errorf("Expected address %s, got %s", testConfig.Address, loaded.Address)
	}

	if loaded.Restore == nil || *loaded.Restore != false {
		t.Errorf("Expected restore false, got %v", loaded.Restore)
	}

	if loaded.StoreInterval != testConfig.StoreInterval {
		t.Errorf("Expected store_interval %s, got %s", testConfig.StoreInterval, loaded.StoreInterval)
	}

	if loaded.StoreFile != testConfig.StoreFile {
		t.Errorf("Expected store_file %s, got %s", testConfig.StoreFile, loaded.StoreFile)
	}

	if loaded.DatabaseDSN != testConfig.DatabaseDSN {
		t.Errorf("Expected database_dsn %s, got %s", testConfig.DatabaseDSN, loaded.DatabaseDSN)
	}

	if loaded.CryptoKey != testConfig.CryptoKey {
		t.Errorf("Expected crypto_key %s, got %s", testConfig.CryptoKey, loaded.CryptoKey)
	}
}

func TestLoadJSONConfigInvalidFile(t *testing.T) {
	_, err := loadJSONConfig("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadJSONConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(configPath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = loadJSONConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestResolveStringWithJSON(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		envVal     string
		flagVal    string
		jsonGetter func() string
		def        string
		expected   string
	}{
		{
			name:       "flag takes priority",
			envVar:     "TEST_VAR",
			envVal:     "env_value",
			flagVal:    "flag_value",
			jsonGetter: func() string { return "json_value" },
			def:        "default_value",
			expected:   "env_value", // env takes priority over flag in our resolveStringWithJSON
		},
		{
			name:       "flag when no env",
			envVar:     "TEST_VAR_EMPTY",
			envVal:     "",
			flagVal:    "flag_value",
			jsonGetter: func() string { return "json_value" },
			def:        "default_value",
			expected:   "flag_value",
		},
		{
			name:       "json when no flag or env",
			envVar:     "TEST_VAR_EMPTY2",
			envVal:     "",
			flagVal:    "",
			jsonGetter: func() string { return "json_value" },
			def:        "default_value",
			expected:   "json_value",
		},
		{
			name:       "default when nothing set",
			envVar:     "TEST_VAR_EMPTY3",
			envVal:     "",
			flagVal:    "",
			jsonGetter: func() string { return "" },
			def:        "default_value",
			expected:   "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envVar, tt.envVal)
				defer os.Unsetenv(tt.envVar)
			}

			result := resolveStringWithJSON(tt.envVar, tt.flagVal, tt.jsonGetter, tt.def)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestResolveBoolWithJSON(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		envVal     string
		flagVal    bool
		jsonGetter func() *bool
		def        bool
		expected   bool
	}{
		{
			name:       "env takes priority",
			envVar:     "TEST_BOOL",
			envVal:     "true",
			flagVal:    false,
			jsonGetter: func() *bool { return boolPtr(false) },
			def:        false,
			expected:   true,
		},
		{
			name:       "flag when no env",
			envVar:     "TEST_BOOL_EMPTY",
			envVal:     "",
			flagVal:    true,
			jsonGetter: func() *bool { return boolPtr(false) },
			def:        false,
			expected:   true,
		},
		{
			name:       "json when no flag or env",
			envVar:     "TEST_BOOL_EMPTY2",
			envVal:     "",
			flagVal:    false,
			jsonGetter: func() *bool { return boolPtr(true) },
			def:        false,
			expected:   true,
		},
		{
			name:       "default when nothing set",
			envVar:     "TEST_BOOL_EMPTY3",
			envVal:     "",
			flagVal:    false,
			jsonGetter: func() *bool { return nil },
			def:        true,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envVar, tt.envVal)
				defer os.Unsetenv(tt.envVar)
			}

			result := resolveBoolWithJSON(tt.envVar, tt.flagVal, tt.jsonGetter, tt.def)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestResolveIntWithJSON(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		envVal     string
		flagVal    int
		jsonGetter func() int
		def        int
		expected   int
	}{
		{
			name:       "env takes priority",
			envVar:     "TEST_INT",
			envVal:     "100",
			flagVal:    50,
			jsonGetter: func() int { return 25 },
			def:        10,
			expected:   100,
		},
		{
			name:       "flag when no env",
			envVar:     "TEST_INT_EMPTY",
			envVal:     "",
			flagVal:    50,
			jsonGetter: func() int { return 25 },
			def:        10,
			expected:   50,
		},
		{
			name:       "json when no flag or env",
			envVar:     "TEST_INT_EMPTY2",
			envVal:     "",
			flagVal:    0,
			jsonGetter: func() int { return 25 },
			def:        10,
			expected:   25,
		},
		{
			name:       "default when nothing set",
			envVar:     "TEST_INT_EMPTY3",
			envVal:     "",
			flagVal:    0,
			jsonGetter: func() int { return 0 },
			def:        10,
			expected:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envVar, tt.envVal)
				defer os.Unsetenv(tt.envVar)
			}

			result := resolveIntWithJSON(tt.envVar, tt.flagVal, tt.jsonGetter, tt.def)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestStoreIntervalParsing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	testConfig := JSONConfig{
		Address:       "localhost:8080",
		StoreInterval: "60s",
	}

	data, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loaded, err := loadJSONConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	duration, err := time.ParseDuration(loaded.StoreInterval)
	if err != nil {
		t.Fatalf("Failed to parse store_interval: %v", err)
	}

	if duration != 60*time.Second {
		t.Errorf("Expected 60 seconds, got %v", duration)
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
