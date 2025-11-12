package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// PEM block types
const (
	PEMTypeRSAPrivateKey = "RSA PRIVATE KEY"
	PEMTypePublicKey     = "PUBLIC KEY"
)

// LoadPublicKeyFromFile loads an RSA public key from a PEM file
func LoadPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	return ParsePublicKeyPEM(keyData)
}

// LoadPrivateKeyFromFile loads an RSA private key from a PEM file
func LoadPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	return ParsePrivateKeyPEM(keyData)
}

// ParsePublicKeyPEM parses an RSA public key from PEM-encoded data
func ParsePublicKeyPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
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

// ParsePrivateKeyPEM parses an RSA private key from PEM-encoded data
func ParsePrivateKeyPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	// Try PKCS1 format first
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return priv, nil
	}

	// Try PKCS8 format as fallback
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key (tried PKCS1 and PKCS8): %w", err)
	}

	rsaPriv, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA private key")
	}

	return rsaPriv, nil
}

// SavePrivateKeyToFile saves a private key to a PEM file
func SavePrivateKeyToFile(path string, key *rsa.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("private key cannot be nil")
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  PEMTypeRSAPrivateKey,
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

// SavePublicKeyToFile saves a public key to a PEM file
func SavePublicKeyToFile(path string, key *rsa.PublicKey) error {
	if key == nil {
		return fmt.Errorf("public key cannot be nil")
	}

	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  PEMTypePublicKey,
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

// EncodePublicKeyPEM encodes an RSA public key to PEM format
func EncodePublicKeyPEM(key *rsa.PublicKey) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("public key cannot be nil")
	}

	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  PEMTypePublicKey,
		Bytes: keyBytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

// EncodePrivateKeyPEM encodes an RSA private key to PEM format
func EncodePrivateKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("private key cannot be nil")
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  PEMTypeRSAPrivateKey,
		Bytes: keyBytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}
