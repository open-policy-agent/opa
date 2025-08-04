package jwk

import (
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"reflect"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/ecutil"
	"github.com/lestrrat-go/jwx/v3/jwa"
	ourecdsa "github.com/lestrrat-go/jwx/v3/jwk/ecdsa"
)

func init() {
	ourecdsa.RegisterCurve(jwa.P256(), elliptic.P256())
	ourecdsa.RegisterCurve(jwa.P384(), elliptic.P384())
	ourecdsa.RegisterCurve(jwa.P521(), elliptic.P521())

	RegisterKeyExporter(jwa.EC(), KeyExportFunc(ecdsaJWKToRaw))
}

func (k *ecdsaPublicKey) Import(rawKey *ecdsa.PublicKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if rawKey.X == nil {
		return fmt.Errorf(`invalid ecdsa.PublicKey`)
	}

	if rawKey.Y == nil {
		return fmt.Errorf(`invalid ecdsa.PublicKey`)
	}

	xbuf := ecutil.AllocECPointBuffer(rawKey.X, rawKey.Curve)
	ybuf := ecutil.AllocECPointBuffer(rawKey.Y, rawKey.Curve)
	defer ecutil.ReleaseECPointBuffer(xbuf)
	defer ecutil.ReleaseECPointBuffer(ybuf)

	k.x = make([]byte, len(xbuf))
	copy(k.x, xbuf)
	k.y = make([]byte, len(ybuf))
	copy(k.y, ybuf)

	alg, err := ourecdsa.AlgorithmFromCurve(rawKey.Curve)
	if err != nil {
		return fmt.Errorf(`jwk: failed to get algorithm for converting ECDSA public key to JWK: %w`, err)
	}
	k.crv = &alg

	return nil
}

func (k *ecdsaPrivateKey) Import(rawKey *ecdsa.PrivateKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if rawKey.PublicKey.X == nil {
		return fmt.Errorf(`invalid ecdsa.PrivateKey`)
	}
	if rawKey.PublicKey.Y == nil {
		return fmt.Errorf(`invalid ecdsa.PrivateKey`)
	}
	if rawKey.D == nil {
		return fmt.Errorf(`invalid ecdsa.PrivateKey`)
	}

	xbuf := ecutil.AllocECPointBuffer(rawKey.PublicKey.X, rawKey.Curve)
	ybuf := ecutil.AllocECPointBuffer(rawKey.PublicKey.Y, rawKey.Curve)
	dbuf := ecutil.AllocECPointBuffer(rawKey.D, rawKey.Curve)
	defer ecutil.ReleaseECPointBuffer(xbuf)
	defer ecutil.ReleaseECPointBuffer(ybuf)
	defer ecutil.ReleaseECPointBuffer(dbuf)

	k.x = make([]byte, len(xbuf))
	copy(k.x, xbuf)
	k.y = make([]byte, len(ybuf))
	copy(k.y, ybuf)
	k.d = make([]byte, len(dbuf))
	copy(k.d, dbuf)

	alg, err := ourecdsa.AlgorithmFromCurve(rawKey.Curve)
	if err != nil {
		return fmt.Errorf(`jwk: failed to get algorithm for converting ECDSA private key to JWK: %w`, err)
	}
	k.crv = &alg

	return nil
}

func buildECDSAPublicKey(alg jwa.EllipticCurveAlgorithm, xbuf, ybuf []byte) (*ecdsa.PublicKey, error) {
	crv, err := ourecdsa.CurveFromAlgorithm(alg)
	if err != nil {
		return nil, fmt.Errorf(`jwk: failed to get algorithm for ECDSA public key: %w`, err)
	}

	var x, y big.Int
	x.SetBytes(xbuf)
	y.SetBytes(ybuf)

	return &ecdsa.PublicKey{Curve: crv, X: &x, Y: &y}, nil
}

func buildECDHPublicKey(alg jwa.EllipticCurveAlgorithm, xbuf, ybuf []byte) (*ecdh.PublicKey, error) {
	var ecdhcrv ecdh.Curve
	switch alg {
	case jwa.X25519():
		ecdhcrv = ecdh.X25519()
	case jwa.P256():
		ecdhcrv = ecdh.P256()
	case jwa.P384():
		ecdhcrv = ecdh.P384()
	case jwa.P521():
		ecdhcrv = ecdh.P521()
	default:
		return nil, fmt.Errorf(`jwk: unsupported ECDH curve %s`, alg)
	}

	return ecdhcrv.NewPublicKey(append([]byte{0x04}, append(xbuf, ybuf...)...))
}

