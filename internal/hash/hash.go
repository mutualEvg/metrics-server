package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// CalculateHash calculates SHA256 HMAC hash of data with the given key
func CalculateHash(data []byte, key string) string {
	if key == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHash verifies that the provided hash matches the calculated hash of data with key
func VerifyHash(data []byte, key, providedHash string) bool {
	if key == "" || providedHash == "" {
		return key == "" && providedHash == "" // Both should be empty if no key
	}

	calculatedHash := CalculateHash(data, key)
	return providedHash == calculatedHash
}

// HashReader reads all data from reader and returns data + hash
func HashReader(reader io.Reader, key string) ([]byte, string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}

	hash := CalculateHash(data, key)
	return data, hash, nil
}
