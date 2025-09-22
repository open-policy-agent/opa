package jwsbb

import (
	"crypto/ed25519"

	"github.com/lestrrat-go/dsig"
)

// SignEdDSA generates an EdDSA (Ed25519) signature for the given payload.
// The raw parameter should be the pre-computed signing input (typically header.payload).
// EdDSA is deterministic and doesn't require additional hashing of the input.
//
// This function is now a thin wrapper around dsig.SignEdDSA. For new projects, you should
// consider using dsig instead of this function.
func SignEdDSA(key ed25519.PrivateKey, payload []byte) ([]byte, error) {
	// Use dsig.Sign with EdDSA algorithm constant
	return dsig.Sign(key, dsig.EdDSA, payload, nil)
}

// VerifyEdDSA verifies an EdDSA (Ed25519) signature for the given payload.
// This function verifies the signature using Ed25519 verification algorithm.
// The payload parameter should be the pre-computed signing input (typically header.payload).
// EdDSA is deterministic and provides strong security guarantees without requiring hash function selection.
//
// This function is now a thin wrapper around dsig.VerifyEdDSA. For new projects, you should
// consider using dsig instead of this function.
func VerifyEdDSA(key ed25519.PublicKey, payload, signature []byte) error {
	// Use dsig.Verify with EdDSA algorithm constant
	return dsig.Verify(key, dsig.EdDSA, payload, signature)
}