func buildECDHPrivateKey(alg jwa.EllipticCurveAlgorithm, dbuf []byte) (*ecdh.PrivateKey, error) {
	var ecdhcrv ecdh.Curve
	switch alg {
	case jwa.X25519():
		ecdhcrv = ecdh.X25519()
	case jwa.P256():
		ecdhcrv = ecdh.P256()
	case jwa.P384():
		ecdhcrv = ecdh.P384()
	case jwa.P521():
		ecdhcrv = ecdh.P521()
	default:
		return nil, fmt.Errorf(`jwk: unsupported ECDH curve %s`, alg)
	}

	return ecdhcrv.NewPrivateKey(dbuf)
}

var ecdsaConvertibleTypes = []reflect.Type{
	reflect.TypeOf((*ECDSAPrivateKey)(nil)).Elem(),
	reflect.TypeOf((*ECDSAPublicKey)(nil)).Elem(),
}

func ecdsaJWKToRaw(keyif Key, hint any) (any, error) {
	var isECDH bool

	extracted, err := extractEmbeddedKey(keyif, ecdsaConvertibleTypes)
	if err != nil {
		return nil, fmt.Errorf(`jwk: failed to extract embedded key: %w`, err)
	}

	switch k := extracted.(type) {
	case ECDSAPrivateKey:
		switch hint.(type) {
		case ecdsa.PrivateKey, *ecdsa.PrivateKey:
		case ecdh.PrivateKey, *ecdh.PrivateKey:
			isECDH = true
		default:
			rv := reflect.ValueOf(hint)
			//nolint:revive
			if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Interface {
				// pointer to an interface value, presumably they want us to dynamically
				// create an object of the right type
			} else {
				return nil, fmt.Errorf(`invalid destination object type %T: %w`, hint, ContinueError())
			}
		}

		locker, ok := k.(rlocker)
		if ok {
			locker.rlock()
			defer locker.runlock()
		}

		crv, ok := k.Crv()
		if !ok {
			return nil, fmt.Errorf(`missing "crv" field`)
		}

		if isECDH {
			d, ok := k.D()
			if !ok {
				return nil, fmt.Errorf(`missing "d" field`)
			}
			return buildECDHPrivateKey(crv, d)
		}

		x, ok := k.X()
		if !ok {
			return nil, fmt.Errorf(`missing "x" field`)
		}
		y, ok := k.Y()
		if !ok {
			return nil, fmt.Errorf(`missing "y" field`)
		}
		pubk, err := buildECDSAPublicKey(crv, x, y)
		if err != nil {
			return nil, fmt.Errorf(`failed to build public key: %w`, err)
		}

		var key ecdsa.PrivateKey
		var d big.Int

		origD, ok := k.D()
		if !ok {
			return nil, fmt.Errorf(`missing "d" field`)
		}

		d.SetBytes(origD)
		key.D = &d
		key.PublicKey = *pubk

		return &key, nil
	case ECDSAPublicKey:
		switch hint.(type) {
		case ecdsa.PublicKey, *ecdsa.PublicKey:
		case ecdh.PublicKey, *ecdh.PublicKey:
			isECDH = true
		default:
			rv := reflect.ValueOf(hint)
			//nolint:revive
			if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Interface {
				// pointer to an interface value, presumably they want us to dynamically
				// create an object of the right type
			} else {
				return nil, fmt.Errorf(`invalid destination object type %T: %w`, hint, ContinueError())
			}
		}

		locker, ok := k.(rlocker)
		if ok {
			locker.rlock()
			defer locker.runlock()
		}

		crv, ok := k.Crv()
		if !ok {
			return nil, fmt.Errorf(`missing "crv" field`)
		}

		x, ok := k.X()
		if !ok {
			return nil, fmt.Errorf(`missing "x" field`)
		}

		y, ok := k.Y()
		if !ok {
			return nil, fmt.Errorf(`missing "y" field`)
		}
		if isECDH {
			return buildECDHPublicKey(crv, x, y)
		}
		return buildECDSAPublicKey(crv, x, y)
	default:
		return nil, ContinueError()
	}
}

