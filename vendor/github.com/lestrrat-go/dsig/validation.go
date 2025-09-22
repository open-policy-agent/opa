package dsig

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
)

// isValidRSAKey validates that the provided key type is appropriate for RSA algorithms.
// It returns false if the key is clearly incompatible (e.g., ECDSA or EdDSA keys).
func isValidRSAKey(key any) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		ed25519.PrivateKey:
		// these are NOT ok for RSA algorithms
		return false
	}
	return true
}

// isValidECDSAKey validates that the provided key type is appropriate for ECDSA algorithms.
// It returns false if the key is clearly incompatible (e.g., RSA or EdDSA keys).
func isValidECDSAKey(key any) bool {
	switch key.(type) {
	case
		ed25519.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey:
		// these are NOT ok for ECDSA algorithms
		return false
	}
	return true
}

// isValidEDDSAKey validates that the provided key type is appropriate for EdDSA algorithms.
// It returns false if the key is clearly incompatible (e.g., RSA or ECDSA keys).
func isValidEDDSAKey(key any) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey:
		// these are NOT ok for EdDSA algorithms
		return false
	}
	return true
}

// VerificationError represents an error that occurred during signature verification.
type VerificationError struct {
	message string
}

func (e *VerificationError) Error() string {
	return e.message
}

// NewVerificationError creates a new verification error with the given message.
func NewVerificationError(message string) error {
	return &VerificationError{message: message}
}

// IsVerificationError checks if the given error is a verification error.
func IsVerificationError(err error) bool {
	_, ok := err.(*VerificationError)
	return ok
}
