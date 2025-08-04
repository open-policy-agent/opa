//go:generate ../tools/cmd/genjws.sh

// Package jws implements the digital signature on JSON based data
// structures as described in https://tools.ietf.org/html/rfc7515
//
// If you do not care about the details, the only things that you
// would need to use are the following functions:
//
//	jws.Sign(payload, jws.WithKey(algorithm, key))
//	jws.Verify(serialized, jws.WithKey(algorithm, key))
//
// To sign, simply use `jws.Sign`. `payload` is a []byte buffer that
// contains whatever data you want to sign. `alg` is one of the
// jwa.SignatureAlgorithm constants from package jwa. For RSA and
// ECDSA family of algorithms, you will need to prepare a private key.
// For HMAC family, you just need a []byte value. The `jws.Sign`
// function will return the encoded JWS message on success.
//
// To verify, use `jws.Verify`. It will parse the `encodedjws` buffer
// and verify the result using `algorithm` and `key`. Upon successful
// verification, the original payload is returned, so you can work on it.
package jws

import (
	"bufio"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/jwxio"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws/jwsbb"
)

var registry = json.NewRegistry()

var signers = make(map[jwa.SignatureAlgorithm]Signer)
var muSigner = &sync.Mutex{}

func removeSigner(alg jwa.SignatureAlgorithm) {
	muSigner.Lock()
	defer muSigner.Unlock()
	delete(signers, alg)
}

type defaultSigner struct {
	alg jwa.SignatureAlgorithm
}

func (s defaultSigner) Algorithm() jwa.SignatureAlgorithm {
	return s.alg
}

func (s defaultSigner) Sign(key any, payload []byte) ([]byte, error) {
	return jwsbb.Sign(key, s.alg.String(), payload, nil)
}

type signerAdapter struct {
	signer Signer
}

func (s signerAdapter) Algorithm() jwa.SignatureAlgorithm {
	return s.signer.Algorithm()
}

func (s signerAdapter) Sign(key any, payload []byte) ([]byte, error) {
	return s.signer.Sign(payload, key)
}

const (
	fmtInvalid = 1 << iota
	fmtCompact
	fmtJSON
	fmtJSONPretty
	fmtMax
)

// silence linters
var _ = fmtInvalid
var _ = fmtMax

func validateKeyBeforeUse(key any) error {
	jwkKey, ok := key.(jwk.Key)
	if !ok {
		converted, err := jwk.Import(key)
		if err != nil {
			return fmt.Errorf(`could not convert key of type %T to jwk.Key for validation: %w`, key, err)
		}
		jwkKey = converted
	}
	return jwkKey.Validate()
}

