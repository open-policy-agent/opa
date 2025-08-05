//go:generate ../tools/cmd/genjwk.sh

package jwk

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
)

var registry = json.NewRegistry()

func bigIntToBytes(n *big.Int) ([]byte, error) {
	if n == nil {
		return nil, fmt.Errorf(`invalid *big.Int value`)
	}
	return n.Bytes(), nil
}

func init() {
	if err := RegisterProbeField(reflect.StructField{
		Name: "Kty",
		Type: reflect.TypeOf(""),
		Tag:  `json:"kty"`,
	}); err != nil {
		panic(fmt.Errorf("failed to register mandatory probe for 'kty' field: %w", err))
	}
	if err := RegisterProbeField(reflect.StructField{
		Name: "D",
		Type: reflect.TypeOf(json.RawMessage(nil)),
		Tag:  `json:"d,omitempty"`,
	}); err != nil {
		panic(fmt.Errorf("failed to register mandatory probe for 'kty' field: %w", err))
	}
}

// Import creates a jwk.Key from the given key (RSA/ECDSA/symmetric keys).
//
// The constructor auto-detects the type of key to be instantiated
// based on the input type:
//
//   - "crypto/rsa".PrivateKey and "crypto/rsa".PublicKey creates an RSA based key
//   - "crypto/ecdsa".PrivateKey and "crypto/ecdsa".PublicKey creates an EC based key
//   - "crypto/ed25519".PrivateKey and "crypto/ed25519".PublicKey creates an OKP based key
//   - "crypto/ecdh".PrivateKey and "crypto/ecdh".PublicKey creates an OKP based key
//   - []byte creates a symmetric key
func Import(raw any) (Key, error) {
	if raw == nil {
		return nil, importerr(`a non-nil key is required`)
	}

	muKeyImporters.RLock()
	conv, ok := keyImporters[reflect.TypeOf(raw)]
	muKeyImporters.RUnlock()
	if !ok {
		return nil, importerr(`failed to convert %T to jwk.Key: no converters were able to convert`, raw)
	}

	return conv.Import(raw)
}

// PublicSetOf returns a new jwk.Set consisting of
// public keys of the keys contained in the set.
//
// This is useful when you are generating a set of private keys, and
// you want to generate the corresponding public versions for the
// users to verify with.
//
// Be aware that all fields will be copied onto the new public key. It is the caller's
// responsibility to remove any fields, if necessary.
func PublicSetOf(v Set) (Set, error) {
	newSet := NewSet()

	n := v.Len()
	for i := range n {
		k, ok := v.Key(i)
		if !ok {
			return nil, fmt.Errorf(`key not found`)
		}
		pubKey, err := PublicKeyOf(k)
		if err != nil {
			return nil, fmt.Errorf(`failed to get public key of %T: %w`, k, err)
		}
		if err := newSet.AddKey(pubKey); err != nil {
			return nil, fmt.Errorf(`failed to add key to public key set: %w`, err)
		}
	}

	return newSet, nil
}

// PublicKeyOf returns the corresponding public version of the jwk.Key.
// If `v` is a SymmetricKey, then the same value is returned.
// If `v` is already a public key, the key itself is returned.
//
// If `v` is a private key type that has a `PublicKey()` method, be aware
// that all fields will be copied onto the new public key. It is the caller's
// responsibility to remove any fields, if necessary
//
// If `v` is a raw key, the key is first converted to a `jwk.Key`
func PublicKeyOf(v any) (Key, error) {
	// This should catch all jwk.Key instances
	if pk, ok := v.(PublicKeyer); ok {
		return pk.PublicKey()
	}

	jk, err := Import(v)
	if err != nil {
		return nil, fmt.Errorf(`jwk.PublicKeyOf: failed to convert key into JWK: %w`, err)
	}

	return jk.PublicKey()
}

