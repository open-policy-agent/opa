package jwsbb

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"

	"github.com/lestrrat-go/jwx/v3/jws/internal/keytype"
)

func rsaGetSignerCryptoSignerKey(key any) (crypto.Signer, bool, error) {
	cs, isCryptoSigner := key.(crypto.Signer)
	if isCryptoSigner {
		if !keytype.IsValidRSAKey(key) {
			return nil, false, fmt.Errorf(`cannot use key of type %T`, key)
		}
		return cs, true, nil
	}
	return nil, false, nil
}

var rsaHashFuncs = map[string]struct {
	Hash crypto.Hash
	PSS  bool // whether to use PSS padding
}{
	"RS256": {Hash: crypto.SHA256, PSS: false},
	"RS384": {Hash: crypto.SHA384, PSS: false},
	"RS512": {Hash: crypto.SHA512, PSS: false},
	"PS256": {Hash: crypto.SHA256, PSS: true},
	"PS384": {Hash: crypto.SHA384, PSS: true},
	"PS512": {Hash: crypto.SHA512, PSS: true},
}

func isSuppotedRSAAlgorithm(alg string) bool {
	_, ok := rsaHashFuncs[alg]
	return ok
}

// RSAHashFuncFor returns the appropriate hash function and PSS flag for the given RSA algorithm.
// Supported algorithms: RS256, RS384, RS512 (PKCS#1 v1.5) and PS256, PS384, PS512 (PSS).
// Returns the hash function, PSS flag, and an error if the algorithm is unsupported.
func RSAHashFuncFor(alg string) (crypto.Hash, bool, error) {
	if h, ok := rsaHashFuncs[alg]; ok {
		return h.Hash, h.PSS, nil
	}
	return 0, false, fmt.Errorf("unsupported RSA algorithm %s", alg)
}

// RSAPSSOptions returns the PSS options for RSA-PSS signatures with the specified hash.
// The salt length is set to equal the hash length as per RFC 7518.
func RSAPSSOptions(h crypto.Hash) rsa.PSSOptions {
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
	var opts crypto.SignerOpts = h
	if pss {
		rsaopts := RSAPSSOptions(h)
		opts = &rsaopts
	}
	return cryptosign(key, payload, h, opts, rr)
}

// VerifyRSA verifies an RSA signature for the given payload and header.
// This function constructs the signing input by encoding the header and payload according to JWS specification,
// then verifies the signature using the specified public key and hash algorithm.
// If pss is true, RSA-PSS verification is used; otherwise, PKCS#1 v1.5 verification is used.
func VerifyRSA(key *rsa.PublicKey, payload, signature []byte, h crypto.Hash, pss bool) error {
	hasher := h.New()
	hasher.Write(payload)
	digest := hasher.Sum(nil)
	if pss {
		return rsa.VerifyPSS(key, h, digest, signature, &rsa.PSSOptions{Hash: h, SaltLength: rsa.PSSSaltLengthEqualsHash})
	}
	return rsa.VerifyPKCS1v15(key, h, digest, signature)
}
