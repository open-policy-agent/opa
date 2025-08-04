package jwsbb

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
)

// Verify verifies a JWS signature using the specified key and algorithm.
//
// This function loads the verifier registered in the jwsbb package _ONLY_.
// It does not support custom verifiers that the user might have registered.
func Verify(key any, alg string, payload, signature []byte) error {
	switch {
	case isSupportedHMACAlgorithm(alg):
		return dispatchHMACVerify(key, alg, payload, signature)
	case isSuppotedRSAAlgorithm(alg):
		return dispatchRSAVerify(key, alg, payload, signature)
	case isSuppotedECDSAAlgorithm(alg):
		return dispatchECDSAVerify(key, alg, payload, signature)
	case isSupportedEdDSAAlgorithm(alg):
		return dispatchEdDSAVerify(key, alg, payload, signature)
	}

	return fmt.Errorf(`jwsbb.Verify: unsupported signature algorithm %q`, alg)
}

func dispatchHMACVerify(key any, alg string, payload, signature []byte) error {
	h, err := HMACHashFuncFor(alg)
	if err != nil {
		return fmt.Errorf(`jwsbb.Verify: failed to get hash function for %s: %w`, alg, err)
	}

	var hmackey []byte
	if err := toHMACKey(&hmackey, key); err != nil {
		return fmt.Errorf(`jwsbb.Verify: %w`, err)
	}
	return VerifyHMAC(hmackey, payload, signature, h)
}

func dispatchRSAVerify(key any, alg string, payload, signature []byte) error {
	h, pss, err := RSAHashFuncFor(alg)
	if err != nil {
		return fmt.Errorf(`jwsbb.Verify: failed to get hash function for %s: %w`, alg, err)
	}

	var pubkey *rsa.PublicKey

	if cs, ok := key.(crypto.Signer); ok {
		cpub := cs.Public()
		switch cpub := cpub.(type) {
		case rsa.PublicKey:
			pubkey = &cpub
		case *rsa.PublicKey:
			pubkey = cpub
		default:
			return fmt.Errorf(`jwsbb.Verify: failed to retrieve rsa.PublicKey out of crypto.Signer %T`, key)
		}
	} else {
		if err := keyconv.RSAPublicKey(&pubkey, key); err != nil {
			return fmt.Errorf(`jwsbb.Verify: failed to retrieve rsa.PublicKey out of %T: %w`, key, err)
		}
	}

	return VerifyRSA(pubkey, payload, signature, h, pss)
}

func dispatchECDSAVerify(key any, alg string, payload, signature []byte) error {
	h, err := ECDSAHashFuncFor(alg)
	if err != nil {
		return fmt.Errorf(`jwsbb.Verify: failed to get hash function for %s: %w`, alg, err)
	}

	pubkey, cs, isCryptoSigner, err := ecdsaGetVerifierKey(key)
	if err != nil {
		return fmt.Errorf(`jwsbb.Verify: %w`, err)
	}
	if isCryptoSigner {
		return VerifyECDSACryptoSigner(cs, payload, signature, h)
	}
	return VerifyECDSA(pubkey, payload, signature, h)
}

func dispatchEdDSAVerify(key any, _ string, payload, signature []byte) error {
	var pubkey ed25519.PublicKey
	signer, ok := key.(crypto.Signer)
	if ok {
		v := signer.Public()
		pubkey, ok = v.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`jwsbb.Verify: expected crypto.Signer.Public() to return ed25519.PublicKey, but got %T`, v)
		}
	} else {
		if err := keyconv.Ed25519PublicKey(&pubkey, key); err != nil {
			return fmt.Errorf(`jwsbb.Verify: failed to retrieve ed25519.PublicKey out of %T: %w`, key, err)
		}
	}

	return VerifyEdDSA(pubkey, payload, signature)
}

func ecdsaGetVerifierKey(key any) (*ecdsa.PublicKey, crypto.Signer, bool, error) {
	cs, isCryptoSigner := key.(crypto.Signer)
	if isCryptoSigner {
		switch key.(type) {
		case ecdsa.PublicKey, *ecdsa.PublicKey:
			// if it's ecdsa.PublicKey, it's more efficient to
			// go through the non-crypto.Signer route. Set isCryptoSigner to false
			isCryptoSigner = false
		}
	}

	if isCryptoSigner {
		return nil, cs, true, nil
	}

	var pubkey *ecdsa.PublicKey
	if err := keyconv.ECDSAPublicKey(&pubkey, key); err != nil {
		return nil, nil, false, fmt.Errorf(`invalid key type %T. ecdsa.PublicKey is required: %w`, key, err)
	}

	return pubkey, nil, false, nil
}
