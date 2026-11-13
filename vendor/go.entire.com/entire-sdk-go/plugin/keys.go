package plugin

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// ParsePrivateKey reads an Ed25519 private key from a PEM "PRIVATE KEY"
// block (PKCS #8), as `openssl genpkey -algorithm ed25519` writes it.
func ParsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("plugin: no PEM PRIVATE KEY block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("plugin: parse private key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("plugin: private key is %T, not Ed25519", parsed)
	}
	return key, nil
}

// PublicKeyPEM returns key as a PEM "PUBLIC KEY" block, the form Entire
// takes when a plugin registers public keys instead of a JWKS URL.
func PublicKeyPEM(key ed25519.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("plugin: encode public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
