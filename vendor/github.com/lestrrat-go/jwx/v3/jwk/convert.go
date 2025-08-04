package jwk

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sync"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/ecutil"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

// # Converting between Raw Keys and `jwk.Key`s
//
// A converter that converts from a raw key to a `jwk.Key` is called a KeyImporter.
// A converter that converts from a `jwk.Key` to a raw key is called a KeyExporter.

var keyImporters = make(map[reflect.Type]KeyImporter)
var keyExporters = make(map[jwa.KeyType][]KeyExporter)

var muKeyImporters sync.RWMutex
var muKeyExporters sync.RWMutex

// RegisterKeyImporter registers a KeyImporter for the given raw key. When `jwk.Import()` is called,
// the library will look up the appropriate KeyImporter for the given raw key type (via `reflect`)
// and execute the KeyImporters in succession until either one of them succeeds, or all of them fail.
func RegisterKeyImporter(from any, conv KeyImporter) {
	muKeyImporters.Lock()
	defer muKeyImporters.Unlock()
	keyImporters[reflect.TypeOf(from)] = conv
}

// RegisterKeyExporter registers a KeyExporter for the given key type. When `key.Raw()` is called,
// the library will look up the appropriate KeyExporter for the given key type and execute the
// KeyExporters in succession until either one of them succeeds, or all of them fail.
func RegisterKeyExporter(kty jwa.KeyType, conv KeyExporter) {
	muKeyExporters.Lock()
	defer muKeyExporters.Unlock()
	convs, ok := keyExporters[kty]
	if !ok {
		convs = []KeyExporter{conv}
	} else {
		convs = append([]KeyExporter{conv}, convs...)
	}
	keyExporters[kty] = convs
}

// KeyImporter is used to convert from a raw key to a `jwk.Key`. mneumonic: from the PoV of the `jwk.Key`,
// we're _importing_ a raw key.
type KeyImporter interface {
	// Import takes the raw key to be converted, and returns a `jwk.Key` or an error if the conversion fails.
	Import(any) (Key, error)
}

// KeyImportFunc is a convenience type to implement KeyImporter as a function.
type KeyImportFunc func(any) (Key, error)

func (f KeyImportFunc) Import(raw any) (Key, error) {
	return f(raw)
}

// KeyExporter is used to convert from a `jwk.Key` to a raw key. mneumonic: from the PoV of the `jwk.Key`,
// we're _exporting_ it to a raw key.
type KeyExporter interface {
	// Export takes the `jwk.Key` to be converted, and a hint (the raw key to be converted to).
	// The hint is the object that the user requested the result to be assigned to.
	// The method should return the converted raw key, or an error if the conversion fails.
	//
	// Third party modules MUST NOT modifiy the hint object.
	//
	// When the user calls `key.Export(dst)`, the `dst` object is a _pointer_ to the
	// object that the user wants the result to be assigned to, but the converter
	// receives the _value_ that this pointer points to, to make it easier to
	// detect the type of the result.
	//
	// Note that the second argument may be an `any` (which means that the
	// user has delegated the type detection to the converter).
	//
	// Export must NOT modify the hint object, and should return jwk.ContinueError
	// if the hint object is not compatible with the converter.
	Export(Key, any) (any, error)
}

// KeyExportFunc is a convenience type to implement KeyExporter as a function.
type KeyExportFunc func(Key, any) (any, error)

func (f KeyExportFunc) Export(key Key, hint any) (any, error) {
	return f(key, hint)
}