func makeECDSAPublicKey(src Key) (Key, error) {
	newKey := newECDSAPublicKey()

	// Iterate and copy everything except for the bits that should not be in the public key
	for _, k := range src.Keys() {
		switch k {
		case ECDSADKey:
			continue
		default:
			var v any
			if err := src.Get(k, &v); err != nil {
				return nil, fmt.Errorf(`ecdsa: makeECDSAPublicKey: failed to get field %q: %w`, k, err)
			}

			if err := newKey.Set(k, v); err != nil {
				return nil, fmt.Errorf(`ecdsa: makeECDSAPublicKey: failed to set field %q: %w`, k, err)
			}
		}
	}

	return newKey, nil
}

func (k *ecdsaPrivateKey) PublicKey() (Key, error) {
	return makeECDSAPublicKey(k)
}

func (k *ecdsaPublicKey) PublicKey() (Key, error) {
	return makeECDSAPublicKey(k)
}

func ecdsaThumbprint(hash crypto.Hash, crv, x, y string) []byte {
	h := hash.New()
	fmt.Fprint(h, `{"crv":"`)
	fmt.Fprint(h, crv)
	fmt.Fprint(h, `","kty":"EC","x":"`)
	fmt.Fprint(h, x)
	fmt.Fprint(h, `","y":"`)
	fmt.Fprint(h, y)
	fmt.Fprint(h, `"}`)
	return h.Sum(nil)
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638
func (k ecdsaPublicKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var key ecdsa.PublicKey
	if err := Export(&k, &key); err != nil {
		return nil, fmt.Errorf(`failed to export ecdsa.PublicKey for thumbprint generation: %w`, err)
	}

	xbuf := ecutil.AllocECPointBuffer(key.X, key.Curve)
	ybuf := ecutil.AllocECPointBuffer(key.Y, key.Curve)
	defer ecutil.ReleaseECPointBuffer(xbuf)
	defer ecutil.ReleaseECPointBuffer(ybuf)

	return ecdsaThumbprint(
		hash,
		key.Curve.Params().Name,
		base64.EncodeToString(xbuf),
		base64.EncodeToString(ybuf),
	), nil
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638
func (k ecdsaPrivateKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var key ecdsa.PrivateKey
	if err := Export(&k, &key); err != nil {
		return nil, fmt.Errorf(`failed to export ecdsa.PrivateKey for thumbprint generation: %w`, err)
	}

	xbuf := ecutil.AllocECPointBuffer(key.X, key.Curve)
	ybuf := ecutil.AllocECPointBuffer(key.Y, key.Curve)
	defer ecutil.ReleaseECPointBuffer(xbuf)
	defer ecutil.ReleaseECPointBuffer(ybuf)

	return ecdsaThumbprint(
		hash,
		key.Curve.Params().Name,
		base64.EncodeToString(xbuf),
		base64.EncodeToString(ybuf),
	), nil
}

func ecdsaValidateKey(k interface {
	Crv() (jwa.EllipticCurveAlgorithm, bool)
	X() ([]byte, bool)
	Y() ([]byte, bool)
}, checkPrivate bool) error {
	crvtyp, ok := k.Crv()
	if !ok {
		return fmt.Errorf(`missing "crv" field`)
	}

	crv, err := ourecdsa.CurveFromAlgorithm(crvtyp)
	if err != nil {
		return fmt.Errorf(`invalid curve algorithm %q: %w`, crvtyp, err)
	}

	keySize := ecutil.CalculateKeySize(crv)
	if x, ok := k.X(); !ok || len(x) != keySize {
		return fmt.Errorf(`invalid "x" length (%d) for curve %q`, len(x), crv.Params().Name)
	}

	if y, ok := k.Y(); !ok || len(y) != keySize {
		return fmt.Errorf(`invalid "y" length (%d) for curve %q`, len(y), crv.Params().Name)
	}

	if checkPrivate {
		if priv, ok := k.(keyWithD); ok {
			if d, ok := priv.D(); !ok || len(d) != keySize {
				return fmt.Errorf(`invalid "d" length (%d) for curve %q`, len(d), crv.Params().Name)
			}
		} else {
			return fmt.Errorf(`missing "d" value`)
		}
	}
	return nil
}

func (k *ecdsaPrivateKey) Validate() error {
	if err := ecdsaValidateKey(k, true); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.ECDSAPrivateKey: %w`, err))
	}
	return nil
}

func (k *ecdsaPublicKey) Validate() error {
	if err := ecdsaValidateKey(k, false); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.ECDSAPublicKey: %w`, err))
	}
	return nil
}
