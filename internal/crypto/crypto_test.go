package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if privateKey == nil {
		t.Fatal("Private key is nil")
	}

	if publicKey == nil {
		t.Fatal("Public key is nil")
	}

	if privateKey.N.BitLen() != 2048 {
		t.Errorf("Expected 2048-bit key, got %d-bit", privateKey.N.BitLen())
	}
}

func TestEncryptDecrypt(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	testData := []byte("Hello, World!")

	// Encrypt
	ciphertext, err := Encrypt(testData, publicKey)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, testData) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	plaintext, err := Decrypt(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, testData) {
		t.Errorf("Decrypted data doesn't match original. Got %s, expected %s", plaintext, testData)
	}
}

func TestEncryptDecryptChunked(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(2048)
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
			ciphertext, err := EncryptChunked(tt.data, publicKey)
			if err != nil {
				t.Fatalf("Failed to encrypt chunked data: %v", err)
			}

			// Decrypt
			plaintext, err := DecryptChunked(ciphertext, privateKey)
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
	privateKey, _, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temp file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")

	// Save
	err = SavePrivateKey(keyPath, privateKey)
	if err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	// Load
	loadedKey, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}

	// Verify they are the same
	if privateKey.N.Cmp(loadedKey.N) != 0 {
		t.Error("Loaded private key doesn't match original")
	}
}

func TestSaveLoadPublicKey(t *testing.T) {
	_, publicKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temp file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "public.pem")

	// Save
	err = SavePublicKey(keyPath, publicKey)
	if err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Load
	loadedKey, err := LoadPublicKey(keyPath)
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
		_, err := LoadPrivateKey(filepath.Join(tmpDir, "nonexistent.pem"))
		if err == nil {
			t.Error("Expected error loading non-existent file")
		}
	})

	t.Run("load non-existent public key", func(t *testing.T) {
		_, err := LoadPublicKey(filepath.Join(tmpDir, "nonexistent.pem"))
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

		_, err = LoadPrivateKey(invalidPath)
		if err == nil {
			t.Error("Expected error loading invalid PEM file")
		}

		_, err = LoadPublicKey(invalidPath)
		if err == nil {
			t.Error("Expected error loading invalid PEM file")
		}
	})
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Generate keys
	privateKey, publicKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save keys to temp files
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "private.pem")
	pubPath := filepath.Join(tmpDir, "public.pem")

	err = SavePrivateKey(privPath, privateKey)
	if err != nil {
		t.Fatalf("Failed to save private key: %v", err)
	}

	err = SavePublicKey(pubPath, publicKey)
	if err != nil {
		t.Fatalf("Failed to save public key: %v", err)
	}

	// Load keys from files
	loadedPriv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}

	loadedPub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	// Test encryption/decryption with loaded keys
	testData := []byte("Secret message for testing")

	ciphertext, err := EncryptChunked(testData, loadedPub)
	if err != nil {
		t.Fatalf("Failed to encrypt with loaded public key: %v", err)
	}

	plaintext, err := DecryptChunked(ciphertext, loadedPriv)
	if err != nil {
		t.Fatalf("Failed to decrypt with loaded private key: %v", err)
	}

	if !bytes.Equal(plaintext, testData) {
		t.Errorf("Round-trip encryption/decryption failed. Got %s, expected %s", plaintext, testData)
	}
}

func BenchmarkEncrypt(b *testing.B) {
	_, publicKey, _ := GenerateKeyPair(2048)
	data := []byte("Test data for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Encrypt(data, publicKey)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	privKey, publicKey, _ := GenerateKeyPair(2048)
	data := []byte("Test data for benchmarking")
	ciphertext, _ := Encrypt(data, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decrypt(ciphertext, privKey)
	}
}

func BenchmarkEncryptChunked(b *testing.B) {
	_, publicKey, _ := GenerateKeyPair(2048)
	data := bytes.Repeat([]byte("A"), 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncryptChunked(data, publicKey)
	}
}

func BenchmarkDecryptChunked(b *testing.B) {
	privKey, publicKey, _ := GenerateKeyPair(2048)
	data := bytes.Repeat([]byte("A"), 1000)
	ciphertext, _ := EncryptChunked(data, publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecryptChunked(ciphertext, privKey)
	}
}
