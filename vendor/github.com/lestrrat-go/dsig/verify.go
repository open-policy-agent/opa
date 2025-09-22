package dsig

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
)

// Verify verifies a digital signature using the specified key and algorithm.
//
// This function loads the verifier registered in the dsig package _ONLY_.
// It does not support custom verifiers that the user might have registered.
func Verify(key any, alg string, payload, signature []byte) error {
	info, ok := GetAlgorithmInfo(alg)
	if !ok {
		return fmt.Errorf(`dsig.Verify: unsupported signature algorithm %q`, alg)
	}

	switch info.Family {
	case HMAC:
		return dispatchHMACVerify(key, info, payload, signature)
	case RSA:
		return dispatchRSAVerify(key, info, payload, signature)
	case ECDSA:
		return dispatchECDSAVerify(key, info, payload, signature)
	case EdDSAFamily:
		return dispatchEdDSAVerify(key, info, payload, signature)
	default:
		return fmt.Errorf(`dsig.Verify: unsupported signature family %q`, info.Family)
	}
}

func dispatchHMACVerify(key any, info AlgorithmInfo, payload, signature []byte) error {
	meta, ok := info.Meta.(HMACFamilyMeta)
	if !ok {
		return fmt.Errorf(`dsig.Verify: invalid HMAC metadata`)
	}

	var hmackey []byte
	if err := toHMACKey(&hmackey, key); err != nil {
		return fmt.Errorf(`dsig.Verify: %w`, err)
	}
	return VerifyHMAC(hmackey, payload, signature, meta.HashFunc)
}

func dispatchRSAVerify(key any, info AlgorithmInfo, payload, signature []byte) error {
	meta, ok := info.Meta.(RSAFamilyMeta)
	if !ok {
		return fmt.Errorf(`dsig.Verify: invalid RSA metadata`)
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
			return fmt.Errorf(`dsig.Verify: failed to retrieve rsa.PublicKey out of crypto.Signer %T`, key)
		}
	} else {
		var ok bool
		pubkey, ok = key.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf(`dsig.Verify: failed to retrieve *rsa.PublicKey out of %T`, key)
		}
	}

	return VerifyRSA(pubkey, payload, signature, meta.Hash, meta.PSS)
}

func dispatchECDSAVerify(key any, info AlgorithmInfo, payload, signature []byte) error {
	meta, ok := info.Meta.(ECDSAFamilyMeta)
	if !ok {
		return fmt.Errorf(`dsig.Verify: invalid ECDSA metadata`)
	}

	pubkey, cs, isCryptoSigner, err := ecdsaGetVerifierKey(key)
	if err != nil {
		return fmt.Errorf(`dsig.Verify: %w`, err)
	}
	if isCryptoSigner {
		return VerifyECDSACryptoSigner(cs, payload, signature, meta.Hash)
	}
	return VerifyECDSA(pubkey, payload, signature, meta.Hash)
}

func dispatchEdDSAVerify(key any, _ AlgorithmInfo, payload, signature []byte) error {
	var pubkey ed25519.PublicKey
	signer, ok := key.(crypto.Signer)
	if ok {
		v := signer.Public()
		pubkey, ok = v.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`dsig.Verify: expected crypto.Signer.Public() to return ed25519.PublicKey, but got %T`, v)
		}
	} else {
		var ok bool
		pubkey, ok = key.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`dsig.Verify: failed to retrieve ed25519.PublicKey out of %T`, key)
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

	pubkey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, false, fmt.Errorf(`invalid key type %T. *ecdsa.PublicKey is required`, key)
	}

	return pubkey, nil, false, nil
}
