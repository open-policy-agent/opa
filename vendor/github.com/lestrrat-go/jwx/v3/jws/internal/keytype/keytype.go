package keytype

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"

	"github.com/lestrrat-go/jwx/v3/jwk"
)

// Because the keys defined in github.com/lestrrat-go/jwx/jwk may also implement
// crypto.Signer, it would be possible for to mix up key types when signing/verifying
// for example, when we specify jws.WithKey(jwa.RSA256, cryptoSigner), the cryptoSigner
// can be for RSA, or any other type that implements crypto.Signer... even if it's for the
// wrong algorithm.
//
// These functions are there to differentiate between the valid KNOWN key types.
// For any other key type that is outside of the Go std library and our own code,
// we must rely on the user to be vigilant.
//
// Notes: symmetric keys are obviously not part of this. for v2 OKP keys,
// x25519 does not implement Sign()
func IsValidRSAKey(key any) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		ed25519.PrivateKey,
		jwk.ECDSAPrivateKey, jwk.OKPPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}

func IsValidECDSAKey(key any) bool {
	switch key.(type) {
	case
		ed25519.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey,
		jwk.RSAPrivateKey, jwk.OKPPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}

func IsValidEDDSAKey(key any) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey,
		jwk.RSAPrivateKey, jwk.ECDSAPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}