func init() {
	{
		f := KeyImportFunc(rsaPrivateKeyToJWK)
		k := rsa.PrivateKey{}
		RegisterKeyImporter(k, f)
		RegisterKeyImporter(&k, f)
	}
	{
		f := KeyImportFunc(rsaPublicKeyToJWK)
		k := rsa.PublicKey{}
		RegisterKeyImporter(k, f)
		RegisterKeyImporter(&k, f)
	}
	{
		f := KeyImportFunc(ecdsaPrivateKeyToJWK)
		k := ecdsa.PrivateKey{}
		RegisterKeyImporter(k, f)
		RegisterKeyImporter(&k, f)
	}
	{
		f := KeyImportFunc(ecdsaPublicKeyToJWK)
		k := ecdsa.PublicKey{}
		RegisterKeyImporter(k, f)
		RegisterKeyImporter(&k, f)
	}
	{
		f := KeyImportFunc(okpPrivateKeyToJWK)
		for _, k := range []any{ed25519.PrivateKey(nil)} {
			RegisterKeyImporter(k, f)
		}
	}
	{
		f := KeyImportFunc(ecdhPrivateKeyToJWK)
		for _, k := range []any{ecdh.PrivateKey{}, &ecdh.PrivateKey{}} {
			RegisterKeyImporter(k, f)
		}
	}
	{
		f := KeyImportFunc(okpPublicKeyToJWK)
		for _, k := range []any{ed25519.PublicKey(nil)} {
			RegisterKeyImporter(k, f)
		}
	}
	{
		f := KeyImportFunc(ecdhPublicKeyToJWK)
		for _, k := range []any{ecdh.PublicKey{}, &ecdh.PublicKey{}} {
			RegisterKeyImporter(k, f)
		}
	}
	RegisterKeyImporter([]byte(nil), KeyImportFunc(bytesToKey))
}

func ecdhPrivateKeyToJWK(src any) (Key, error) {
	var raw *ecdh.PrivateKey
	switch src := src.(type) {
	case *ecdh.PrivateKey:
		raw = src
	case ecdh.PrivateKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to ECDH jwk.Key`, src)
	}

	switch raw.Curve() {
	case ecdh.X25519():
		return okpPrivateKeyToJWK(raw)
	case ecdh.P256():
		return ecdhPrivateKeyToECJWK(raw, elliptic.P256())
	case ecdh.P384():
		return ecdhPrivateKeyToECJWK(raw, elliptic.P384())
	case ecdh.P521():
		return ecdhPrivateKeyToECJWK(raw, elliptic.P521())
	default:
		return nil, fmt.Errorf(`unsupported curve %s`, raw.Curve())
	}
}

func ecdhPrivateKeyToECJWK(raw *ecdh.PrivateKey, crv elliptic.Curve) (Key, error) {
	pub := raw.PublicKey()
	rawpub := pub.Bytes()

	size := ecutil.CalculateKeySize(crv)
	var x, y, d big.Int
	x.SetBytes(rawpub[1 : 1+size])
	y.SetBytes(rawpub[1+size:])
	d.SetBytes(raw.Bytes())

	var ecdsaPriv ecdsa.PrivateKey
	ecdsaPriv.Curve = crv
	ecdsaPriv.D = &d
	ecdsaPriv.X = &x
	ecdsaPriv.Y = &y
	return ecdsaPrivateKeyToJWK(&ecdsaPriv)
}

func ecdhPublicKeyToJWK(src any) (Key, error) {
	var raw *ecdh.PublicKey
	switch src := src.(type) {
	case *ecdh.PublicKey:
		raw = src
	case ecdh.PublicKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to ECDH jwk.Key`, src)
	}

	switch raw.Curve() {
	case ecdh.X25519():
		return okpPublicKeyToJWK(raw)
	case ecdh.P256():
		return ecdhPublicKeyToECJWK(raw, elliptic.P256())
	case ecdh.P384():
		return ecdhPublicKeyToECJWK(raw, elliptic.P384())
	case ecdh.P521():
		return ecdhPublicKeyToECJWK(raw, elliptic.P521())
	default:
		return nil, fmt.Errorf(`unsupported curve %s`, raw.Curve())
	}
}

func ecdhPublicKeyToECJWK(raw *ecdh.PublicKey, crv elliptic.Curve) (Key, error) {
	rawbytes := raw.Bytes()
	size := ecutil.CalculateKeySize(crv)
	var x, y big.Int

	x.SetBytes(rawbytes[1 : 1+size])
	y.SetBytes(rawbytes[1+size:])
	var ecdsaPriv ecdsa.PublicKey
	ecdsaPriv.Curve = crv
	ecdsaPriv.X = &x
	ecdsaPriv.Y = &y
	return ecdsaPublicKeyToJWK(&ecdsaPriv)
}