// PublicRawKeyOf returns the corresponding public key of the given
// value `v` (e.g. given *rsa.PrivateKey, *rsa.PublicKey is returned)
// If `v` is already a public key, the key itself is returned.
//
// The returned value will always be a pointer to the public key,
// except when a []byte (e.g. symmetric key, ed25519 key) is passed to `v`.
// In this case, the same []byte value is returned.
//
// This function must go through converting the object once to a jwk.Key,
// then back to a raw key, so it's not exactly efficient.
func PublicRawKeyOf(v any) (any, error) {
	pk, ok := v.(PublicKeyer)
	if !ok {
		k, err := Import(v)
		if err != nil {
			return nil, fmt.Errorf(`jwk.PublicRawKeyOf: failed to convert key to jwk.Key: %w`, err)
		}

		pk, ok = k.(PublicKeyer)
		if !ok {
			return nil, fmt.Errorf(`jwk.PublicRawKeyOf: failed to convert key to jwk.PublicKeyer: %w`, err)
		}
	}

	pubk, err := pk.PublicKey()
	if err != nil {
		return nil, fmt.Errorf(`jwk.PublicRawKeyOf: failed to obtain public key from %T: %w`, v, err)
	}

	var raw any
	if err := Export(pubk, &raw); err != nil {
		return nil, fmt.Errorf(`jwk.PublicRawKeyOf: failed to obtain raw key from %T: %w`, pubk, err)
	}
	return raw, nil
}

// ParseRawKey is a combination of ParseKey and Raw. It parses a single JWK key,
// and assigns the "raw" key to the given parameter. The key must either be
// a pointer to an empty interface, or a pointer to the actual raw key type
// such as *rsa.PrivateKey, *ecdsa.PublicKey, *[]byte, etc.
func ParseRawKey(data []byte, rawkey any) error {
	key, err := ParseKey(data)
	if err != nil {
		return fmt.Errorf(`failed to parse key: %w`, err)
	}

	if err := Export(key, rawkey); err != nil {
		return fmt.Errorf(`failed to assign to raw key variable: %w`, err)
	}

	return nil
}

type setDecodeCtx struct {
	json.DecodeCtx

	ignoreParseError bool
}

func (ctx *setDecodeCtx) IgnoreParseError() bool {
	return ctx.ignoreParseError
}

// ParseKey parses a single key JWK. Unlike `jwk.Parse` this method will
// report failure if you attempt to pass a JWK set. Only use this function
// when you know that the data is a single JWK.
//
// Given a WithPEM(true) option, this function assumes that the given input
// is PEM encoded ASN.1 DER format key.
//
// Note that a successful parsing of any type of key does NOT necessarily
// guarantee a valid key. For example, no checks against expiration dates
// are performed for certificate expiration, no checks against missing
// parameters are performed, etc.
func ParseKey(data []byte, options ...ParseOption) (Key, error) {
	var parsePEM bool
	var localReg *json.Registry
	var pemDecoder PEMDecoder
	for _, option := range options {
		switch option.Ident() {
		case identPEM{}:
			if err := option.Value(&parsePEM); err != nil {
				return nil, fmt.Errorf(`failed to retrieve PEM option value: %w`, err)
			}
		case identPEMDecoder{}:
			if err := option.Value(&pemDecoder); err != nil {
				return nil, fmt.Errorf(`failed to retrieve PEMDecoder option value: %w`, err)
			}
		case identLocalRegistry{}:
			if err := option.Value(&localReg); err != nil {
				return nil, fmt.Errorf(`failed to retrieve local registry option value: %w`, err)
			}
		case identTypedField{}:
			var pair typedFieldPair // temporary var needed for typed field
			if err := option.Value(&pair); err != nil {
				return nil, fmt.Errorf(`failed to retrieve typed field option value: %w`, err)
			}
			if localReg == nil {
				localReg = json.NewRegistry()
			}
			localReg.Register(pair.Name, pair.Value)
		case identIgnoreParseError{}:
			return nil, fmt.Errorf(`jwk.WithIgnoreParseError() cannot be used for ParseKey()`)
		}
	}

	if parsePEM {
		var raw any

		// PEMDecoder should probably be deprecated, because of being a misnomer.
		if pemDecoder != nil {
			if err := decodeX509WithPEMDEcoder(&raw, data, pemDecoder); err != nil {
				return nil, fmt.Errorf(`failed to decode PEM encoded key: %w`, err)
			}
		} else {
			// This version takes into account the various X509 decoders that are
			// pre-registered.
			if err := decodeX509(&raw, data); err != nil {
				return nil, fmt.Errorf(`failed to decode X.509 encoded key: %w`, err)
			}
		}
		return Import(raw)
	}

	probe, err := keyProbe.Probe(data)
	if err != nil {
		return nil, fmt.Errorf(`jwk.Parse: failed to probe data: %w`, err)
	}

	unmarshaler := keyUnmarshaler{localReg: localReg}

	muKeyParser.RLock()
	parsers := make([]KeyParser, len(keyParsers))
	copy(parsers, keyParsers)
	muKeyParser.RUnlock()

	for i := len(parsers) - 1; i >= 0; i-- {
		parser := parsers[i]
		key, err := parser.ParseKey(probe, &unmarshaler, data)
		if err == nil {
			return key, nil
		}

		if errors.Is(err, ContinueError()) {
			continue
		}

		return nil, err
	}
	return nil, fmt.Errorf(`jwk.Parse: no parser was able to parse the key`)
}

