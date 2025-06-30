package jws

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

// HMACSigner uses crypto/hmac to sign the payloads.
type HMACSigner struct {
	alg   jwa.SignatureAlgorithm
	hfunc func() hash.Hash
}

var hmacSigners map[jwa.SignatureAlgorithm]Signer

func init() {
	algs := map[jwa.SignatureAlgorithm]func() hash.Hash{
		jwa.HS256(): sha256.New,
		jwa.HS384(): sha512.New384,
		jwa.HS512(): sha512.New,
	}

	hmacSigners = make(map[jwa.SignatureAlgorithm]Signer)

	for alg, h := range algs {
		hmacSigners[alg] = &HMACSigner{
			alg:   alg,
			hfunc: h,
		}
	}
}

func newHMACSigner(alg jwa.SignatureAlgorithm) Signer {
	return hmacSigners[alg]
}

func (s HMACSigner) Algorithm() jwa.SignatureAlgorithm {
	return s.alg
}

func (s HMACSigner) Sign(payload []byte, key interface{}) ([]byte, error) {
	var hmackey []byte
	if err := keyconv.ByteSliceKey(&hmackey, key); err != nil {
		return nil, fmt.Errorf(`invalid key type %T. []byte is required: %w`, key, err)
	}

	if len(hmackey) == 0 {
		return nil, fmt.Errorf(`missing key while signing payload`)
	}

	h := hmac.New(s.hfunc, hmackey)
	if _, err := h.Write(payload); err != nil {
		return nil, fmt.Errorf(`failed to write payload using hmac: %w`, err)
	}
	return h.Sum(nil), nil
}

func newHMACVerifier(alg jwa.SignatureAlgorithm) Verifier {
	s := newHMACSigner(alg)
	return &HMACVerifier{signer: s}
}

func (v HMACVerifier) Verify(payload, signature []byte, key interface{}) (err error) {
	expected, err := v.signer.Sign(payload, key)
	if err != nil {
		return fmt.Errorf(`failed to generated signature: %w`, err)
	}

	if !hmac.Equal(signature, expected) {
		return fmt.Errorf(`failed to match hmac signature`)
	}
	return nil
}
