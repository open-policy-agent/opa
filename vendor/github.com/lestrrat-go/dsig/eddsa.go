package dsig

import (
	"crypto"
	"crypto/ed25519"
	"fmt"
)

func eddsaGetSigner(key any) (crypto.Signer, error) {
	// The ed25519.PrivateKey object implements crypto.Signer, so we should
	// simply accept a crypto.Signer here.
	signer, ok := key.(crypto.Signer)
	if ok {
		if !isValidEDDSAKey(key) {
			return nil, fmt.Errorf(`invalid key type %T for EdDSA algorithm`, key)
		}
		return signer, nil
	}

	// This fallback exists for cases when users give us a pointer instead of non-pointer, etc.
	privkey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf(`failed to retrieve ed25519.PrivateKey out of %T`, key)
	}
	return privkey, nil
}

// SignEdDSA generates an EdDSA (Ed25519) signature for the given payload.
// The raw parameter should be the pre-computed signing input (typically header.payload).
// EdDSA is deterministic and doesn't require additional hashing of the input.
func SignEdDSA(key ed25519.PrivateKey, payload []byte) ([]byte, error) {
	return ed25519.Sign(key, payload), nil
}

// VerifyEdDSA verifies an EdDSA (Ed25519) signature for the given payload.
// This function verifies the signature using Ed25519 verification algorithm.
// The payload parameter should be the pre-computed signing input (typically header.payload).
// EdDSA is deterministic and provides strong security guarantees without requiring hash function selection.
func VerifyEdDSA(key ed25519.PublicKey, payload, signature []byte) error {
	if !ed25519.Verify(key, payload, signature) {
		return fmt.Errorf("invalid EdDSA signature")
	}
	return nil
}
