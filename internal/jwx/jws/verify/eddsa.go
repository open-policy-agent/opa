package verify

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

var eddsaVerifyFuncs = map[jwa.SignatureAlgorithm]eddsaVerifyFunc{}

func init() {
	algs := map[jwa.SignatureAlgorithm]struct {
		Hash       crypto.Hash
		VerifyFunc func(crypto.Hash) eddsaVerifyFunc
	}{
		jwa.EdDSA: {
			Hash:       crypto.Hash(0),
			VerifyFunc: makeEdDSAVerify,
		},
	}

	for alg, item := range algs {
		eddsaVerifyFuncs[alg] = item.VerifyFunc(item.Hash)
	}
}

func makeEdDSAVerify(hash crypto.Hash) eddsaVerifyFunc {
	return eddsaVerifyFunc(func(payload, signature []byte, key ed25519.PublicKey) error {
		if !ed25519.Verify(key, payload, signature) {
			return fmt.Errorf("")
		}

		return nil
	})
}

func newEdDSA(alg jwa.SignatureAlgorithm) (*EdDSAVerifier, error) {
	verifyfn, ok := eddsaVerifyFuncs[alg]
	if !ok {
		return nil, fmt.Errorf(`unsupported algorithm while trying to create EdDSA verifier: %s`, alg)
	}

	return &EdDSAVerifier{
		verify: verifyfn,
	}, nil
}

// Verify checks if a JWS is valid.
func (v EdDSAVerifier) Verify(payload, signature []byte, key interface{}) error {
	if key == nil {
		return errors.New(`missing public key while verifying payload`)
	}
	eddsaKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf(`invalid key type %T. ed25519.PublicKey is required`, key)
	}

	return v.verify(payload, signature, eddsaKey)
}
