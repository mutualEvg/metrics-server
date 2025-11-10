package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPublicKey loads an RSA public key from a PEM file
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return rsaPub, nil
}

// LoadPrivateKey loads an RSA private key from a PEM file
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format as fallback
		privKey, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key (tried PKCS1 and PKCS8): %w", err)
		}
		rsaPriv, ok := privKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not an RSA private key")
		}
		return rsaPriv, nil
	}

	return priv, nil
}

// Encrypt encrypts data using RSA-OAEP with SHA256
func Encrypt(data []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	hash := sha256.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts data using RSA-OAEP with SHA256
func Decrypt(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}
	return plaintext, nil
}

// EncryptChunked encrypts data in chunks to handle large messages
// RSA encryption has a size limit based on key size
func EncryptChunked(data []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	// Calculate max chunk size: keySize - 2*hashSize - 2
	// For 2048-bit RSA key and SHA256: 2048/8 - 2*32 - 2 = 190 bytes
	keySize := publicKey.Size()
	maxChunkSize := keySize - 2*sha256.Size - 2

	if len(data) <= maxChunkSize {
		// Data fits in single chunk
		return Encrypt(data, publicKey)
	}

	// Encrypt in chunks
	var result []byte
	for i := 0; i < len(data); i += maxChunkSize {
		end := i + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		encryptedChunk, err := Encrypt(chunk, publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt chunk: %w", err)
		}

		// Append chunk size (2 bytes) followed by encrypted chunk
		chunkLen := uint16(len(encryptedChunk))
		result = append(result, byte(chunkLen>>8), byte(chunkLen&0xFF))
		result = append(result, encryptedChunk...)
	}

	return result, nil
}

// DecryptChunked decrypts data that was encrypted in chunks
func DecryptChunked(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	keySize := privateKey.Size()

	// If data size equals key size, it's a single chunk
	if len(ciphertext) == keySize {
		return Decrypt(ciphertext, privateKey)
	}

	// Decrypt multiple chunks
	var result []byte
	offset := 0

	for offset < len(ciphertext) {
		if offset+2 > len(ciphertext) {
			return nil, fmt.Errorf("invalid chunked data: incomplete chunk length")
		}

		// Read chunk size
		chunkLen := int(ciphertext[offset])<<8 | int(ciphertext[offset+1])
		offset += 2

		if offset+chunkLen > len(ciphertext) {
			return nil, fmt.Errorf("invalid chunked data: incomplete chunk")
		}

		// Decrypt chunk
		chunk := ciphertext[offset : offset+chunkLen]
		decryptedChunk, err := Decrypt(chunk, privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt chunk: %w", err)
		}

		result = append(result, decryptedChunk...)
		offset += chunkLen
	}

	return result, nil
}

// GenerateKeyPair generates a new RSA key pair for testing
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}

// SavePrivateKey saves a private key to a PEM file
func SavePrivateKey(path string, key *rsa.PrivateKey) error {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer file.Close()

	if err := pem.Encode(file, pemBlock); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

// SavePublicKey saves a public key to a PEM file
func SavePublicKey(path string, key *rsa.PublicKey) error {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer file.Close()

	if err := pem.Encode(file, pemBlock); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}
