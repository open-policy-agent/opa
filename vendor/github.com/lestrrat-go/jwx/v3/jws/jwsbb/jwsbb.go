// Package jwsbb provides the building blocks (hence the name "bb") for JWS operations.
// It should be thought of as a low-level API, almost akin to internal packages
// that should not be used directly by users of the jwx package. However, these exist
// to provide a more efficient way to perform JWS operations without the overhead of
// the higher-level jws package to power-users who know what they are doing.
//
// This package is currently considered EXPERIMENTAL, and the API may change
// without notice. It is not recommended to use this package unless you are
// fully aware of the implications of using it.
//
// All bb packages in jwx follow the same design principles:
// 1. Does minimal checking of input parameters (for performance); callers need to ensure that the parameters are valid.
// 2. All exported functions are strongly typed (i.e. they do not take `any` types unless they absolutely have to).
// 3. Does not rely on other public jwx packages (they are standalone, except for internal packages).
package jwsbb

// Signer is a generic interface that defines the method for signing payloads.
// The type parameter K represents the key type (e.g., []byte for HMAC keys,
// *rsa.PrivateKey for RSA keys, *ecdsa.PrivateKey for ECDSA keys).
type Signer[K any] interface {
	Sign(key K, payload []byte) ([]byte, error)
}

// Verifier is a generic interface that defines the method for verifying signatures.
// The type parameter K represents the key type (e.g., []byte for HMAC keys,
// *rsa.PublicKey for RSA keys, *ecdsa.PublicKey for ECDSA keys).
type Verifier[K any] interface {
	Verify(key K, buf []byte, signature []byte) error
}