// Sign generates a JWS message for the given payload and returns
// it in serialized form, which can be in either compact or
// JSON format. Default is compact.
//
// You must pass at least one key to `jws.Sign()` by using `jws.WithKey()`
// option.
//
//	jws.Sign(payload, jws.WithKey(alg, key))
//	jws.Sign(payload, jws.WithJSON(), jws.WithKey(alg1, key1), jws.WithKey(alg2, key2))
//
// Note that in the second example the `jws.WithJSON()` option is
// specified as well. This is because the compact serialization
// format does not support multiple signatures, and users must
// specifically ask for the JSON serialization format.
//
// Read the documentation for `jws.WithKey()` to learn more about the
// possible values that can be used for `alg` and `key`.
//
// You may create JWS messages with the "none" (jwa.NoSignature) algorithm
// if you use the `jws.WithInsecureNoSignature()` option. This option
// can be combined with one or more signature keys, as well as the
// `jws.WithJSON()` option to generate multiple signatures (though
// the usefulness of such constructs is highly debatable)
//
// Note that this library does not allow you to successfully call `jws.Verify()` on
// signatures with the "none" algorithm. To parse these, use `jws.Parse()` instead.
//
// If you want to use a detached payload, use `jws.WithDetachedPayload()` as
// one of the options. When you use this option, you must always set the
// first parameter (`payload`) to `nil`, or the function will return an error
//
// You may also want to look at how to pass protected headers to the
// signing process, as you will likely be required to set the `b64` field
// when using detached payload.
//
// Look for options that return `jws.SignOption` or `jws.SignVerifyOption`
// for a complete list of options that can be passed to this function.
//
// You can use `errors.Is` with `jws.SignError()` to check if an error is from this function.
func Sign(payload []byte, options ...SignOption) ([]byte, error) {
	sc := signContextPool.Get()
	defer signContextPool.Put(sc)

	sc.payload = payload

	if err := sc.ProcessOptions(options); err != nil {
		return nil, signerr(`failed to process options: %w`, err)
	}

	lsigner := len(sc.sigbuilders)
	if lsigner == 0 {
		return nil, signerr(`no signers available. Specify an algorithm and a key using jws.WithKey()`)
	}

	// Design note: while we could have easily set format = fmtJSON when
	// lsigner > 1, I believe the decision to change serialization formats
	// must be explicitly stated by the caller. Otherwise, I'm pretty sure
	// there would be people filing issues saying "I get JSON when I expected
	// compact serialization".
	//
	// Therefore, instead of making implicit format conversions, we force the
	// user to spell it out as `jws.Sign(..., jws.WithJSON(), jws.WithKey(...), jws.WithKey(...))`
	if sc.format == fmtCompact && lsigner != 1 {
		return nil, signerr(`cannot have multiple signers (keys) specified for compact serialization. Use only one jws.WithKey()`)
	}

	// Create a Message object with all the bits and bobs, and we'll
	// serialize it in the end
	var result Message

	if err := sc.PopulateMessage(&result); err != nil {
		return nil, signerr(`failed to populate message: %w`, err)
	}
	switch sc.format {
	case fmtJSON:
		return json.Marshal(result)
	case fmtJSONPretty:
		return json.MarshalIndent(result, "", "  ")
	case fmtCompact:
		// Take the only signature object, and convert it into a Compact
		// serialization format
		var compactOpts []CompactOption
		if sc.detached {
			compactOpts = append(compactOpts, WithDetached(true))
		}
		for _, option := range options {
			if copt, ok := option.(CompactOption); ok {
				compactOpts = append(compactOpts, copt)
			}
		}
		return Compact(&result, compactOpts...)
	default:
		return nil, signerr(`invalid serialization format`)
	}
}

var allowNoneWhitelist = jwk.WhitelistFunc(func(string) bool {
	return false
})

// Verify checks if the given JWS message is verifiable using `alg` and `key`.
// `key` may be a "raw" key (e.g. rsa.PublicKey) or a jwk.Key
//
// If the verification is successful, `err` is nil, and the content of the
// payload that was signed is returned. If you need more fine-grained
// control of the verification process, manually generate a
// `Verifier` in `verify` subpackage, and call `Verify` method on it.
// If you need to access signatures and JOSE headers in a JWS message,
// use `Parse` function to get `Message` object.
//
// Because the use of "none" (jwa.NoSignature) algorithm is strongly discouraged,
// this function DOES NOT consider it a success when `{"alg":"none"}` is
// encountered in the message (it would also be counterintuitive when the code says
// it _verified_ something when in fact it did no such thing). If you want to
// accept messages with "none" signature algorithm, use `jws.Parse` to get the
// raw JWS message.
//
// The error returned by this function is of type can be checked against
// `jws.VerifyError()` and `jws.VerificationError()`. The latter is returned
// when the verification process itself fails (e.g. invalid signature, wrong key),
// while the former is returned when any other part of the `jws.Verify()`
// function fails.
func Verify(buf []byte, options ...VerifyOption) ([]byte, error) {
	vc := verifyContextPool.Get()
	defer verifyContextPool.Put(vc)

	if err := vc.ProcessOptions(options); err != nil {
		return nil, verifyerr(`failed to process options: %w`, err)
	}

	return vc.VerifyMessage(buf)
}

// get the value of b64 header field.
// If the field does not exist, returns true (default)
// Otherwise return the value specified by the header field.
func getB64Value(hdr Headers) bool {
	var b64 bool
	if err := hdr.Get("b64", &b64); err != nil {
		return true // default
	}

	return b64
}

