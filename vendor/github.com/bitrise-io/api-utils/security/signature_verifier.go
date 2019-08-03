package security

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"hash"
	"strings"
)

// SignatureVerifier ...
type SignatureVerifier struct {
	secretToken      string
	payloadBody      string
	payloadSignature string
}

// NewSignatureVerifier ...
func NewSignatureVerifier(secretToken, payloadBody, payloadSignature string) SignatureVerifier {
	return SignatureVerifier{
		secretToken:      secretToken,
		payloadBody:      payloadBody,
		payloadSignature: payloadSignature,
	}
}

func (s *SignatureVerifier) hashMethod() string {
	return strings.Split(s.payloadSignature, "=")[0]
}

func (s *SignatureVerifier) encryptAlgorithm() func() hash.Hash {
	switch s.hashMethod() {
	case "sha1":
		return sha1.New
	case "sha256":
		return sha256.New
	}
	return sha1.New
}

func (s *SignatureVerifier) generatePayloadSignature() string {
	mac := hmac.New(s.encryptAlgorithm(), []byte(s.secretToken))
	mac.Write([]byte(s.payloadBody))
	expectedMAC := mac.Sum(nil)
	return s.hashMethod() + "=" + hex.EncodeToString(expectedMAC)
}

// Verify ...
func (s *SignatureVerifier) Verify() bool {
	generatedSignature := s.generatePayloadSignature()
	return subtle.ConstantTimeCompare([]byte(generatedSignature), []byte(s.payloadSignature)) == 1
}
