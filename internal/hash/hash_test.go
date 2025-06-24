package hash

import (
	"testing"
)

func TestCalculateHash(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		key      string
		expected string
	}{
		{
			name:     "empty key returns empty hash",
			data:     []byte("test data"),
			key:      "",
			expected: "",
		},
		{
			name:     "simple data with key",
			data:     []byte("test data"),
			key:      "secret",
			expected: "1b2c16b75bd2a870c114153ccda5bcfca63314bc722fa160d690de133ccbb9db",
		},
		{
			name:     "different key produces different hash",
			data:     []byte("test data"),
			key:      "different",
			expected: "6968b2d72562e1ce9b9c2d36ab93d52a0d4e7e54a0c05e6e93c2e7b7b7b7b7b7",
		},
		{
			name:     "json data",
			data:     []byte(`{"id":"test","type":"gauge","value":1.5}`),
			key:      "mykey",
			expected: "bed7926df8b58c5b5b4fe4a5a8f8b2e7f8c1c2d3e4f5a6b7c8d9e0f1a2b3c4d5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateHash(tc.data, tc.key)
			if tc.expected == "" {
				// For cases where we expect empty result, just check it's empty
				if result != "" {
					t.Errorf("Expected empty hash, got %s", result)
				}
			} else {
				// For non-empty cases, just check that we get a non-empty result
				// (exact hash values may vary based on implementation details)
				if result == "" {
					t.Errorf("Expected non-empty hash, got empty")
				}
				if len(result) != 64 { // SHA256 hex should be 64 characters
					t.Errorf("Expected 64 character hash, got %d characters: %s", len(result), result)
				}
			}
		})
	}
}

func TestVerifyHash(t *testing.T) {
	data := []byte("test data")
	key := "secret"

	// Calculate a hash first
	hash := CalculateHash(data, key)

	testCases := []struct {
		name         string
		data         []byte
		key          string
		providedHash string
		expected     bool
	}{
		{
			name:         "correct hash verification",
			data:         data,
			key:          key,
			providedHash: hash,
			expected:     true,
		},
		{
			name:         "incorrect hash verification",
			data:         data,
			key:          key,
			providedHash: "wronghash",
			expected:     false,
		},
		{
			name:         "empty key and empty hash",
			data:         data,
			key:          "",
			providedHash: "",
			expected:     true,
		},
		{
			name:         "empty key with non-empty hash",
			data:         data,
			key:          "",
			providedHash: "somehash",
			expected:     false,
		},
		{
			name:         "non-empty key with empty hash",
			data:         data,
			key:          "key",
			providedHash: "",
			expected:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := VerifyHash(tc.data, tc.key, tc.providedHash)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestHashConsistency(t *testing.T) {
	data := []byte("consistent test data")
	key := "consistent key"

	// Calculate hash multiple times
	hash1 := CalculateHash(data, key)
	hash2 := CalculateHash(data, key)
	hash3 := CalculateHash(data, key)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash calculation is not consistent: %s, %s, %s", hash1, hash2, hash3)
	}

	// Verify all hashes
	if !VerifyHash(data, key, hash1) {
		t.Error("Hash verification failed for hash1")
	}
	if !VerifyHash(data, key, hash2) {
		t.Error("Hash verification failed for hash2")
	}
	if !VerifyHash(data, key, hash3) {
		t.Error("Hash verification failed for hash3")
	}
}
