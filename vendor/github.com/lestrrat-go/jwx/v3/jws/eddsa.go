package jws

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

type eddsaSigner struct{}

func newEdDSASigner() Signer {
	return &eddsaSigner{}
}

func (s eddsaSigner) Algorithm() jwa.SignatureAlgorithm {
	return jwa.EdDSA()
}

func (s eddsaSigner) Sign(payload []byte, key interface{}) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf(`missing private key while signing payload`)
	}

	// The ed25519.PrivateKey object implements crypto.Signer, so we should
	// simply accept a crypto.Signer here.
	signer, ok := key.(crypto.Signer)
	if ok {
		if !isValidEDDSAKey(key) {
			return nil, fmt.Errorf(`cannot use key of type %T to generate EdDSA based signatures`, key)
		}
	} else {
		// This fallback exists for cases when jwk.Key was passed, or
		// users gave us a pointer instead of non-pointer, etc.
		var privkey ed25519.PrivateKey
		if err := keyconv.Ed25519PrivateKey(&privkey, key); err != nil {
			return nil, fmt.Errorf(`failed to retrieve ed25519.PrivateKey out of %T: %w`, key, err)
		}
		signer = privkey
	}
	return signer.Sign(rand.Reader, payload, crypto.Hash(0))
}

type eddsaVerifier struct{}

func newEdDSAVerifier() Verifier {
	return &eddsaVerifier{}
}

func (v eddsaVerifier) Verify(payload, signature []byte, key interface{}) (err error) {
	if key == nil {
		return fmt.Errorf(`missing public key while verifying payload`)
	}

	var pubkey ed25519.PublicKey
	signer, ok := key.(crypto.Signer)
	if ok {
		v := signer.Public()
		pubkey, ok = v.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`expected crypto.Signer.Public() to return ed25519.PublicKey, but got %T`, v)
		}
	} else {
		if err := keyconv.Ed25519PublicKey(&pubkey, key); err != nil {
			return fmt.Errorf(`failed to retrieve ed25519.PublicKey out of %T: %w`, key, err)
		}
	}

	if !ed25519.Verify(pubkey, payload, signature) {
		return fmt.Errorf(`failed to match EdDSA signature`)
	}

	return nil
}