// These may seem a bit repetitive and redandunt, but the problem is that
// each key type has its own Import method -- for example, Import(*ecdsa.PrivateKey)
// vs Import(*rsa.PrivateKey), and therefore they can't just be bundled into
// a single function.
func rsaPrivateKeyToJWK(src any) (Key, error) {
	var raw *rsa.PrivateKey
	switch src := src.(type) {
	case *rsa.PrivateKey:
		raw = src
	case rsa.PrivateKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to RSA jwk.Key`, src)
	}
	k := newRSAPrivateKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func rsaPublicKeyToJWK(src any) (Key, error) {
	var raw *rsa.PublicKey
	switch src := src.(type) {
	case *rsa.PublicKey:
		raw = src
	case rsa.PublicKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to RSA jwk.Key`, src)
	}
	k := newRSAPublicKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func ecdsaPrivateKeyToJWK(src any) (Key, error) {
	var raw *ecdsa.PrivateKey
	switch src := src.(type) {
	case *ecdsa.PrivateKey:
		raw = src
	case ecdsa.PrivateKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to ECDSA jwk.Key`, src)
	}
	k := newECDSAPrivateKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func ecdsaPublicKeyToJWK(src any) (Key, error) {
	var raw *ecdsa.PublicKey
	switch src := src.(type) {
	case *ecdsa.PublicKey:
		raw = src
	case ecdsa.PublicKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to ECDSA jwk.Key`, src)
	}
	k := newECDSAPublicKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func okpPrivateKeyToJWK(src any) (Key, error) {
	var raw any
	switch src.(type) {
	case ed25519.PrivateKey, *ecdh.PrivateKey:
		raw = src
	case ecdh.PrivateKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to OKP jwk.Key`, src)
	}
	k := newOKPPrivateKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func okpPublicKeyToJWK(src any) (Key, error) {
	var raw any
	switch src.(type) {
	case ed25519.PublicKey, *ecdh.PublicKey:
		raw = src
	case ecdh.PublicKey:
		raw = &src
	default:
		return nil, fmt.Errorf(`jwk: convert raw to OKP jwk.Key: cannot convert key type '%T' to OKP jwk.Key`, src)
	}
	k := newOKPPublicKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

func bytesToKey(src any) (Key, error) {
	var raw []byte
	switch src := src.(type) {
	case []byte:
		raw = src
	default:
		return nil, fmt.Errorf(`cannot convert key type '%T' to symmetric jwk.Key`, src)
	}

	k := newSymmetricKey()
	if err := k.Import(raw); err != nil {
		return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, raw, err)
	}
	return k, nil
}

// Export converts a `jwk.Key` to a Export key. The dst argument must be a pointer to the
// object that the user wants the result to be assigned to.
//
// Normally you would pass a pointer to the zero value of the raw key type
// such as &(*rsa.PrivateKey) or &(*ecdsa.PublicKey), which gets assigned
// the converted key.
//
// If you do not know the exact type of a jwk.Key before attempting
// to obtain the raw key, you can simply pass a pointer to an
// empty interface as the second argument
//
// If you already know the exact type, it is recommended that you
// pass a pointer to the zero value of the actual key type for efficiency.
//
// Be careful when/if you are using a third party key type that implements
// the `jwk.Key` interface, as the first argument. This function tries hard
// to Do The Right Thing, but it is not guaranteed to work in all cases,
// especially when the object implements the `jwk.Key` interface via
// embedding.
func Export(key Key, dst any) error {
	// dst better be a pointer
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf(`jwk.Export: destination object must be a pointer`)
	}
	muKeyExporters.RLock()
	exporters, ok := keyExporters[key.KeyType()]
	muKeyExporters.RUnlock()
	if !ok {
		return fmt.Errorf(`jwk.Export: no exporters registered for key type '%T'`, key)
	}
	for _, conv := range exporters {
		v, err := conv.Export(key, dst)
		if err != nil {
			if errors.Is(err, ContinueError()) {
				continue
			}
			return fmt.Errorf(`jwk.Export: failed to export jwk.Key to raw format: %w`, err)
		}
		if err := blackmagic.AssignIfCompatible(dst, v); err != nil {
			return fmt.Errorf(`jwk.Export: failed to assign key: %w`, err)
		}
		return nil
	}
	return fmt.Errorf(`jwk.Export: no suitable exporter found for key type '%T'`, key)
}
