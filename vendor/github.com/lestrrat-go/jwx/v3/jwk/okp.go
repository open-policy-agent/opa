package jwk

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ed25519"
	"fmt"
	"reflect"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

func init() {
	RegisterKeyExporter(jwa.OKP(), KeyExportFunc(okpJWKToRaw))
}

// Mental note:
//
// Curve25519 refers to a particular curve, and is represented in its Montgomery form.
//
// Ed25519 refers to the biratinally equivalent curve of Curve25519, except it's in Edwards form.
// Ed25519 is the name of the curve and the also the signature scheme using that curve.
// The full name of the scheme is Edwards Curve Digital Signature Algorithm, and thus it is
// also referred to as EdDSA.
//
// X25519 refers to the Diffie-Hellman key exchange protocol that uses Cruve25519.
// Because this is an elliptic curve based Diffie Hellman protocol, it is also referred to
// as ECDH.
//
// OKP keys are used to represent private/public pairs of thse elliptic curve
// keys. But note that the name just means Octet Key Pair.

func (k *okpPublicKey) Import(rawKeyIf any) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	var crv jwa.EllipticCurveAlgorithm
	switch rawKey := rawKeyIf.(type) {
	case ed25519.PublicKey:
		k.x = rawKey
		crv = jwa.Ed25519()
		k.crv = &crv
	case *ecdh.PublicKey:
		k.x = rawKey.Bytes()
		crv = jwa.X25519()
		k.crv = &crv
	default:
		return fmt.Errorf(`unknown key type %T`, rawKeyIf)
	}

	return nil
}

func (k *okpPrivateKey) Import(rawKeyIf any) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	var crv jwa.EllipticCurveAlgorithm
	switch rawKey := rawKeyIf.(type) {
	case ed25519.PrivateKey:
		k.d = rawKey.Seed()
		k.x = rawKey.Public().(ed25519.PublicKey) //nolint:forcetypeassert
		crv = jwa.Ed25519()
		k.crv = &crv
	case *ecdh.PrivateKey:
		// k.d = rawKey.Seed()
		k.d = rawKey.Bytes()
		k.x = rawKey.PublicKey().Bytes()
		crv = jwa.X25519()
		k.crv = &crv
	default:
		return fmt.Errorf(`unknown key type %T`, rawKeyIf)
	}

	return nil
}

func buildOKPPublicKey(alg jwa.EllipticCurveAlgorithm, xbuf []byte) (any, error) {
	switch alg {
	case jwa.Ed25519():
		return ed25519.PublicKey(xbuf), nil
	case jwa.X25519():
		ret, err := ecdh.X25519().NewPublicKey(xbuf)
		if err != nil {
			return nil, fmt.Errorf(`failed to parse x25519 public key %x (size %d): %w`, xbuf, len(xbuf), err)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf(`invalid curve algorithm %s`, alg)
	}
}

// Raw returns the EC-DSA public key represented by this JWK
func (k *okpPublicKey) Raw(v any) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	crv, ok := k.Crv()
	if !ok {
		return fmt.Errorf(`missing "crv" field`)
	}

	pubk, err := buildOKPPublicKey(crv, k.x)
	if err != nil {
		return fmt.Errorf(`jwk.OKPPublicKey: failed to build public key: %w`, err)
	}

	if err := blackmagic.AssignIfCompatible(v, pubk); err != nil {
		return fmt.Errorf(`jwk.OKPPublicKey: failed to assign to destination variable: %w`, err)
	}
	return nil
}

