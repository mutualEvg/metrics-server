package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
)

// Constants for RSA encryption
const (
	DefaultKeySize     = 2048
	MinimumKeySize     = 2048
	RecommendedKeySize = 4096
	RSAOAEPHashSize    = 32 // SHA-256 produces 32 bytes
	RSAOAEPOverhead    = 2*RSAOAEPHashSize + 2
	ChunkLengthSize    = 2 // bytes for chunk length prefix
)

// EncryptRSA encrypts data using RSA-OAEP with SHA256
func EncryptRSA(data []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("public key cannot be nil")
	}

	hash := sha256.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}
	return ciphertext, nil
}

// DecryptRSA decrypts data using RSA-OAEP with SHA256
func DecryptRSA(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}

	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}
	return plaintext, nil
}

// EncryptRSAChunked encrypts data in chunks to handle large messages
// RSA encryption has a size limit based on key size
func EncryptRSAChunked(data []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("public key cannot be nil")
	}

	// Calculate max chunk size: keySize - 2*hashSize - 2
	// For 2048-bit RSA key and SHA256: 2048/8 - 2*32 - 2 = 190 bytes
	maxChunkSize := calculateMaxChunkSize(publicKey)

	if len(data) <= maxChunkSize {
		// Data fits in single chunk
		return EncryptRSA(data, publicKey)
	}

	// Encrypt in chunks
	var result []byte
	for i := 0; i < len(data); i += maxChunkSize {
		end := i + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		encryptedChunk, err := EncryptRSA(chunk, publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt chunk at offset %d: %w", i, err)
		}

		// Append chunk size (2 bytes) followed by encrypted chunk
		chunkLen := uint16(len(encryptedChunk))
		result = append(result, byte(chunkLen>>8), byte(chunkLen&0xFF))
		result = append(result, encryptedChunk...)
	}

	return result, nil
}

// DecryptRSAChunked decrypts data that was encrypted in chunks
func DecryptRSAChunked(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}

	keySize := privateKey.Size()

	// If data size equals key size, it's a single chunk
	if len(ciphertext) == keySize {
		return DecryptRSA(ciphertext, privateKey)
	}

	// Decrypt multiple chunks
	var result []byte
	offset := 0

	for offset < len(ciphertext) {
		if offset+ChunkLengthSize > len(ciphertext) {
			return nil, fmt.Errorf("invalid chunked data: incomplete chunk length at offset %d", offset)
		}

		// Read chunk size
		chunkLen := int(ciphertext[offset])<<8 | int(ciphertext[offset+1])
		offset += ChunkLengthSize

		if offset+chunkLen > len(ciphertext) {
			return nil, fmt.Errorf("invalid chunked data: incomplete chunk at offset %d (expected %d bytes, have %d)",
				offset, chunkLen, len(ciphertext)-offset)
		}

		// Decrypt chunk
		chunk := ciphertext[offset : offset+chunkLen]
		decryptedChunk, err := DecryptRSA(chunk, privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt chunk at offset %d: %w", offset, err)
		}

		result = append(result, decryptedChunk...)
		offset += chunkLen
	}

	return result, nil
}

// calculateMaxChunkSize calculates the maximum chunk size for RSA-OAEP encryption
func calculateMaxChunkSize(publicKey *rsa.PublicKey) int {
	return publicKey.Size() - RSAOAEPOverhead
}

// GenerateKeyPair generates a new RSA key pair for testing
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	if bits < MinimumKeySize {
		return nil, nil, fmt.Errorf("key size must be at least %d bits", MinimumKeySize)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}
