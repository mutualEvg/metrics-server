package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/mutualEvg/metrics-server/internal/crypto"
)

func main() {
	bits := flag.Int("bits", 2048, "RSA key size in bits")
	privPath := flag.String("priv", "private.pem", "Path for private key output")
	pubPath := flag.String("pub", "public.pem", "Path for public key output")
	flag.Parse()

	fmt.Printf("Generating %d-bit RSA key pair...\n", *bits)

	// Generate key pair
	privateKey, publicKey, err := crypto.GenerateKeyPair(*bits)
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}

	// Save private key
	if err := crypto.SavePrivateKey(*privPath, privateKey); err != nil {
		log.Fatalf("Failed to save private key: %v", err)
	}
	fmt.Printf("Private key saved to: %s\n", *privPath)

	// Save public key
	if err := crypto.SavePublicKey(*pubPath, publicKey); err != nil {
		log.Fatalf("Failed to save public key: %v", err)
	}
	fmt.Printf("Public key saved to: %s\n", *pubPath)

	fmt.Println("\nKey pair generated successfully!")
	fmt.Printf("\nUsage:\n")
	fmt.Printf("  Server: ./server -crypto-key=%s\n", *privPath)
	fmt.Printf("  Agent:  ./agent -crypto-key=%s\n", *pubPath)
}