func buildOKPPrivateKey(alg jwa.EllipticCurveAlgorithm, xbuf []byte, dbuf []byte) (any, error) {
	if len(dbuf) == 0 {
		return nil, fmt.Errorf(`cannot use empty seed`)
	}
	switch alg {
	case jwa.Ed25519():
		if len(dbuf) != ed25519.SeedSize {
			return nil, fmt.Errorf(`ed25519: wrong private key size`)
		}
		ret := ed25519.NewKeyFromSeed(dbuf)
		//nolint:forcetypeassert
		if !bytes.Equal(xbuf, ret.Public().(ed25519.PublicKey)) {
			return nil, fmt.Errorf(`ed25519: invalid x value given d value`)
		}
		return ret, nil
	case jwa.X25519():
		ret, err := ecdh.X25519().NewPrivateKey(dbuf)
		if err != nil {
			return nil, fmt.Errorf(`x25519: unable to construct x25519 private key from seed: %w`, err)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf(`invalid curve algorithm %s`, alg)
	}
}

var okpConvertibleKeys = []reflect.Type{
	reflect.TypeOf((*OKPPrivateKey)(nil)).Elem(),
	reflect.TypeOf((*OKPPublicKey)(nil)).Elem(),
}

// This is half baked. I think it will blow up if we used ecdh.* keys and/or x25519 keys
func okpJWKToRaw(key Key, _ any /* this is unused because this is half baked */) (any, error) {
	extracted, err := extractEmbeddedKey(key, okpConvertibleKeys)
	if err != nil {
		return nil, fmt.Errorf(`jwk.OKP: failed to extract embedded key: %w`, err)
	}

	switch key := extracted.(type) {
	case OKPPrivateKey:
		locker, ok := key.(rlocker)
		if ok {
			locker.rlock()
			defer locker.runlock()
		}

		crv, ok := key.Crv()
		if !ok {
			return nil, fmt.Errorf(`missing "crv" field`)
		}

		x, ok := key.X()
		if !ok {
			return nil, fmt.Errorf(`missing "x" field`)
		}

		d, ok := key.D()
		if !ok {
			return nil, fmt.Errorf(`missing "d" field`)
		}

		privk, err := buildOKPPrivateKey(crv, x, d)
		if err != nil {
			return nil, fmt.Errorf(`jwk.OKPPrivateKey: failed to build private key: %w`, err)
		}
		return privk, nil
	case OKPPublicKey:
		locker, ok := key.(rlocker)
		if ok {
			locker.rlock()
			defer locker.runlock()
		}

		crv, ok := key.Crv()
		if !ok {
			return nil, fmt.Errorf(`missing "crv" field`)
		}

		x, ok := key.X()
		if !ok {
			return nil, fmt.Errorf(`missing "x" field`)
		}
		pubk, err := buildOKPPublicKey(crv, x)
		if err != nil {
			return nil, fmt.Errorf(`jwk.OKPPublicKey: failed to build public key: %w`, err)
		}
		return pubk, nil
	default:
		return nil, ContinueError()
	}
}

func makeOKPPublicKey(src Key) (Key, error) {
	newKey := newOKPPublicKey()

	// Iterate and copy everything except for the bits that should not be in the public key
	for _, k := range src.Keys() {
		switch k {
		case OKPDKey:
			continue
		default:
			var v any
			if err := src.Get(k, &v); err != nil {
				return nil, fmt.Errorf(`failed to get field %q: %w`, k, err)
			}

			if err := newKey.Set(k, v); err != nil {
				return nil, fmt.Errorf(`failed to set field %q: %w`, k, err)
			}
		}
	}

	return newKey, nil
}

func (k *okpPrivateKey) PublicKey() (Key, error) {
	return makeOKPPublicKey(k)
}

func (k *okpPublicKey) PublicKey() (Key, error) {
	return makeOKPPublicKey(k)
}

func okpThumbprint(hash crypto.Hash, crv, x string) []byte {
	h := hash.New()
	fmt.Fprint(h, `{"crv":"`)
	fmt.Fprint(h, crv)
	fmt.Fprint(h, `","kty":"OKP","x":"`)
	fmt.Fprint(h, x)
	fmt.Fprint(h, `"}`)
	return h.Sum(nil)
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638 / 8037
func (k okpPublicKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	crv, ok := k.Crv()
	if !ok {
		return nil, fmt.Errorf(`missing "crv" field`)
	}
	return okpThumbprint(
		hash,
		crv.String(),
		base64.EncodeToString(k.x),
	), nil
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638 / 8037
func (k okpPrivateKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	crv, ok := k.Crv()
	if !ok {
		return nil, fmt.Errorf(`missing "crv" field`)
	}

	return okpThumbprint(
		hash,
		crv.String(),
		base64.EncodeToString(k.x),
	), nil
}

func validateOKPKey(key interface {
	Crv() (jwa.EllipticCurveAlgorithm, bool)
	X() ([]byte, bool)
}) error {
	if v, ok := key.Crv(); !ok || v == jwa.InvalidEllipticCurve() {
		return fmt.Errorf(`invalid curve algorithm`)
	}

	if v, ok := key.X(); !ok || len(v) == 0 {
		return fmt.Errorf(`missing "x" field`)
	}

	if priv, ok := key.(keyWithD); ok {
		if d, ok := priv.D(); !ok || len(d) == 0 {
			return fmt.Errorf(`missing "d" field`)
		}
	}
	return nil
}

func (k *okpPublicKey) Validate() error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if err := validateOKPKey(k); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.OKPPublicKey: %w`, err))
	}
	return nil
}

func (k *okpPrivateKey) Validate() error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if err := validateOKPKey(k); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.OKPPrivateKey: %w`, err))
	}
	return nil
}
