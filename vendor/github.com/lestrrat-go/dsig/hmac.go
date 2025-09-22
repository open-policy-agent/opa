package dsig

import (
	"crypto/hmac"
	"fmt"
	"hash"
)

func toHMACKey(dst *[]byte, key any) error {
	keyBytes, ok := key.([]byte)
	if !ok {
		return fmt.Errorf(`dsig.toHMACKey: invalid key type %T. []byte is required`, key)
	}

	if len(keyBytes) == 0 {
		return fmt.Errorf(`dsig.toHMACKey: missing key while signing payload`)
	}

	*dst = keyBytes
	return nil
}

// SignHMAC generates an HMAC signature for the given payload using the specified hash function and key.
// The raw parameter should be the pre-computed signing input (typically header.payload).
func SignHMAC(key, payload []byte, hfunc func() hash.Hash) ([]byte, error) {
	h := hmac.New(hfunc, key)
	if _, err := h.Write(payload); err != nil {
		return nil, fmt.Errorf(`failed to write payload using hmac: %w`, err)
	}
	return h.Sum(nil), nil
}

// VerifyHMAC verifies an HMAC signature for the given payload.
// This function verifies the signature using the specified key and hash function.
// The payload parameter should be the pre-computed signing input (typically header.payload).
func VerifyHMAC(key, payload, signature []byte, hfunc func() hash.Hash) error {
	expected, err := SignHMAC(key, payload, hfunc)
	if err != nil {
		return fmt.Errorf("failed to sign payload for verification: %w", err)
	}
	if !hmac.Equal(signature, expected) {
		return NewVerificationError("invalid HMAC signature")
	}
	return nil
}