// Parse parses contents from the given source and creates a jws.Message
// struct. By default the input can be in either compact or full JSON serialization.
//
// You may pass `jws.WithJSON()` and/or `jws.WithCompact()` to specify
// explicitly which format to use. If neither or both is specified, the function
// will attempt to autodetect the format. If one or the other is specified,
// only the specified format will be attempted.
//
// On error, returns a jws.ParseError.
func Parse(src []byte, options ...ParseOption) (*Message, error) {
	var formats int
	for _, option := range options {
		switch option.Ident() {
		case identSerialization{}:
			var v int
			if err := option.Value(&v); err != nil {
				return nil, parseerr(`failed to retrieve serialization option value: %w`, err)
			}
			switch v {
			case fmtJSON:
				formats |= fmtJSON
			case fmtCompact:
				formats |= fmtCompact
			}
		}
	}

	// if format is 0 or both JSON/Compact, auto detect
	if v := formats & (fmtJSON | fmtCompact); v == 0 || v == fmtJSON|fmtCompact {
	CHECKLOOP:
		for i := range src {
			r := rune(src[i])
			if r >= utf8.RuneSelf {
				r, _ = utf8.DecodeRune(src)
			}
			if !unicode.IsSpace(r) {
				if r == tokens.OpenCurlyBracket {
					formats = fmtJSON
				} else {
					formats = fmtCompact
				}
				break CHECKLOOP
			}
		}
	}

	if formats&fmtCompact == fmtCompact {
		msg, err := parseCompact(src)
		if err != nil {
			return nil, parseerr(`failed to parse compact format: %w`, err)
		}
		return msg, nil
	} else if formats&fmtJSON == fmtJSON {
		msg, err := parseJSON(src)
		if err != nil {
			return nil, parseerr(`failed to parse JSON format: %w`, err)
		}
		return msg, nil
	}

	return nil, parseerr(`invalid byte sequence`)
}

// ParseString parses contents from the given source and creates a jws.Message
// struct. The input can be in either compact or full JSON serialization.
//
// On error, returns a jws.ParseError.
func ParseString(src string) (*Message, error) {
	msg, err := Parse([]byte(src))
	if err != nil {
		return nil, sparseerr(`failed to parse string: %w`, err)
	}
	return msg, nil
}

// ParseReader parses contents from the given source and creates a jws.Message
// struct. The input can be in either compact or full JSON serialization.
//
// On error, returns a jws.ParseError.
func ParseReader(src io.Reader) (*Message, error) {
	data, err := jwxio.ReadAllFromFiniteSource(src)
	if err == nil {
		return Parse(data)
	}

	if !errors.Is(err, jwxio.NonFiniteSourceError()) {
		return nil, rparseerr(`failed to read from finite source: %w`, err)
	}

	rdr := bufio.NewReader(src)
	var first rune
	for {
		r, _, err := rdr.ReadRune()
		if err != nil {
			return nil, rparseerr(`failed to read rune: %w`, err)
		}
		if !unicode.IsSpace(r) {
			first = r
			if err := rdr.UnreadRune(); err != nil {
				return nil, rparseerr(`failed to unread rune: %w`, err)
			}

			break
		}
	}

	var parser func(io.Reader) (*Message, error)
	if first == tokens.OpenCurlyBracket {
		parser = parseJSONReader
	} else {
		parser = parseCompactReader
	}

	m, err := parser(rdr)
	if err != nil {
		return nil, rparseerr(`failed to parse reader: %w`, err)
	}

	return m, nil
}

func parseJSONReader(src io.Reader) (result *Message, err error) {
	var m Message
	if err := json.NewDecoder(src).Decode(&m); err != nil {
		return nil, fmt.Errorf(`failed to unmarshal jws message: %w`, err)
	}
	return &m, nil
}

func parseJSON(data []byte) (result *Message, err error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf(`failed to unmarshal jws message: %w`, err)
	}
	return &m, nil
}

// SplitCompact splits a JWS in compact format and returns its three parts
// separately: protected headers, payload and signature.
// On error, returns a jws.ParseError.
//
// This function will be deprecated in v4. It is a low-level API, and
// thus will be available in the `jwsbb` package.
func SplitCompact(src []byte) ([]byte, []byte, []byte, error) {
	hdr, payload, signature, err := jwsbb.SplitCompact(src)
	if err != nil {
		return nil, nil, nil, parseerr(`%w`, err)
	}
	return hdr, payload, signature, nil
}

// SplitCompactString splits a JWT and returns its three parts
// separately: protected headers, payload and signature.
// On error, returns a jws.ParseError.
//
// This function will be deprecated in v4. It is a low-level API, and
// thus will be available in the `jwsbb` package.
func SplitCompactString(src string) ([]byte, []byte, []byte, error) {
	hdr, payload, signature, err := jwsbb.SplitCompactString(src)
	if err != nil {
		return nil, nil, nil, parseerr(`%w`, err)
	}
	return hdr, payload, signature, nil
}

