package dsig

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"
)

func rsaGetSignerCryptoSignerKey(key any) (crypto.Signer, bool, error) {
	if !isValidRSAKey(key) {
		return nil, false, fmt.Errorf(`invalid key type %T for RSA algorithm`, key)
	}
	cs, isCryptoSigner := key.(crypto.Signer)
	if isCryptoSigner {
		return cs, true, nil
	}
	return nil, false, nil
}

// rsaPSSOptions returns the PSS options for RSA-PSS signatures with the specified hash.
// The salt length is set to equal the hash length as per RFC 7518.
func rsaPSSOptions(h crypto.Hash) rsa.PSSOptions {
	return rsa.PSSOptions{
		Hash:       h,
		SaltLength: rsa.PSSSaltLengthEqualsHash,
	}
}

// SignRSA generates an RSA signature for the given payload using the specified private key and options.
// The raw parameter should be the pre-computed signing input (typically header.payload).
// If pss is true, RSA-PSS is used; otherwise, PKCS#1 v1.5 is used.
//
// The rr parameter is an optional io.Reader that can be used to provide randomness for signing.
// If rr is nil, it defaults to rand.Reader.
func SignRSA(key *rsa.PrivateKey, payload []byte, h crypto.Hash, pss bool, rr io.Reader) ([]byte, error) {
	if !isValidRSAKey(key) {
		return nil, fmt.Errorf(`invalid key type %T for RSA algorithm`, key)
	}
	var opts crypto.SignerOpts = h
	if pss {
		rsaopts := rsaPSSOptions(h)
		opts = &rsaopts
	}
	return cryptosign(key, payload, h, opts, rr)
}

// VerifyRSA verifies an RSA signature for the given payload and header.
// This function constructs the signing input by encoding the header and payload according to JWS specification,
// then verifies the signature using the specified public key and hash algorithm.
// If pss is true, RSA-PSS verification is used; otherwise, PKCS#1 v1.5 verification is used.
func VerifyRSA(key *rsa.PublicKey, payload, signature []byte, h crypto.Hash, pss bool) error {
	if !isValidRSAKey(key) {
		return fmt.Errorf(`invalid key type %T for RSA algorithm`, key)
	}
	hasher := h.New()
	hasher.Write(payload)
	digest := hasher.Sum(nil)
	if pss {
		return rsa.VerifyPSS(key, h, digest, signature, &rsa.PSSOptions{Hash: h, SaltLength: rsa.PSSSaltLengthEqualsHash})
	}
	return rsa.VerifyPKCS1v15(key, h, digest, signature)
}