// Parse parses JWK from the incoming []byte.
//
// For JWK sets, this is a convenience function. You could just as well
// call `json.Unmarshal` against an empty set created by `jwk.NewSet()`
// to parse a JSON buffer into a `jwk.Set`.
//
// This function exists because many times the user does not know before hand
// if a JWK(s) resource at a remote location contains a single JWK key or
// a JWK set, and `jwk.Parse()` can handle either case, returning a JWK Set
// even if the data only contains a single JWK key
//
// If you are looking for more information on how JWKs are parsed, or if
// you know for sure that you have a single key, please see the documentation
// for `jwk.ParseKey()`.
func Parse(src []byte, options ...ParseOption) (Set, error) {
	var parsePEM bool
	var parseX509 bool
	var localReg *json.Registry
	var ignoreParseError bool
	var pemDecoder PEMDecoder
	for _, option := range options {
		switch option.Ident() {
		case identPEM{}:
			if err := option.Value(&parsePEM); err != nil {
				return nil, parseerr(`failed to retrieve PEM option value: %w`, err)
			}
		case identX509{}:
			if err := option.Value(&parseX509); err != nil {
				return nil, parseerr(`failed to retrieve X509 option value: %w`, err)
			}
		case identPEMDecoder{}:
			if err := option.Value(&pemDecoder); err != nil {
				return nil, parseerr(`failed to retrieve PEMDecoder option value: %w`, err)
			}
		case identIgnoreParseError{}:
			if err := option.Value(&ignoreParseError); err != nil {
				return nil, parseerr(`failed to retrieve IgnoreParseError option value: %w`, err)
			}
		case identTypedField{}:
			var pair typedFieldPair // temporary var needed for typed field
			if err := option.Value(&pair); err != nil {
				return nil, parseerr(`failed to retrieve typed field option value: %w`, err)
			}
			if localReg == nil {
				localReg = json.NewRegistry()
			}
			localReg.Register(pair.Name, pair.Value)
		}
	}

	s := NewSet()

	if parsePEM || parseX509 {
		if pemDecoder == nil {
			pemDecoder = NewPEMDecoder()
		}
		src = bytes.TrimSpace(src)
		for len(src) > 0 {
			raw, rest, err := pemDecoder.Decode(src)
			if err != nil {
				return nil, parseerr(`failed to parse PEM encoded key: %w`, err)
			}
			key, err := Import(raw)
			if err != nil {
				return nil, parseerr(`failed to create jwk.Key from %T: %w`, raw, err)
			}
			if err := s.AddKey(key); err != nil {
				return nil, parseerr(`failed to add jwk.Key to set: %w`, err)
			}
			src = bytes.TrimSpace(rest)
		}
		return s, nil
	}

	if localReg != nil || ignoreParseError {
		dcKs, ok := s.(KeyWithDecodeCtx)
		if !ok {
			return nil, parseerr(`typed field was requested, but the key set (%T) does not support DecodeCtx`, s)
		}
		dc := &setDecodeCtx{
			DecodeCtx:        json.NewDecodeCtx(localReg),
			ignoreParseError: ignoreParseError,
		}
		dcKs.SetDecodeCtx(dc)
		defer func() { dcKs.SetDecodeCtx(nil) }()
	}

	if err := json.Unmarshal(src, s); err != nil {
		return nil, parseerr(`failed to unmarshal JWK set: %w`, err)
	}

	return s, nil
}

