package jwsbb

import (
	"fmt"
	"hash"

	"github.com/lestrrat-go/dsig"
)

// hmacHashToDsigAlgorithm maps HMAC hash function sizes to dsig algorithm constants
func hmacHashToDsigAlgorithm(hfunc func() hash.Hash) (string, error) {
	h := hfunc()
	switch h.Size() {
	case 32: // SHA256
		return dsig.HMACWithSHA256, nil
	case 48: // SHA384
		return dsig.HMACWithSHA384, nil
	case 64: // SHA512
		return dsig.HMACWithSHA512, nil
	default:
		return "", fmt.Errorf("unsupported HMAC hash function: size=%d", h.Size())
	}
}

// SignHMAC generates an HMAC signature for the given payload using the specified hash function and key.
// The raw parameter should be the pre-computed signing input (typically header.payload).
//
// This function is now a thin wrapper around dsig.SignHMAC. For new projects, you should
// consider using dsig instead of this function.
func SignHMAC(key, payload []byte, hfunc func() hash.Hash) ([]byte, error) {
	dsigAlg, err := hmacHashToDsigAlgorithm(hfunc)
	if err != nil {
		return nil, fmt.Errorf("jwsbb.SignHMAC: %w", err)
	}

	return dsig.Sign(key, dsigAlg, payload, nil)
}

// VerifyHMAC verifies an HMAC signature for the given payload.
// This function verifies the signature using the specified key and hash function.
// The payload parameter should be the pre-computed signing input (typically header.payload).
//
// This function is now a thin wrapper around dsig.VerifyHMAC. For new projects, you should
// consider using dsig instead of this function.
func VerifyHMAC(key, payload, signature []byte, hfunc func() hash.Hash) error {
	dsigAlg, err := hmacHashToDsigAlgorithm(hfunc)
	if err != nil {
		return fmt.Errorf("jwsbb.VerifyHMAC: %w", err)
	}

	return dsig.Verify(key, dsigAlg, payload, signature)
}
