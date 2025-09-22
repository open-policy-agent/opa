package dsig

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"
)

// Sign generates a digital signature using the specified key and algorithm.
//
// This function loads the signer registered in the dsig package _ONLY_.
// It does not support custom signers that the user might have registered.
//
// rr is an io.Reader that provides randomness for signing. If rr is nil, it defaults to rand.Reader.
// Not all algorithms require this parameter, but it is included for consistency.
// 99% of the time, you can pass nil for rr, and it will work fine.
func Sign(key any, alg string, payload []byte, rr io.Reader) ([]byte, error) {
	info, ok := GetAlgorithmInfo(alg)
	if !ok {
		return nil, fmt.Errorf(`dsig.Sign: unsupported signature algorithm %q`, alg)
	}

	switch info.Family {
	case HMAC:
		return dispatchHMACSign(key, info, payload)
	case RSA:
		return dispatchRSASign(key, info, payload, rr)
	case ECDSA:
		return dispatchECDSASign(key, info, payload, rr)
	case EdDSAFamily:
		return dispatchEdDSASign(key, info, payload, rr)
	default:
		return nil, fmt.Errorf(`dsig.Sign: unsupported signature family %q`, info.Family)
	}
}

func dispatchHMACSign(key any, info AlgorithmInfo, payload []byte) ([]byte, error) {
	meta, ok := info.Meta.(HMACFamilyMeta)
	if !ok {
		return nil, fmt.Errorf(`dsig.Sign: invalid HMAC metadata`)
	}

	var hmackey []byte
	if err := toHMACKey(&hmackey, key); err != nil {
		return nil, fmt.Errorf(`dsig.Sign: %w`, err)
	}
	return SignHMAC(hmackey, payload, meta.HashFunc)
}

func dispatchRSASign(key any, info AlgorithmInfo, payload []byte, rr io.Reader) ([]byte, error) {
	meta, ok := info.Meta.(RSAFamilyMeta)
	if !ok {
		return nil, fmt.Errorf(`dsig.Sign: invalid RSA metadata`)
	}

	cs, isCryptoSigner, err := rsaGetSignerCryptoSignerKey(key)
	if err != nil {
		return nil, fmt.Errorf(`dsig.Sign: %w`, err)
	}
	if isCryptoSigner {
		var options crypto.SignerOpts = meta.Hash
		if meta.PSS {
			rsaopts := rsaPSSOptions(meta.Hash)
			options = &rsaopts
		}
		return SignCryptoSigner(cs, payload, meta.Hash, options, rr)
	}

	privkey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf(`dsig.Sign: invalid key type %T. *rsa.PrivateKey is required`, key)
	}
	return SignRSA(privkey, payload, meta.Hash, meta.PSS, rr)
}

func dispatchEdDSASign(key any, _ AlgorithmInfo, payload []byte, rr io.Reader) ([]byte, error) {
	signer, err := eddsaGetSigner(key)
	if err != nil {
		return nil, fmt.Errorf(`dsig.Sign: %w`, err)
	}

	return SignCryptoSigner(signer, payload, crypto.Hash(0), crypto.Hash(0), rr)
}

func dispatchECDSASign(key any, info AlgorithmInfo, payload []byte, rr io.Reader) ([]byte, error) {
	meta, ok := info.Meta.(ECDSAFamilyMeta)
	if !ok {
		return nil, fmt.Errorf(`dsig.Sign: invalid ECDSA metadata`)
	}

	privkey, cs, isCryptoSigner, err := ecdsaGetSignerKey(key)
	if err != nil {
		return nil, fmt.Errorf(`dsig.Sign: %w`, err)
	}
	if isCryptoSigner {
		return SignECDSACryptoSigner(cs, payload, meta.Hash, rr)
	}
	return SignECDSA(privkey, payload, meta.Hash, rr)
}
