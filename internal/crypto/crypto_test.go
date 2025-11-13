package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if privateKey == nil {
		t.Fatal("Private key is nil")
	}

	if publicKey == nil {
		t.Fatal("Public key is nil")
	}

	if privateKey.N.BitLen() != DefaultKeySize {
		t.Errorf("Expected %d-bit key, got %d-bit", DefaultKeySize, privateKey.N.BitLen())
	}
}

func TestGenerateKeyPairMinimumSize(t *testing.T) {
	_, _, err := GenerateKeyPair(1024) // Less than minimum
	if err == nil {
		t.Error("Expected error for key size less than minimum")
	}
}

func TestEncryptDecryptRSA(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testData := []byte("Hello, World!")

	// Encrypt
	ciphertext, err := EncryptRSA(testData, publicKey)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, testData) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	plaintext, err := DecryptRSA(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, testData) {
		t.Errorf("Decrypted data doesn't match original. Got %s, expected %s", plaintext, testData)
	}
}

func TestEncryptDecryptRSAChunked(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "small data",
			data: []byte("Hello, World!"),
		},
		{
			name: "medium data",
			data: bytes.Repeat([]byte("A"), 500),
		},
		{
			name: "large data",
			data: bytes.Repeat([]byte("B"), 2000),
		},
		{
			name: "very large data",
			data: bytes.Repeat([]byte("C"), 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := EncryptRSAChunked(tt.data, publicKey)
			if err != nil {
				t.Fatalf("Failed to encrypt chunked data: %v", err)
			}

			// Decrypt
			plaintext, err := DecryptRSAChunked(ciphertext, privateKey)
			if err != nil {
				t.Fatalf("Failed to decrypt chunked data: %v", err)
			}

			if !bytes.Equal(plaintext, tt.data) {
				t.Errorf("Decrypted data doesn't match original. Length: got %d, expected %d", len(plaintext), len(tt.data))
			}
		})
	}
}

func TestSaveLoadPrivateKey(t *testing.T) {
	privateKey, _, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temp file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")

	// Save
	err = SavePrivateKeyToFile(keyPath, privateKey)
	if err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	// Load
	loadedKey, err := LoadPrivateKeyFromFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}

	// Verify they are the same
	if privateKey.N.Cmp(loadedKey.N) != 0 {
		t.Error("Loaded private key doesn't match original")
	}
}

func TestSaveLoadPublicKey(t *testing.T) {
	_, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temp file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "public.pem")

	// Save
	err = SavePublicKeyToFile(keyPath, publicKey)
	if err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Load
	loadedKey, err := LoadPublicKeyFromFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	// Verify they are the same
	if publicKey.N.Cmp(loadedKey.N) != 0 {
		t.Error("Loaded public key doesn't match original")
	}
}

func TestLoadInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("load non-existent private key", func(t *testing.T) {
		_, err := LoadPrivateKeyFromFile(filepath.Join(tmpDir, "nonexistent.pem"))
		if err == nil {
			t.Error("Expected error loading non-existent file")
		}
	})

	t.Run("load non-existent public key", func(t *testing.T) {
		_, err := LoadPublicKeyFromFile(filepath.Join(tmpDir, "nonexistent.pem"))
		if err == nil {
			t.Error("Expected error loading non-existent file")
		}
	})

	t.Run("load invalid PEM file", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.pem")
		err := os.WriteFile(invalidPath, []byte("not a valid PEM file"), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		_, err = LoadPrivateKeyFromFile(invalidPath)
		if err == nil {
			t.Error("Expected error loading invalid PEM file")
		}

		_, err = LoadPublicKeyFromFile(invalidPath)
		if err == nil {
			t.Error("Expected error loading invalid PEM file")
		}
	})
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Generate keys
	privateKey, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save keys to temp files
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "private.pem")
	pubPath := filepath.Join(tmpDir, "public.pem")

	err = SavePrivateKeyToFile(privPath, privateKey)
	if err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	err = SavePublicKeyToFile(pubPath, publicKey)
	if err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Load keys from files
	loadedPriv, err := LoadPrivateKeyFromFile(privPath)
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}

	loadedPub, err := LoadPublicKeyFromFile(pubPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	// Test encryption/decryption with loaded keys
	testData := []byte("Secret message for testing")

	ciphertext, err := EncryptRSAChunked(testData, loadedPub)
	if err != nil {
		t.Fatalf("Failed to encrypt with loaded public key: %v", err)
	}

	plaintext, err := DecryptRSAChunked(ciphertext, loadedPriv)
	if err != nil {
		t.Fatalf("Failed to decrypt with loaded private key: %v", err)
	}

	if !bytes.Equal(plaintext, testData) {
		t.Errorf("Round-trip encryption/decryption failed. Got %s, expected %s", plaintext, testData)
	}
}

func TestEncryptRSAWithNilKey(t *testing.T) {
	_, err := EncryptRSA([]byte("test"), nil)
	if err == nil {
		t.Error("Expected error when encrypting with nil key")
	}
}

func TestDecryptRSAWithNilKey(t *testing.T) {
	_, err := DecryptRSA([]byte("test"), nil)
	if err == nil {
		t.Error("Expected error when decrypting with nil key")
	}
}

func TestParsePEMFunctions(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(DefaultKeySize)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	t.Run("encode and parse public key PEM", func(t *testing.T) {
		pemData, err := EncodePublicKeyPEM(publicKey)
		if err != nil {
			t.Fatalf("Failed to encode public key: %v", err)
		}

		parsedKey, err := ParsePublicKeyPEM(pemData)
		if err != nil {
			t.Fatalf("Failed to parse public key PEM: %v", err)
		}

		if publicKey.N.Cmp(parsedKey.N) != 0 {
			t.Error("Parsed public key doesn't match original")
		}
	})

	t.Run("encode and parse private key PEM", func(t *testing.T) {
		pemData, err := EncodePrivateKeyPEM(privateKey)
		if err != nil {
			t.Fatalf("Failed to encode private key: %v", err)
		}

		parsedKey, err := ParsePrivateKeyPEM(pemData)
		if err != nil {
			t.Fatalf("Failed to parse private key PEM: %v", err)
		}

		if privateKey.N.Cmp(parsedKey.N) != 0 {
			t.Error("Parsed private key doesn't match original")
		}
	})
}

func BenchmarkEncryptRSA(b *testing.B) {
	_, publicKey, _ := GenerateKeyPair(DefaultKeySize)
	data := []byte("Test data for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncryptRSA(data, publicKey)
	}
}

func BenchmarkDecryptRSA(b *testing.B) {
	privKey, publicKey, _ := GenerateKeyPair(DefaultKeySize)
	data := []byte("Test data for benchmarking")
	ciphertext, _ := EncryptRSA(data, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecryptRSA(ciphertext, privKey)
	}
}

func BenchmarkEncryptRSAChunked(b *testing.B) {
	_, publicKey, _ := GenerateKeyPair(DefaultKeySize)
	data := bytes.Repeat([]byte("A"), 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncryptRSAChunked(data, publicKey)
	}
}

func BenchmarkDecryptRSAChunked(b *testing.B) {
	privKey, publicKey, _ := GenerateKeyPair(DefaultKeySize)
	data := bytes.Repeat([]byte("A"), 1000)
	ciphertext, _ := EncryptRSAChunked(data, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecryptRSAChunked(ciphertext, privKey)
	}
}
