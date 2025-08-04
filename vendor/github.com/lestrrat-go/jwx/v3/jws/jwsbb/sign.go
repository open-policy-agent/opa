package jwsbb

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
)

// Sign generates a JWS signature using the specified key and algorithm.
//
// This function loads the signer registered in the hwsbb package _ONLY_.
// It does not support custom signers that the user might have registered.
//
// rr is an io.Reader that provides randomness for signing. If rr is nil, it defaults to rand.Reader.
// Not all algorithms require this parameter, but it is included for consistency.
// 99% of the time, you can pass nil for rr, and it will work fine.
func Sign(key any, alg string, payload []byte, rr io.Reader) ([]byte, error) {
	switch {
	case isSupportedHMACAlgorithm(alg):
		return dispatchHMACSign(key, alg, payload)
	case isSuppotedRSAAlgorithm(alg):
		return dispatchRSASign(key, alg, payload, rr)
	case isSuppotedECDSAAlgorithm(alg):
		return dispatchECDSASign(key, alg, payload, rr)
	case isSupportedEdDSAAlgorithm(alg):
		return dispatchEdDSASign(key, alg, payload, rr)
	}

	return nil, fmt.Errorf(`jwsbb.Sign: unsupported signature algorithm %q`, alg)
}

func dispatchHMACSign(key any, alg string, payload []byte) ([]byte, error) {
	h, err := HMACHashFuncFor(alg)
	if err != nil {
		return nil, fmt.Errorf(`jwsbb.Sign: failed to get hash function for %s: %w`, alg, err)
	}

	var hmackey []byte
	if err := toHMACKey(&hmackey, key); err != nil {
		return nil, fmt.Errorf(`jwsbb.Sign: %w`, err)
	}
	return SignHMAC(hmackey, payload, h)
}

func dispatchRSASign(key any, alg string, payload []byte, rr io.Reader) ([]byte, error) {
	h, pss, err := RSAHashFuncFor(alg)
	if err != nil {
		return nil, fmt.Errorf(`jwsbb.Sign: failed to get hash function for %s: %w`, alg, err)
	}
	cs, isCryptoSigner, err := rsaGetSignerCryptoSignerKey(key)
	if err != nil {
		return nil, fmt.Errorf(`jwsbb.Sign: %w`, err)
	}
	if isCryptoSigner {
		var options crypto.SignerOpts = h
		if pss {
			rsaopts := RSAPSSOptions(h)
			options = &rsaopts
		}
		return SignCryptoSigner(cs, payload, h, options, rr)
	}

	var privkey *rsa.PrivateKey
	if err := keyconv.RSAPrivateKey(&privkey, key); err != nil {
		return nil, fmt.Errorf(`jws.RSASigner: invalid key type %T. rsa.PrivateKey is required: %w`, key, err)
	}
	return SignRSA(privkey, payload, h, pss, rr)
}

func dispatchEdDSASign(key any, _ string, payload []byte, rr io.Reader) ([]byte, error) {
	signer, err := eddsaGetSigner(key)
	if err != nil {
		return nil, fmt.Errorf(`jws.EdDSASigner: %w`, err)
	}

	return SignCryptoSigner(signer, payload, crypto.Hash(0), crypto.Hash(0), rr)
}

func dispatchECDSASign(key any, alg string, payload []byte, rr io.Reader) ([]byte, error) {
	h, err := ECDSAHashFuncFor(alg)
	if err != nil {
		return nil, fmt.Errorf(`jwsbb.Sign: failed to get hash function for %s: %w`, alg, err)
	}
	privkey, cs, isCryptoSigner, err := ecdsaGetSignerKey(key)
	if err != nil {
		return nil, fmt.Errorf(`jws.ECDSASigner: %w`, err)
	}
	if isCryptoSigner {
		return SignECDSACryptoSigner(cs, payload, h, rr)
	}
	return SignECDSA(privkey, payload, h, rr)
}