// ParseReader parses a JWK set from the incoming byte buffer.
func ParseReader(src io.Reader, options ...ParseOption) (Set, error) {
	// meh, there's no way to tell if a stream has "ended" a single
	// JWKs except when we encounter an EOF, so just... ReadAll
	buf, err := io.ReadAll(src)
	if err != nil {
		return nil, rparseerr(`failed to read from io.Reader: %w`, err)
	}

	set, err := Parse(buf, options...)
	if err != nil {
		return nil, rparseerr(`failed to parse reader: %w`, err)
	}
	return set, nil
}

// ParseString parses a JWK set from the incoming string.
func ParseString(s string, options ...ParseOption) (Set, error) {
	set, err := Parse([]byte(s), options...)
	if err != nil {
		return nil, sparseerr(`failed to parse string: %w`, err)
	}
	return set, nil
}

// AssignKeyID is a convenience function to automatically assign the "kid"
// section of the key, if it already doesn't have one. It uses Key.Thumbprint
// method with crypto.SHA256 as the default hashing algorithm
func AssignKeyID(key Key, options ...AssignKeyIDOption) error {
	if key.Has(KeyIDKey) {
		return nil
	}

	hash := crypto.SHA256
	for _, option := range options {
		switch option.Ident() {
		case identThumbprintHash{}:
			if err := option.Value(&hash); err != nil {
				return fmt.Errorf(`failed to retrieve thumbprint hash option value: %w`, err)
			}
		}
	}

	h, err := key.Thumbprint(hash)
	if err != nil {
		return fmt.Errorf(`failed to generate thumbprint: %w`, err)
	}

	if err := key.Set(KeyIDKey, base64.EncodeToString(h)); err != nil {
		return fmt.Errorf(`failed to set "kid": %w`, err)
	}

	return nil
}

// NOTE: may need to remove this to allow pluggale key types
func cloneKey(src Key) (Key, error) {
	var dst Key
	switch src.(type) {
	case RSAPrivateKey:
		dst = newRSAPrivateKey()
	case RSAPublicKey:
		dst = newRSAPublicKey()
	case ECDSAPrivateKey:
		dst = newECDSAPrivateKey()
	case ECDSAPublicKey:
		dst = newECDSAPublicKey()
	case OKPPrivateKey:
		dst = newOKPPrivateKey()
	case OKPPublicKey:
		dst = newOKPPublicKey()
	case SymmetricKey:
		dst = newSymmetricKey()
	default:
		return nil, fmt.Errorf(`jwk.cloneKey: unknown key type %T`, src)
	}

	for _, k := range src.Keys() {
		// It's absolutely
		var v any
		if err := src.Get(k, &v); err != nil {
			return nil, fmt.Errorf(`jwk.cloneKey: failed to get %q: %w`, k, err)
		}
		if err := dst.Set(k, v); err != nil {
			return nil, fmt.Errorf(`jwk.cloneKey: failed to set %q: %w`, k, err)
		}
	}
	return dst, nil
}

