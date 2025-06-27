//go:generate ../tools/cmd/genjwa.sh

// Package jwa defines the various algorithm described in https://tools.ietf.org/html/rfc7518
package jwa

import (
	"errors"
	"fmt"
)

// KeyAlgorithm is a workaround for jwk.Key being able to contain different
// types of algorithms in its `alg` field.
//
// Previously the storage for the `alg` field was represented as a string,
// but this caused some users to wonder why the field was not typed appropriately
// like other fields.
//
// Ideally we would like to keep track of Signature Algorithms and
// Key Encryption Algorithms separately, and force the APIs to
// type-check at compile time, but this allows users to pass a value from a
// jwk.Key directly
type KeyAlgorithm interface {
	String() string
	IsDeprecated() bool
}

var errInvalidKeyAlgorithm = errors.New(`invalid key algorithm`)

func ErrInvalidKeyAlgorithm() error {
	return errInvalidKeyAlgorithm
}

// KeyAlgorithmFrom takes either a string, `jwa.SignatureAlgorithm`,
// `jwa.KeyEncryptionAlgorithm`, or `jwa.ContentEncryptionAlgorithm`.
// and returns a `jwa.KeyAlgorithm`.
//
// If the value cannot be handled, it returns an `jwa.InvalidKeyAlgorithm`
// object instead of returning an error. This design choice was made to allow
// users to directly pass the return value to functions such as `jws.Sign()`
func KeyAlgorithmFrom(v any) (KeyAlgorithm, error) {
	switch v := v.(type) {
	case SignatureAlgorithm:
		return v, nil
	case KeyEncryptionAlgorithm:
		return v, nil
	case ContentEncryptionAlgorithm:
		return v, nil
	case string:
		salg, ok := LookupSignatureAlgorithm(v)
		if ok {
			return salg, nil
		}

		kalg, ok := LookupKeyEncryptionAlgorithm(v)
		if ok {
			return kalg, nil
		}

		calg, ok := LookupContentEncryptionAlgorithm(v)
		if ok {
			return calg, nil
		}

		return nil, fmt.Errorf(`invalid key value: %q: %w`, v, errInvalidKeyAlgorithm)
	default:
		return nil, fmt.Errorf(`invalid key type: %T: %w`, v, errInvalidKeyAlgorithm)
	}
}
