// Package legacy provides support for legacy implementation of JWS signing and verification.
// Types, functions, and variables in this package are exported only for legacy support,
// and should not be relied upon for new code.
//
// This package will be available until v3 is sunset, but it will be removed in v4
package legacy

import (
	"github.com/lestrrat-go/jwx/v3/jwa"
)

// Signer generates the signature for a given payload.
// This is for legacy support only.
type Signer interface {
	// Sign creates a signature for the given payload.
	// The second argument is the key used for signing the payload, and is usually
	// the private key type associated with the signature method. For example,
	// for `jwa.RSXXX` and `jwa.PSXXX` types, you need to pass the
	// `*"crypto/rsa".PrivateKey` type.
	// Check the documentation for each signer for details
	Sign([]byte, any) ([]byte, error)

	Algorithm() jwa.SignatureAlgorithm
}

// This is for legacy support only.
type Verifier interface {
	// Verify checks whether the payload and signature are valid for
	// the given key.
	// `key` is the key used for verifying the payload, and is usually
	// the public key associated with the signature method. For example,
	// for `jwa.RSXXX` and `jwa.PSXXX` types, you need to pass the
	// `*"crypto/rsa".PublicKey` type.
	// Check the documentation for each verifier for details
	Verify(payload []byte, signature []byte, key any) error
}
