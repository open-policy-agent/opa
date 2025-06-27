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
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/option/v2"
)

var registry = json.NewRegistry()

type payloadSigner struct {
	signer    Signer
	key       interface{}
	protected Headers
	public    Headers
}

func (s *payloadSigner) Sign(payload []byte) ([]byte, error) {
	return s.signer.Sign(payload, s.key)
}

func (s *payloadSigner) Algorithm() jwa.SignatureAlgorithm {
	return s.signer.Algorithm()
}

func (s *payloadSigner) ProtectedHeader() Headers {
	return s.protected
}

func (s *payloadSigner) PublicHeader() Headers {
	return s.public
}

var signers = make(map[jwa.SignatureAlgorithm]Signer)
var muSigner = &sync.Mutex{}

func removeSigner(alg jwa.SignatureAlgorithm) {
	muSigner.Lock()
	defer muSigner.Unlock()
	delete(signers, alg)
}

func initSigner(ps *payloadSigner, alg jwa.SignatureAlgorithm, key interface{}, public, protected Headers) error {
	muSigner.Lock()
	signer, ok := signers[alg]
	if !ok {
		v, err := NewSigner(alg)
		if err != nil {
			muSigner.Unlock()
			return fmt.Errorf(`failed to create payload signer: %w`, err)
		}
		signers[alg] = v
		signer = v
	}
	muSigner.Unlock()

	ps.signer = signer
	ps.key = key
	ps.public = public
	ps.protected = protected
	return nil
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

func validateKeyBeforeUse(key interface{}) error {
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
func Sign(userPayload []byte, options ...SignOption) ([]byte, error) {
	signContext := signContextPool.Get()
	defer signContextPool.Put(signContext)

	// note: DO NOT use userPayload after this. signContext.payload
	// is the one that will be used for signing, as it may be modified
	signContext.payload = userPayload

	if err := signContext.ProcessOptions(options); err != nil {
		return nil, signerr(`failed to process options: %w`, err)
	}

	signed, err := signContext.Do()
	if err != nil {
		return nil, signerr("failed to sign payload: %w", err)
	}
	return signed, nil
}

type signContext struct {
	format         int
	detached       bool
	validateKey    bool
	encoder        Base64Encoder
	signers        []*payloadSigner
	noneSigner     *payloadSigner
	payload        []byte
	compactOptions *option.Set[CompactOption]
}

var signContextPool = pool.New(allocSignContext, destroySignContext)

func allocSignContext() interface{} {
	return &signContext{
		format:         fmtCompact,
		detached:       false,
		validateKey:    false,
		encoder:        base64.DefaultEncoder(),
		signers:        make([]*payloadSigner, 0, 1),
		noneSigner:     nil,
		payload:        nil,
		compactOptions: CompactOptionListPool().Get(),
	}
}

func destroySignContext(ctx *signContext) {
	ctx.format = fmtCompact
	ctx.detached = false
	ctx.validateKey = false
	ctx.encoder = base64.DefaultEncoder()
	for _, signer := range ctx.signers {
		payloadSignerPool.Put(signer)
	}
	ctx.signers = ctx.signers[:0] // clear the slice, but do not reallocate
	ctx.noneSigner = nil
	ctx.payload = nil
	ctx.compactOptions.Reset()
}

var msgPool = pool.New(allocMessage, destroyMessage)

func allocMessage() interface{} {
	return &Message{
		payload:    nil,
		signatures: make([]*Signature, 0, 1),
	}
}

func destroyMessage(msg *Message) {
	msg.payload = nil
	for _, sig := range msg.signatures {
		signaturePool.Put(sig)
	}
	msg.signatures = msg.signatures[:0] // clear the slice, but do not reallocate
	msg.clearRaw()                      // clear the raw buffer
	msg.b64 = false
}

var signaturePool = pool.New(allocSignature, destroySignature)

func allocSignature() interface{} {
	return &Signature{}
}

func destroySignature(sig *Signature) {
	sig.headers = nil
	sig.protected = nil
	sig.signature = nil
	sig.encoder = nil
	sig.detached = false
}

var payloadSignerPool = pool.New(allocPayloadSigner, destroyPayloadSigner)

func allocPayloadSigner() interface{} {
	return &payloadSigner{}
}

func destroyPayloadSigner(ps *payloadSigner) {
	ps.signer = nil
	ps.key = nil
	ps.protected = nil
	ps.public = nil
}

var errNoneSignature = errors.New(`"none" (jwa.NoSignature) cannot be used with jws.WithKey`)
var errNilPayloadRequiredWhenDetached = errors.New(`payload must be nil when jws.WithDetachedPayload() is specified`)
var errNoSignersAvailable = errors.New(`no signers available. Specify an alogirthm and akey using jws.WithKey()`)
var errMultipleSignersForCompactSerialization = errors.New(`cannot have multiple signers (keys) specified for compact serialization. Use only one jws.WithKey()`)
var errInvalidSerializationFormat = errors.New(`invalid serialization format`)

func (sc *signContext) ProcessOptions(options []SignOption) error {
	for _, option := range options {
		switch option.Ident() {
		case identSerialization{}:
			if err := option.Value(&sc.format); err != nil {
				return fmt.Errorf(`invalid value for serialization : %w`, err)
			}
		case identInsecureNoSignature{}:
			var data *withInsecureNoSignature
			if err := option.Value(&data); err != nil {
				return fmt.Errorf(`invalid value for WithInsecureNoSignature: %w`, err)
			}
			// only the last one is used (we overwrite previous values)
			signer := payloadSignerPool.Get()

			signer.signer = noneSigner{}
			signer.protected = data.protected
			sc.noneSigner = signer
		case identKey{}:
			var data withKey
			if err := option.Value(&data); err != nil {
				return fmt.Errorf(`invalid value for WithKey: %w`, err)
			}

			alg, ok := data.alg.(jwa.SignatureAlgorithm)
			if !ok {
				return fmt.Errorf(`expected algorithm to be of type jwa.SignatureAlgorithm but got (%[1]q, %[1]T)`, data.alg)
			}

			// No, we don't accept "none" here.
			if alg == jwa.NoSignature() {
				return errNoneSignature
			}

			signer := payloadSignerPool.Get()
			if err := initSigner(signer, alg, data.key, data.public, data.protected); err != nil {
				return fmt.Errorf(`failed to create signer: %w`, err)
			}
			sc.signers = append(sc.signers, signer)
		case identDetachedPayload{}:
			if sc.payload != nil {
				return errNilPayloadRequiredWhenDetached
			}
			if err := option.Value(&sc.payload); err != nil {
				return fmt.Errorf(`invalid value for WithDetachedPayload: %w`, err)
			}
			sc.compactOptions.Add(WithDetached(true))
			sc.detached = true
		case identValidateKey{}:
			if err := option.Value(&sc.validateKey); err != nil {
				return fmt.Errorf(`invalid value for WithValidateKey: %w`, err)
			}
		case identBase64Encoder{}:
			if err := option.Value(&sc.encoder); err != nil {
				return fmt.Errorf(`invalid value for WithBase64Encoder: %w`, err)
			}
			sc.compactOptions.Add(WithBase64Encoder(sc.encoder))
		default:
			if cop, ok := option.(CompactOption); ok {
				sc.compactOptions.Add(cop)
			}
		}
	}
	return nil
}

func (sc *signContext) canUseFastPath() bool {
	return len(sc.signers) == 1 && sc.format == fmtCompact && sc.payload != nil && !sc.detached
}

func (sc *signContext) Do() ([]byte, error) {
	if sc.noneSigner != nil {
		sc.signers = append(sc.signers, sc.noneSigner)
	}

	if sc.canUseFastPath() {
		signer := sc.signers[0]
		sig, err := sc.generateSignature(signer)
		if err != nil {
			return nil, fmt.Errorf(`failed to generate signature for signer #0 (alg=%s): %w`, signer.Algorithm(), err)
		}
		defer signaturePool.Put(sig)
		return compactSingle(sc.payload, sig, false, sc.encoder)
	}

	lsigner := len(sc.signers)
	if lsigner == 0 {
		return nil, errNoSignersAvailable
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
		return nil, errMultipleSignersForCompactSerialization
	}

	// Create a Message object with all the bits and bobs, and we'll
	// serialize it in the end
	result := msgPool.Get()
	defer msgPool.Put(result)

	result.payload = sc.payload
	for i, signer := range sc.signers {
		sig, err := sc.generateSignature(signer)
		if err != nil {
			return nil, fmt.Errorf(`failed to generate signature for signer #%d (alg=%s): %w`, i, signer.Algorithm(), err)
		}

		result.signatures = append(result.signatures, sig)
	}

	switch sc.format {
	case fmtJSON:
		return json.Marshal(result)
	case fmtJSONPretty:
		return json.MarshalIndent(result, "", "  ")
	case fmtCompact:
		return Compact(result, sc.compactOptions.List()...)
	default:
		return nil, errInvalidSerializationFormat
	}
}

func (sc *signContext) generateSignature(signer *payloadSigner) (*Signature, error) {
	protected := signer.ProtectedHeader()
	if protected == nil {
		protected = NewHeaders()
	}

	if err := protected.Set(AlgorithmKey, signer.Algorithm()); err != nil {
		return nil, fmt.Errorf(`failed to set "alg" header: %w`, err)
	}

	if key, ok := signer.key.(jwk.Key); ok {
		if kid, ok := key.KeyID(); ok && kid != "" {
			if err := protected.Set(KeyIDKey, kid); err != nil {
				return nil, fmt.Errorf(`failed to set "kid" header: %w`, err)
			}
		}
	}

	// releasing is done by the message
	sig := signaturePool.Get()

	sig.headers = signer.PublicHeader()
	sig.protected = protected
	sig.encoder = sc.encoder
	sig.detached = sc.detached

	if sc.validateKey {
		if err := validateKeyBeforeUse(signer.key); err != nil {
			return nil, fmt.Errorf(`failed to validate key before signing: %w`, err)
		}
	}
	_, _, err := sig.Sign(sc.payload, signer.signer, signer.key)
	if err != nil {
		signaturePool.Put(sig)
		return nil, err
	}
	return sig, nil
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
	var parseOptions []ParseOption
	var dst *Message
	var detachedPayload []byte
	var keyProviders []KeyProvider
	var keyUsed interface{}
	var validateKey bool
	var encoder Base64Encoder = base64.DefaultEncoder()

	ctx := context.Background()

	//nolint:forcetypeassert
	for _, option := range options {
		switch option.Ident() {
		case identMessage{}:
			if err := option.Value(&dst); err != nil {
				return nil, verifyerr(`invalid value for WithMessage: %w`, err)
			}
		case identDetachedPayload{}:
			if err := option.Value(&detachedPayload); err != nil {
				return nil, verifyerr(`invalid value for WithDetachedPayload: %w`, err)
			}
		case identKey{}:
			var pair withKey
			if err := option.Value(&pair); err != nil {
				return nil, verifyerr(`invalid value for WithKey: %w`, err)
			}
			alg, ok := pair.alg.(jwa.SignatureAlgorithm)
			if !ok {
				return nil, verifyerr(`WithKey() option must be specified using jwa.SignatureAlgorithm (got %T)`, pair.alg)
			}
			keyProviders = append(keyProviders, &staticKeyProvider{
				alg: alg,
				key: pair.key,
			})
		case identKeyProvider{}:
			var kp KeyProvider
			if err := option.Value(&kp); err != nil {
				return nil, verifyerr(`invalid value for WithKeyProvider: %w`, err)
			}
			keyProviders = append(keyProviders, kp)
		case identKeyUsed{}:
			if err := option.Value(&keyUsed); err != nil {
				return nil, verifyerr(`invalid value for WithKeyUsed: %w`, err)
			}
		case identContext{}:
			if err := option.Value(&ctx); err != nil {
				return nil, verifyerr(`invalid value for WithContext: %w`, err)
			}
		case identValidateKey{}:
			if err := option.Value(&validateKey); err != nil {
				return nil, verifyerr(`invalid value for WithValidateKey: %w`, err)
			}
		case identSerialization{}:
			parseOptions = append(parseOptions, option.(ParseOption))
		case identBase64Encoder{}:
			if err := option.Value(&encoder); err != nil {
				return nil, verifyerr(`invalid value for WithBase64Encoder: %w`, err)
			}
		default:
			return nil, verifyerr(`invalid jws.VerifyOption %q passed`, `With`+strings.TrimPrefix(fmt.Sprintf(`%T`, option.Ident()), `jws.ident`))
		}
	}

	if len(keyProviders) < 1 {
		return nil, verifyerr(`no key providers have been provided (see jws.WithKey(), jws.WithKeySet(), jws.WithVerifyAuto(), and jws.WithKeyProvider()`)
	}

	msg, err := Parse(buf, parseOptions...)
	if err != nil {
		return nil, verifyerr(`failed to parse jws: %w`, err)
	}
	defer msg.clearRaw()

	if detachedPayload != nil {
		if len(msg.payload) != 0 {
			return nil, verifyerr(`can't specify detached payload for JWS with payload`)
		}

		msg.payload = detachedPayload
	}

	// Pre-compute the base64 encoded version of payload
	var payload string
	if msg.b64 {
		payload = encoder.EncodeToString(msg.payload)
	} else {
		payload = string(msg.payload)
	}

	verifyBuf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(verifyBuf)

	var errs []error
	for i, sig := range msg.signatures {
		verifyBuf.Reset()

		var encodedProtectedHeader string
		if rbp, ok := sig.protected.(interface{ rawBuffer() []byte }); ok {
			if raw := rbp.rawBuffer(); raw != nil {
				encodedProtectedHeader = encoder.EncodeToString(raw)
			}
		}

		if encodedProtectedHeader == "" {
			protected, err := json.Marshal(sig.protected)
			if err != nil {
				return nil, verifyerr(`failed to marshal "protected" for signature #%d: %w`, i+1, err)
			}

			encodedProtectedHeader = encoder.EncodeToString(protected)
		}

		verifyBuf.WriteString(encodedProtectedHeader)
		verifyBuf.WriteByte(tokens.Period)
		verifyBuf.WriteString(payload)

		for i, kp := range keyProviders {
			var sink algKeySink
			if err := kp.FetchKeys(ctx, &sink, sig, msg); err != nil {
				return nil, verifyerr(`key provider %d failed: %w`, i, err)
			}

			for _, pair := range sink.list {
				// alg is converted here because pair.alg is of type jwa.KeyAlgorithm.
				// this may seem ugly, but we're trying to avoid declaring separate
				// structs for `alg jwa.KeyEncryptionAlgorithm` and `alg jwa.SignatureAlgorithm`
				//nolint:forcetypeassert
				alg := pair.alg.(jwa.SignatureAlgorithm)
				key := pair.key

				if validateKey {
					if err := validateKeyBeforeUse(key); err != nil {
						return nil, verifyerr(`failed to validate key before signing: %w`, err)
					}
				}
				verifier, err := NewVerifier(alg)
				if err != nil {
					return nil, verifyerr(`failed to create verifier for algorithm %q: %w`, alg, err)
				}

				if err := verifier.Verify(verifyBuf.Bytes(), sig.signature, key); err != nil {
					errs = append(errs, verificationError{err})
					continue
				}

				if keyUsed != nil {
					if err := blackmagic.AssignIfCompatible(keyUsed, key); err != nil {
						return nil, verifyerr(`failed to assign used key (%T) to %T: %w`, key, keyUsed, err)
					}
				}

				if dst != nil {
					*(dst) = *msg
				}

				return msg.payload, nil
			}
		}
	}
	return nil, verifyerr(`could not verify message using any of the signatures or keys: %w`, errors.Join(errs...))
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

// This is an "optimized" io.ReadAll(). It will attempt to read
// all of the contents from the reader IF the reader is of a certain
// concrete type.
func readAll(rdr io.Reader) ([]byte, bool) {
	switch rdr.(type) {
	case *bytes.Reader, *bytes.Buffer, *strings.Reader:
		data, err := io.ReadAll(rdr)
		if err != nil {
			return nil, false
		}
		return data, true
	default:
		return nil, false
	}
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
				return nil, parseerr(`invalid value for serialization: %w`, err)
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
	if data, ok := readAll(src); ok {
		return Parse(data)
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

const tokenDelim = "."

// SplitCompact splits a JWT and returns its three parts
// separately: protected headers, payload and signature.
//
// On error, returns a jws.ParseError.
func SplitCompact(src []byte) ([]byte, []byte, []byte, error) {
	protected, s, ok := bytes.Cut(src, []byte(tokenDelim))
	if !ok { // no period found
		return nil, nil, nil, parseerr(`invalid number of segments`)
	}
	payload, s, ok := bytes.Cut(s, []byte(tokenDelim))
	if !ok { // only one period found
		return nil, nil, nil, parseerr(`invalid number of segments`)
	}
	signature, _, ok := bytes.Cut(s, []byte(tokenDelim))
	if ok { // three periods found
		return nil, nil, nil, parseerr(`invalid number of segments`)
	}
	return protected, payload, signature, nil
}

// SplitCompactString splits a JWT and returns its three parts
// separately: protected headers, payload and signature.
//
// On error, returns a jws.ParseError.
func SplitCompactString(src string) ([]byte, []byte, []byte, error) {
	return SplitCompact([]byte(src))
}

// SplitCompactReader splits a JWT and returns its three parts
// separately: protected headers, payload and signature.
//
// On error, returns a jws.ParseError.
func SplitCompactReader(rdr io.Reader) ([]byte, []byte, []byte, error) {
	if data, ok := readAll(rdr); ok {
		return SplitCompact(data)
	}

	var protected []byte
	var payload []byte
	var signature []byte
	var periods int
	var state int

	buf := make([]byte, 4096)
	var sofar []byte

	for {
		// read next bytes
		n, err := rdr.Read(buf)
		// return on unexpected read error
		if err != nil && err != io.EOF {
			return nil, nil, nil, parseerr(`unexpected end of input: %w`, err)
		}

		// append to current buffer
		sofar = append(sofar, buf[:n]...)
		// loop to capture multiple tokens.Period in current buffer
		for loop := true; loop; {
			var i = bytes.IndexByte(sofar, tokens.Period)
			if i == -1 && err != io.EOF {
				// no tokens.Period found -> exit and read next bytes (outer loop)
				loop = false
				continue
			} else if i == -1 && err == io.EOF {
				// no tokens.Period found -> process rest and exit
				i = len(sofar)
				loop = false
			} else {
				// tokens.Period found
				periods++
			}

			// Reaching this point means we have found a tokens.Period or EOF and process the rest of the buffer
			switch state {
			case 0:
				protected = sofar[:i]
				state++
			case 1:
				payload = sofar[:i]
				state++
			case 2:
				signature = sofar[:i]
			}
			// Shorten current buffer
			if len(sofar) > i {
				sofar = sofar[i+1:]
			}
		}
		// Exit on EOF
		if err == io.EOF {
			break
		}
	}
	if periods != 2 {
		return nil, nil, nil, parseerr(`invalid number of segments`)
	}

	return protected, payload, signature, nil
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
// of `x-birthday` field, instead of having to use `interface{}`
// and later convert it to `time.Time`
//
//	var bday time.Time
//	_ = hdr.Get(`x-birthday`, &bday)
//
// If you need a more fine-tuned control over the decoding process,
// you can register a `CustomDecoder`. For example, below shows
// how to register a decoder that can parse RFC1123 format string:
//
//	jws.RegisterCustomField(`x-birthday`, jws.CustomDecodeFunc(func(data []byte) (interface{}, error) {
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
func RegisterCustomField(name string, object interface{}) {
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
func AlgorithmsForKey(key interface{}) ([]jwa.SignatureAlgorithm, error) {
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

// Because the keys defined in github.com/lestrrat-go/jwx/jwk may also implement
// crypto.Signer, it would be possible for to mix up key types when signing/verifying
// for example, when we specify jws.WithKey(jwa.RSA256, cryptoSigner), the cryptoSigner
// can be for RSA, or any other type that implements crypto.Signer... even if it's for the
// wrong algorithm.
//
// These functions are there to differentiate between the valid KNOWN key types.
// For any other key type that is outside of the Go std library and our own code,
// we must rely on the user to be vigilant.
//
// Notes: symmetric keys are obviously not part of this. for v2 OKP keys,
// x25519 does not implement Sign()
func isValidRSAKey(key interface{}) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		ed25519.PrivateKey,
		jwk.ECDSAPrivateKey, jwk.OKPPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}

func isValidECDSAKey(key interface{}) bool {
	switch key.(type) {
	case
		ed25519.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey,
		jwk.RSAPrivateKey, jwk.OKPPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}

func isValidEDDSAKey(key interface{}) bool {
	switch key.(type) {
	case
		ecdsa.PrivateKey, *ecdsa.PrivateKey,
		rsa.PrivateKey, *rsa.PrivateKey,
		jwk.RSAPrivateKey, jwk.ECDSAPrivateKey:
		// these are NOT ok
		return false
	}
	return true
}