// Pem serializes the given jwk.Key in PEM encoded ASN.1 DER format,
// using either PKCS8 for private keys and PKIX for public keys.
// If you need to encode using PKCS1 or SEC1, you must do it yourself.
//
// # Argument must be of type jwk.Key or jwk.Set
//
// Currently only EC (including Ed25519) and RSA keys (and jwk.Set
// comprised of these key types) are supported.
func Pem(v any) ([]byte, error) {
	var set Set
	switch v := v.(type) {
	case Key:
		set = NewSet()
		if err := set.AddKey(v); err != nil {
			return nil, fmt.Errorf(`failed to add key to set: %w`, err)
		}
	case Set:
		set = v
	default:
		return nil, fmt.Errorf(`argument to Pem must be either jwk.Key or jwk.Set: %T`, v)
	}

	var ret []byte
	for i := range set.Len() {
		key, _ := set.Key(i)
		typ, buf, err := asnEncode(key)
		if err != nil {
			return nil, fmt.Errorf(`failed to encode content for key #%d: %w`, i, err)
		}

		var block pem.Block
		block.Type = typ
		block.Bytes = buf
		ret = append(ret, pem.EncodeToMemory(&block)...)
	}
	return ret, nil
}

func asnEncode(key Key) (string, []byte, error) {
	switch key := key.(type) {
	case ECDSAPrivateKey:
		var rawkey ecdsa.PrivateKey
		if err := Export(key, &rawkey); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key from jwk.Key: %w`, err)
		}
		buf, err := x509.MarshalECPrivateKey(&rawkey)
		if err != nil {
			return "", nil, fmt.Errorf(`failed to marshal PKCS8: %w`, err)
		}
		return pmECPrivateKey, buf, nil
	case RSAPrivateKey, OKPPrivateKey:
		var rawkey any
		if err := Export(key, &rawkey); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key from jwk.Key: %w`, err)
		}
		buf, err := x509.MarshalPKCS8PrivateKey(rawkey)
		if err != nil {
			return "", nil, fmt.Errorf(`failed to marshal PKCS8: %w`, err)
		}
		return pmPrivateKey, buf, nil
	case RSAPublicKey, ECDSAPublicKey, OKPPublicKey:
		var rawkey any
		if err := Export(key, &rawkey); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key from jwk.Key: %w`, err)
		}
		buf, err := x509.MarshalPKIXPublicKey(rawkey)
		if err != nil {
			return "", nil, fmt.Errorf(`failed to marshal PKIX: %w`, err)
		}
		return pmPublicKey, buf, nil
	default:
		return "", nil, fmt.Errorf(`unsupported key type %T`, key)
	}
}

type CustomDecoder = json.CustomDecoder
type CustomDecodeFunc = json.CustomDecodeFunc

// RegisterCustomField allows users to specify that a private field
// be decoded as an instance of the specified type. This option has
// a global effect.
//
// For example, suppose you have a custom field `x-birthday`, which
// you want to represent as a string formatted in RFC3339 in JSON,
// but want it back as `time.Time`.
//
// In such case you would register a custom field as follows
//
//	jwk.RegisterCustomField(`x-birthday`, time.Time{})
//
// Then you can use a `time.Time` variable to extract the value
// of `x-birthday` field, instead of having to use `any`
// and later convert it to `time.Time`
//
//	var bday time.Time
//	_ = key.Get(`x-birthday`, &bday)
//
// If you need a more fine-tuned control over the decoding process,
// you can register a `CustomDecoder`. For example, below shows
// how to register a decoder that can parse RFC1123 format string:
//
//	jwk.RegisterCustomField(`x-birthday`, jwk.CustomDecodeFunc(func(data []byte) (any, error) {
//	  return time.Parse(time.RFC1123, string(data))
//	}))
//
// Please note that use of custom fields can be problematic if you
// are using a library that does not implement MarshalJSON/UnmarshalJSON
// and you try to roundtrip from an object to JSON, and then back to an object.
// For example, in the above example, you can _parse_ time values formatted
// in the format specified in RFC822, but when you convert an object into
// JSON, it will be formatted in RFC3339, because that's what `time.Time`
// likes to do. To avoid this, it's always better to use a custom type
// that wraps your desired type (in this case `time.Time`) and implement
// MarshalJSON and UnmashalJSON.
func RegisterCustomField(name string, object any) {
	registry.Register(name, object)
}

// Equal compares two keys and returns true if they are equal. The comparison
// is solely done against the thumbprints of k1 and k2. It is possible for keys
// that have, for example, different key IDs, key usage, etc, to be considered equal.
func Equal(k1, k2 Key) bool {
	h := crypto.SHA256
	tp1, err := k1.Thumbprint(h)
	if err != nil {
		return false // can't report error
	}
	tp2, err := k2.Thumbprint(h)
	if err != nil {
		return false // can't report error
	}

	return bytes.Equal(tp1, tp2)
}

// IsPrivateKey returns true if the supplied key is a private key of an
// asymmetric key pair. The argument `k` must implement the `AsymmetricKey`
// interface.
//
// An error is returned if the supplied key is not an `AsymmetricKey`.
func IsPrivateKey(k Key) (bool, error) {
	asymmetric, ok := k.(AsymmetricKey)
	if ok {
		return asymmetric.IsPrivate(), nil
	}
	return false, fmt.Errorf("jwk.IsPrivateKey: %T is not an asymmetric key", k)
}

type keyValidationError struct {
	err error
}

func (e *keyValidationError) Error() string {
	return fmt.Sprintf(`key validation failed: %s`, e.err)
}

func (e *keyValidationError) Unwrap() error {
	return e.err
}

func (e *keyValidationError) Is(target error) bool {
	_, ok := target.(*keyValidationError)
	return ok
}

// NewKeyValidationError wraps the given error with an error that denotes
// `key.Validate()` has failed. This error type should ONLY be used as
// return value from the `Validate()` method.
func NewKeyValidationError(err error) error {
	return &keyValidationError{err: err}
}

func IsKeyValidationError(err error) bool {
	var kve keyValidationError
	return errors.Is(err, &kve)
}

// Configure is used to configure global behavior of the jwk package.
func Configure(options ...GlobalOption) {
	var strictKeyUsagePtr *bool
	for _, option := range options {
		switch option.Ident() {
		case identStrictKeyUsage{}:
			var v bool
			if err := option.Value(&v); err != nil {
				continue
			}
			strictKeyUsagePtr = &v
		}
	}

	if strictKeyUsagePtr != nil {
		strictKeyUsage.Store(*strictKeyUsagePtr)
	}
}

// These are used when validating keys.
type keyWithD interface {
	D() ([]byte, bool)
}

var _ keyWithD = &okpPrivateKey{}

func extractEmbeddedKey(keyif Key, concretTypes []reflect.Type) (Key, error) {
	rv := reflect.ValueOf(keyif)

	// If the value can be converted to one of the concrete types, then we're done
	for _, t := range concretTypes {
		if rv.Type().ConvertibleTo(t) {
			return keyif, nil
		}
	}

	// When a struct implements the Key interface via embedding, you unfortunately
	// cannot use a type switch to determine the concrete type, because
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, fmt.Errorf(`invalid key value (0): %w`, ContinueError())
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf(`invalid key value type %T (1): %w`, keyif, ContinueError())
	}
	if rv.NumField() == 0 {
		return nil, fmt.Errorf(`invalid key value type %T (2): %w`, keyif, ContinueError())
	}
	// Iterate through the fields of the struct to find the first field that
	// implements the Key interface
	rt := rv.Type()
	for i := range rv.NumField() {
		field := rv.Field(i)
		ft := rt.Field(i)
		if !ft.Anonymous {
			// We can only salvage this object if the object implements jwk.Key
			// via embedding, so we skip fields that are not anonymous
			continue
		}

		if field.CanInterface() {
			if k, ok := field.Interface().(Key); ok {
				return extractEmbeddedKey(k, concretTypes)
			}
		}
	}

	return nil, fmt.Errorf(`invalid key value type %T (3): %w`, keyif, ContinueError())
}
