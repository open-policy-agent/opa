package legacy

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

func init() {
	algs := map[jwa.SignatureAlgorithm]func() hash.Hash{
		jwa.HS256(): sha256.New,
		jwa.HS384(): sha512.New384,
		jwa.HS512(): sha512.New,
	}

	for alg, h := range algs {
		hmacSignFuncs[alg] = makeHMACSignFunc(h)
	}
}

// HMACSigner uses crypto/hmac to sign the payloads.
// This is for legacy support only.
type HMACSigner struct {
	alg  jwa.SignatureAlgorithm
	sign hmacSignFunc
}

type HMACVerifier struct {
	signer Signer
}

type hmacSignFunc func(payload []byte, key []byte) ([]byte, error)

var hmacSignFuncs = make(map[jwa.SignatureAlgorithm]hmacSignFunc)

func NewHMACSigner(alg jwa.SignatureAlgorithm) Signer {
	return &HMACSigner{
		alg:  alg,
		sign: hmacSignFuncs[alg], // we know this will succeed
	}
}

func makeHMACSignFunc(hfunc func() hash.Hash) hmacSignFunc {
	return func(payload []byte, key []byte) ([]byte, error) {
		h := hmac.New(hfunc, key)
		if _, err := h.Write(payload); err != nil {
			return nil, fmt.Errorf(`failed to write payload using hmac: %w`, err)
		}
		return h.Sum(nil), nil
	}
}

func (s HMACSigner) Algorithm() jwa.SignatureAlgorithm {
	return s.alg
}

func (s HMACSigner) Sign(payload []byte, key any) ([]byte, error) {
	var hmackey []byte
	if err := keyconv.ByteSliceKey(&hmackey, key); err != nil {
		return nil, fmt.Errorf(`invalid key type %T. []byte is required: %w`, key, err)
	}

	if len(hmackey) == 0 {
		return nil, fmt.Errorf(`missing key while signing payload`)
	}

	return s.sign(payload, hmackey)
}

func NewHMACVerifier(alg jwa.SignatureAlgorithm) Verifier {
	s := NewHMACSigner(alg)
	return &HMACVerifier{signer: s}
}

func (v HMACVerifier) Verify(payload, signature []byte, key any) (err error) {
	expected, err := v.signer.Sign(payload, key)
	if err != nil {
		return fmt.Errorf(`failed to generated signature: %w`, err)
	}

	if !hmac.Equal(signature, expected) {
		return fmt.Errorf(`failed to match hmac signature`)
	}
	return nil
}