// SplitCompactReader splits a JWT and returns its three parts
// separately: protected headers, payload and signature.
// On error, returns a jws.ParseError.
//
// This function will be deprecated in v4. It is a low-level API, and
// thus will be available in the `jwsbb` package.
func SplitCompactReader(rdr io.Reader) ([]byte, []byte, []byte, error) {
	hdr, payload, signature, err := jwsbb.SplitCompactReader(rdr)
	if err != nil {
		return nil, nil, nil, parseerr(`%w`, err)
	}
	return hdr, payload, signature, nil
}

// parseCompactReader parses a JWS value serialized via compact serialization.
func parseCompactReader(rdr io.Reader) (m *Message, err error) {
	protected, payload, signature, err := SplitCompactReader(rdr)
	if err != nil {
		return nil, fmt.Errorf(`invalid compact serialization format: %w`, err)
	}
	return parse(protected, payload, signature)
}

func parseCompact(data []byte) (m *Message, err error) {
	protected, payload, signature, err := SplitCompact(data)
	if err != nil {
		return nil, fmt.Errorf(`invalid compact serialization format: %w`, err)
	}
	return parse(protected, payload, signature)
}

func parse(protected, payload, signature []byte) (*Message, error) {
	decodedHeader, err := base64.Decode(protected)
	if err != nil {
		return nil, fmt.Errorf(`failed to decode protected headers: %w`, err)
	}

	hdr := NewHeaders()
	if err := json.Unmarshal(decodedHeader, hdr); err != nil {
		return nil, fmt.Errorf(`failed to parse JOSE headers: %w`, err)
	}

	var decodedPayload []byte
	b64 := getB64Value(hdr)
	if !b64 {
		decodedPayload = payload
	} else {
		v, err := base64.Decode(payload)
		if err != nil {
			return nil, fmt.Errorf(`failed to decode payload: %w`, err)
		}
		decodedPayload = v
	}

	decodedSignature, err := base64.Decode(signature)
	if err != nil {
		return nil, fmt.Errorf(`failed to decode signature: %w`, err)
	}

	var msg Message
	msg.payload = decodedPayload
	msg.signatures = append(msg.signatures, &Signature{
		protected: hdr,
		signature: decodedSignature,
	})
	msg.b64 = b64
	return &msg, nil
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
//	jws.RegisterCustomField(`x-birthday`, time.Time{})
//
// Then you can use a `time.Time` variable to extract the value
// of `x-birthday` field, instead of having to use `any`
// and later convert it to `time.Time`
//
//	var bday time.Time
//	_ = hdr.Get(`x-birthday`, &bday)
//
// If you need a more fine-tuned control over the decoding process,
// you can register a `CustomDecoder`. For example, below shows
// how to register a decoder that can parse RFC1123 format string:
//
//	jws.RegisterCustomField(`x-birthday`, jws.CustomDecodeFunc(func(data []byte) (any, error) {
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

// Helpers for signature verification
var rawKeyToKeyType = make(map[reflect.Type]jwa.KeyType)
var keyTypeToAlgorithms = make(map[jwa.KeyType][]jwa.SignatureAlgorithm)

func init() {
	rawKeyToKeyType[reflect.TypeOf([]byte(nil))] = jwa.OctetSeq()
	rawKeyToKeyType[reflect.TypeOf(ed25519.PublicKey(nil))] = jwa.OKP()
	rawKeyToKeyType[reflect.TypeOf(rsa.PublicKey{})] = jwa.RSA()
	rawKeyToKeyType[reflect.TypeOf((*rsa.PublicKey)(nil))] = jwa.RSA()
	rawKeyToKeyType[reflect.TypeOf(ecdsa.PublicKey{})] = jwa.EC()
	rawKeyToKeyType[reflect.TypeOf((*ecdsa.PublicKey)(nil))] = jwa.EC()

	addAlgorithmForKeyType(jwa.OKP(), jwa.EdDSA())
	for _, alg := range []jwa.SignatureAlgorithm{jwa.HS256(), jwa.HS384(), jwa.HS512()} {
		addAlgorithmForKeyType(jwa.OctetSeq(), alg)
	}
	for _, alg := range []jwa.SignatureAlgorithm{jwa.RS256(), jwa.RS384(), jwa.RS512(), jwa.PS256(), jwa.PS384(), jwa.PS512()} {
		addAlgorithmForKeyType(jwa.RSA(), alg)
	}
	for _, alg := range []jwa.SignatureAlgorithm{jwa.ES256(), jwa.ES384(), jwa.ES512()} {
		addAlgorithmForKeyType(jwa.EC(), alg)
	}
}

func addAlgorithmForKeyType(kty jwa.KeyType, alg jwa.SignatureAlgorithm) {
	keyTypeToAlgorithms[kty] = append(keyTypeToAlgorithms[kty], alg)
}

// AlgorithmsForKey returns the possible signature algorithms that can
// be used for a given key. It only takes in consideration keys/algorithms
// for verification purposes, as this is the only usage where one may need
// dynamically figure out which method to use.
func AlgorithmsForKey(key any) ([]jwa.SignatureAlgorithm, error) {
	var kty jwa.KeyType
	switch key := key.(type) {
	case jwk.Key:
		kty = key.KeyType()
	case rsa.PublicKey, *rsa.PublicKey, rsa.PrivateKey, *rsa.PrivateKey:
		kty = jwa.RSA()
	case ecdsa.PublicKey, *ecdsa.PublicKey, ecdsa.PrivateKey, *ecdsa.PrivateKey:
		kty = jwa.EC()
	case ed25519.PublicKey, ed25519.PrivateKey, *ecdh.PublicKey, ecdh.PublicKey, *ecdh.PrivateKey, ecdh.PrivateKey:
		kty = jwa.OKP()
	case []byte:
		kty = jwa.OctetSeq()
	default:
		return nil, fmt.Errorf(`unknown key type %T`, key)
	}

	algs, ok := keyTypeToAlgorithms[kty]
	if !ok {
		return nil, fmt.Errorf(`unregistered key type %q`, kty)
	}
	return algs, nil
}

func Settings(options ...GlobalOption) {
	for _, option := range options {
		switch option.Ident() {
		case identLegacySigners{}:
			enableLegacySigners()
		}
	}
}

// VerifyCompactFast is a fast path verification function for JWS messages
// in compact serialization format.
//
// This function is considered experimental, and may change or be removed
// in the future.
//
// VerifyCompactFast performs signature verification on a JWS compact
// serialization without fully parsing the message into a jws.Message object.
// This makes it more efficient for cases where you only need to verify
// the signature and extract the payload, without needing access to headers
// or other JWS metadata.
//
// Returns the original payload that was signed if verification succeeds.
//
// Unlike jws.Verify(), this function requires you to specify the
// algorithm explicitly rather than extracting it from the JWS headers.
// This can be useful for performance-critical applications where the
// algorithm is known in advance.
//
// Since this function avoids doing many checks that jws.Verify would perform,
// you must ensure to perform the necessary checks including ensuring that algorithm is safe to use for your payload yourself.
func VerifyCompactFast(key any, compact []byte, alg jwa.SignatureAlgorithm) ([]byte, error) {
	algstr := alg.String()

	// Split the serialized JWT into its components
	hdr, payload, encodedSig, err := jwsbb.SplitCompact(compact)
	if err != nil {
		return nil, fmt.Errorf("jwt.verifyFast: failed to split compact: %w", err)
	}

	signature, err := base64.Decode(encodedSig)
	if err != nil {
		return nil, fmt.Errorf("jwt.verifyFast: failed to decode signature: %w", err)
	}

	// Instead of appending, copy the data from hdr/payload
	lvb := len(hdr) + 1 + len(payload)
	verifyBuf := pool.ByteSlice().GetCapacity(lvb)
	verifyBuf = verifyBuf[:lvb]
	copy(verifyBuf, hdr)
	verifyBuf[len(hdr)] = tokens.Period
	copy(verifyBuf[len(hdr)+1:], payload)
	defer pool.ByteSlice().Put(verifyBuf)

	// Verify the signature
	if verifier2, err := VerifierFor(alg); err == nil {
		if err := verifier2.Verify(key, verifyBuf, signature); err != nil {
			return nil, verifyError{verificationError{fmt.Errorf("jwt.VerifyCompact: signature verification failed for %s: %w", algstr, err)}}
		}
	} else {
		legacyVerifier, err := NewVerifier(alg)
		if err != nil {
			return nil, verifyerr("jwt.VerifyCompact: failed to create verifier for %s: %w", algstr, err)
		}
		if err := legacyVerifier.Verify(verifyBuf, signature, key); err != nil {
			return nil, verifyError{verificationError{fmt.Errorf("jwt.VerifyCompact: signature verification failed for %s: %w", algstr, err)}}
		}
	}

	decoded, err := base64.Decode(payload)
	if err != nil {
		return nil, verifyerr("jwt.VerifyCompact: failed to decode payload: %w", err)
	}
	return decoded, nil
}
