package dsig

import (
	"crypto"
	"crypto/rand"
	"fmt"
	"io"
)

// cryptosign is a low-level function that signs a payload using a crypto.Signer.
// If hash is crypto.Hash(0), the payload is signed directly without hashing.
// Otherwise, the payload is hashed using the specified hash function before signing.
//
// rr is an io.Reader that provides randomness for signing. If rr is nil, it defaults to rand.Reader.
func cryptosign(signer crypto.Signer, payload []byte, hash crypto.Hash, opts crypto.SignerOpts, rr io.Reader) ([]byte, error) {
	if rr == nil {
		rr = rand.Reader
	}

	var digest []byte
	if hash == crypto.Hash(0) {
		digest = payload
	} else {
		h := hash.New()
		if _, err := h.Write(payload); err != nil {
			return nil, fmt.Errorf(`failed to write payload to hash: %w`, err)
		}
		digest = h.Sum(nil)
	}
	return signer.Sign(rr, digest, opts)
}

// SignCryptoSigner generates a signature using a crypto.Signer interface.
// This function can be used for hardware security modules, smart cards,
// and other implementations of the crypto.Signer interface.
//
// rr is an io.Reader that provides randomness for signing. If rr is nil, it defaults to rand.Reader.
//
// Returns the signature bytes or an error if signing fails.
func SignCryptoSigner(signer crypto.Signer, raw []byte, h crypto.Hash, opts crypto.SignerOpts, rr io.Reader) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("dsig.SignCryptoSigner: signer is nil")
	}
	return cryptosign(signer, raw, h, opts, rr)
}
