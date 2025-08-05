//go:generate ../tools/cmd/genjwt.sh
//go:generate stringer -type=TokenOption -output=token_options_gen.go

// Package jwt implements JSON Web Tokens as described in https://tools.ietf.org/html/rfc7519
package jwt

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/lestrrat-go/jwx/v3"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	jwterrs "github.com/lestrrat-go/jwx/v3/jwt/internal/errors"
	"github.com/lestrrat-go/jwx/v3/jwt/internal/types"
)

var defaultTruncation atomic.Int64

// Settings controls global settings that are specific to JWTs.
func Settings(options ...GlobalOption) {
	var flattenAudience bool
	var parsePedantic bool
	var parsePrecision = types.MaxPrecision + 1  // illegal value, so we can detect nothing was set
	var formatPrecision = types.MaxPrecision + 1 // illegal value, so we can detect nothing was set
	truncation := time.Duration(-1)
	for _, option := range options {
		switch option.Ident() {
		case identTruncation{}:
			if err := option.Value(&truncation); err != nil {
				panic(fmt.Sprintf("jwt.Settings: value for WithTruncation must be time.Duration: %s", err))
			}
		case identFlattenAudience{}:
			if err := option.Value(&flattenAudience); err != nil {
				panic(fmt.Sprintf("jwt.Settings: value for WithFlattenAudience must be bool: %s", err))
			}
		case identNumericDateParsePedantic{}:
			if err := option.Value(&parsePedantic); err != nil {
				panic(fmt.Sprintf("jwt.Settings: value for WithNumericDateParsePedantic must be bool: %s", err))
			}
		case identNumericDateParsePrecision{}:
			var v int
			if err := option.Value(&v); err != nil {
				panic(fmt.Sprintf("jwt.Settings: value for WithNumericDateParsePrecision must be int: %s", err))
			}
			// only accept this value if it's in our desired range
			if v >= 0 && v <= int(types.MaxPrecision) {
				parsePrecision = uint32(v)
			}
		case identNumericDateFormatPrecision{}:
			var v int
			if err := option.Value(&v); err != nil {
				panic(fmt.Sprintf("jwt.Settings: value for WithNumericDateFormatPrecision must be int: %s", err))
			}
			// only accept this value if it's in our desired range
			if v >= 0 && v <= int(types.MaxPrecision) {
				formatPrecision = uint32(v)
			}
		}
	}

	if parsePrecision <= types.MaxPrecision { // remember we set default to max + 1
		v := atomic.LoadUint32(&types.ParsePrecision)
		if v != parsePrecision {
			atomic.CompareAndSwapUint32(&types.ParsePrecision, v, parsePrecision)
		}
	}

	if formatPrecision <= types.MaxPrecision { // remember we set default to max + 1
		v := atomic.LoadUint32(&types.FormatPrecision)
		if v != formatPrecision {
			atomic.CompareAndSwapUint32(&types.FormatPrecision, v, formatPrecision)
		}
	}

	{
		v := atomic.LoadUint32(&types.Pedantic)
		if (v == 1) != parsePedantic {
			var newVal uint32
			if parsePedantic {
				newVal = 1
			}
			atomic.CompareAndSwapUint32(&types.Pedantic, v, newVal)
		}
	}

	{
		defaultOptionsMu.Lock()
		if flattenAudience {
			defaultOptions.Enable(FlattenAudience)
		} else {
			defaultOptions.Disable(FlattenAudience)
		}
		defaultOptionsMu.Unlock()
	}

	if truncation >= 0 {
		defaultTruncation.Store(int64(truncation))
	}
}

var registry = json.NewRegistry()

// ParseString calls Parse against a string
func ParseString(s string, options ...ParseOption) (Token, error) {
	tok, err := parseBytes([]byte(s), options...)
	if err != nil {
		return nil, jwterrs.ParseErrorf(`jwt.ParseString`, `failed to parse string: %w`, err)
	}
	return tok, nil
}

// Parse parses the JWT token payload and creates a new `jwt.Token` object.
// The token must be encoded in JWS compact format, or a raw JSON form of JWT
// without any signatures.
//
// If you need JWE support on top of JWS, you will need to rollout your
// own workaround.
//
// If the token is signed, and you want to verify the payload matches the signature,
// you must pass the jwt.WithKey(alg, key) or jwt.WithKeySet(jwk.Set) option.
// If you do not specify these parameters, no verification will be performed.
//
// During verification, if the JWS headers specify a key ID (`kid`), the
// key used for verification must match the specified ID. If you are somehow
// using a key without a `kid` (which is highly unlikely if you are working
// with a JWT from a well-know provider), you can work around this by modifying
// the `jwk.Key` and setting the `kid` header.
//
// If you also want to assert the validity of the JWT itself (i.e. expiration
// and such), use the `Validate()` function on the returned token, or pass the
// `WithValidate(true)` option. Validate options can also be passed to
// `Parse`
//
// This function takes both ParseOption and ValidateOption types:
// ParseOptions control the parsing behavior, and ValidateOptions are
// passed to `Validate()` when `jwt.WithValidate` is specified.
func Parse(s []byte, options ...ParseOption) (Token, error) {
	tok, err := parseBytes(s, options...)
	if err != nil {
		return nil, jwterrs.ParseErrorf(`jwt.Parse`, `failed to parse token: %w`, err)
	}
	return tok, nil
}

// ParseInsecure is exactly the same as Parse(), but it disables
// signature verification and token validation.
//
// You cannot override `jwt.WithVerify()` or `jwt.WithValidate()`
// using this function. Providing these options would result in
// an error
func ParseInsecure(s []byte, options ...ParseOption) (Token, error) {
	for _, option := range options {
		switch option.Ident() {
		case identVerify{}, identValidate{}:
			return nil, jwterrs.ParseErrorf(`jwt.ParseInsecure`, `jwt.WithVerify() and jwt.WithValidate() may not be specified`)
		}
	}

	options = append(options, WithVerify(false), WithValidate(false))
	tok, err := Parse(s, options...)
	if err != nil {
		return nil, jwterrs.ParseErrorf(`jwt.ParseInsecure`, `failed to parse token: %w`, err)
	}
	return tok, nil
}

// ParseReader calls Parse against an io.Reader
func ParseReader(src io.Reader, options ...ParseOption) (Token, error) {
	// We're going to need the raw bytes regardless. Read it.
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, jwterrs.ParseErrorf(`jwt.ParseReader`, `failed to read from token data source: %w`, err)
	}
	tok, err := parseBytes(data, options...)
	if err != nil {
		return nil, jwterrs.ParseErrorf(`jwt.ParseReader`, `failed to parse token: %w`, err)
	}
	return tok, nil
}

type parseCtx struct {
	token            Token
	validateOpts     []ValidateOption
	verifyOpts       []jws.VerifyOption
	localReg         *json.Registry
	pedantic         bool
	skipVerification bool
	validate         bool
	withKeyCount     int
	withKey          *withKey // this is used to detect if we have a WithKey option
}

func parseBytes(data []byte, options ...ParseOption) (Token, error) {
	var ctx parseCtx

	// Validation is turned on by default. You need to specify
	// jwt.WithValidate(false) if you want to disable it
	ctx.validate = true

	// Verification is required (i.e., it is assumed that the incoming
	// data is in JWS format) unless the user explicitly asks for
	// it to be skipped.
	verification := true

	var verifyOpts []Option
	for _, o := range options {
		if v, ok := o.(ValidateOption); ok {
			ctx.validateOpts = append(ctx.validateOpts, v)
			continue
		}

		switch o.Ident() {
		case identKey{}:
			// it would be nice to be able to detect if ctx.verifyOpts[0]
			// is a WithKey option, but unfortunately at that point we have
			// already converted the options to a jws option, which means
			// we can no longer compare its Ident() to jwt.identKey{}.
			// So let's just count this here
			ctx.withKeyCount++
			if ctx.withKeyCount == 1 {
				if err := o.Value(&ctx.withKey); err != nil {
					return nil, fmt.Errorf("jws.parseBytes: value for WithKey option must be a *jwt.withKey: %w", err)
				}
			}
			verifyOpts = append(verifyOpts, o)
		case identKeySet{}, identVerifyAuto{}, identKeyProvider{}, identBase64Encoder{}:
			verifyOpts = append(verifyOpts, o)
		case identToken{}:
			var token Token
			if err := o.Value(&token); err != nil {
				return nil, fmt.Errorf("jws.parseBytes: value for WithToken option must be a jwt.Token: %w", err)
			}
			ctx.token = token
		case identPedantic{}:
			if err := o.Value(&ctx.pedantic); err != nil {
				return nil, fmt.Errorf("jws.parseBytes: value for WithPedantic option must be a bool: %w", err)
			}
		case identValidate{}:
			if err := o.Value(&ctx.validate); err != nil {
				return nil, fmt.Errorf("jws.parseBytes: value for WithValidate option must be a bool: %w", err)
			}
		case identVerify{}:
			if err := o.Value(&verification); err != nil {
				return nil, fmt.Errorf("jws.parseBytes: value for WithVerify option must be a bool: %w", err)
			}
		case identTypedClaim{}:
			var pair claimPair
			if err := o.Value(&pair); err != nil {
				return nil, fmt.Errorf("jws.parseBytes: value for WithTypedClaim option must be claimPair: %w", err)
			}
			if ctx.localReg == nil {
				ctx.localReg = json.NewRegistry()
			}
			ctx.localReg.Register(pair.Name, pair.Value)
		}
	}

	if !verification {
		ctx.skipVerification = true
	}

	lvo := len(verifyOpts)
	if lvo == 0 && verification {
		return nil, fmt.Errorf(`jwt.Parse: no keys for verification are provided (use jwt.WithVerify(false) to explicitly skip)`)
	}

	if lvo > 0 {
		converted, err := toVerifyOptions(verifyOpts...)
		if err != nil {
			return nil, fmt.Errorf(`jwt.Parse: failed to convert options into jws.VerifyOption: %w`, err)
		}
		ctx.verifyOpts = converted
	}

	data = bytes.TrimSpace(data)
	return parse(&ctx, data)
}

const (
	_JwsVerifyInvalid = iota
	_JwsVerifyDone
	_JwsVerifyExpectNested
	_JwsVerifySkipped
)

var _ = _JwsVerifyInvalid

func verifyJWS(ctx *parseCtx, payload []byte) ([]byte, int, error) {
	lvo := len(ctx.verifyOpts)
	if lvo == 0 {
		return nil, _JwsVerifySkipped, nil
	}

	if lvo == 1 && ctx.withKeyCount == 1 {
		wk := ctx.withKey
		alg, ok := wk.alg.(jwa.SignatureAlgorithm)
		if ok && len(wk.options) == 0 {
			verified, err := jws.VerifyCompactFast(wk.key, payload, alg)
			if err != nil {
				return nil, _JwsVerifyDone, err
			}
			return verified, _JwsVerifyDone, nil
		}
	}

	verifyOpts := append(ctx.verifyOpts, jws.WithCompact())
	verified, err := jws.Verify(payload, verifyOpts...)
	return verified, _JwsVerifyDone, err
}

// verify parameter exists to make sure that we don't accidentally skip
// over verification just because alg == ""  or key == nil or something.
func parse(ctx *parseCtx, data []byte) (Token, error) {
	payload := data
	const maxDecodeLevels = 2

	// If cty = `JWT`, we expect this to be a nested structure
	var expectNested bool

OUTER:
	for i := range maxDecodeLevels {
		switch kind := jwx.GuessFormat(payload); kind {
		case jwx.JWT:
			if ctx.pedantic {
				if expectNested {
					return nil, fmt.Errorf(`expected nested encrypted/signed payload, got raw JWT`)
				}
			}

			if i == 0 {
				// We were NOT enveloped in other formats
				if !ctx.skipVerification {
					if _, _, err := verifyJWS(ctx, payload); err != nil {
						return nil, err
					}
				}
			}

			break OUTER
		case jwx.InvalidFormat:
			return nil, UnknownPayloadTypeError()
		case jwx.UnknownFormat:
			// "Unknown" may include invalid JWTs, for example, those who lack "aud"
			// claim. We could be pedantic and reject these
			if ctx.pedantic {
				return nil, fmt.Errorf(`unknown JWT format (pedantic)`)
			}

			if i == 0 {
				// We were NOT enveloped in other formats
				if !ctx.skipVerification {
					if _, _, err := verifyJWS(ctx, payload); err != nil {
						return nil, err
					}
				}
			}
			break OUTER
		case jwx.JWS:
			// Food for thought: This is going to break if you have multiple layers of
			// JWS enveloping using different keys. It is highly unlikely use case,
			// but it might happen.

			// skipVerification should only be set to true by us. It's used
			// when we just want to parse the JWT out of a payload
			if !ctx.skipVerification {
				// nested return value means:
				// false (next envelope _may_ need to be processed)
				// true (next envelope MUST be processed)
				v, state, err := verifyJWS(ctx, payload)
				if err != nil {
					return nil, err
				}

				if state != _JwsVerifySkipped {
					payload = v

					// We only check for cty and typ if the pedantic flag is enabled
					if !ctx.pedantic {
						continue
					}

					if state == _JwsVerifyExpectNested {
						expectNested = true
						continue OUTER
					}

					// if we're not nested, we found our target. bail out of this loop
					break OUTER
				}
			}

			// No verification.
			m, err := jws.Parse(data, jws.WithCompact())
			if err != nil {
				return nil, fmt.Errorf(`invalid jws message: %w`, err)
			}
			payload = m.Payload()
		default:
			return nil, fmt.Errorf(`unsupported format (layer: #%d)`, i+1)
		}
		expectNested = false
	}

	if ctx.token == nil {
		ctx.token = New()
	}

	if ctx.localReg != nil {
		dcToken, ok := ctx.token.(TokenWithDecodeCtx)
		if !ok {
			return nil, fmt.Errorf(`typed claim was requested, but the token (%T) does not support DecodeCtx`, ctx.token)
		}
		dc := json.NewDecodeCtx(ctx.localReg)
		dcToken.SetDecodeCtx(dc)
		defer func() { dcToken.SetDecodeCtx(nil) }()
	}

	if err := json.Unmarshal(payload, ctx.token); err != nil {
		return nil, fmt.Errorf(`failed to parse token: %w`, err)
	}

	if ctx.validate {
		if err := Validate(ctx.token, ctx.validateOpts...); err != nil {
			return nil, err
		}
	}
	return ctx.token, nil
}

// Sign is a convenience function to create a signed JWT token serialized in
// compact form.
//
// It accepts either a raw key (e.g. rsa.PrivateKey, ecdsa.PrivateKey, etc)
// or a jwk.Key, and the name of the algorithm that should be used to sign
// the token.
//
// For well-known algorithms with no special considerations (e.g. detached
// payloads, extra protected heders, etc), this function will automatically
// take the fast path and bypass the jws.Sign() machinery, which improves
// performance significantly.
//
// If the key is a jwk.Key and the key contains a key ID (`kid` field),
// then it is added to the protected header generated by the signature
//
// The algorithm specified in the `alg` parameter must be able to support
// the type of key you provided, otherwise an error is returned.
// For convenience `alg` is of type jwa.KeyAlgorithm so you can pass
// the return value of `(jwk.Key).Algorithm()` directly, but in practice
// it must be an instance of jwa.SignatureAlgorithm, otherwise an error
// is returned.
//
// The protected header will also automatically have the `typ` field set
// to the literal value `JWT`, unless you provide a custom value for it
// by jws.WithProtectedHeaders option, that can be passed to `jwt.WithKey“.
func Sign(t Token, options ...SignOption) ([]byte, error) {
	// fast path; can only happen if there is exactly one option
	if len(options) == 1 && (options[0].Ident() == identKey{}) {
		// The option must be a withKey option.
		var wk *withKey
		if err := options[0].Value(&wk); err == nil {
			alg, ok := wk.alg.(jwa.SignatureAlgorithm)
			if !ok {
				return nil, fmt.Errorf(`jwt.Sign: invalid algorithm type %T. jwa.SignatureAlgorithm is required`, wk.alg)
			}

			// Check if option contains anything other than alg/key
			if len(wk.options) == 0 {
				// yay, we have something we can put in the FAST PATH!
				return signFast(t, alg, wk.key)
			}
			// fallthrough
		}
		// fallthrough
	}

	var soptions []jws.SignOption
	if l := len(options); l > 0 {
		// we need to from SignOption to Option because ... reasons
		// (todo: when go1.18 prevails, use type parameters
		rawoptions := make([]Option, l)
		for i, option := range options {
			rawoptions[i] = option
		}

		converted, err := toSignOptions(rawoptions...)
		if err != nil {
			return nil, fmt.Errorf(`jwt.Sign: failed to convert options into jws.SignOption: %w`, err)
		}
		soptions = converted
	}
	return NewSerializer().sign(soptions...).Serialize(t)
}

// Equal compares two JWT tokens. Do not use `reflect.Equal` or the like
// to compare tokens as they will also compare extra detail such as
// sync.Mutex objects used to control concurrent access.
//
// The comparison for values is currently done using a simple equality ("=="),
// except for time.Time, which uses time.Equal after dropping the monotonic
// clock and truncating the values to 1 second accuracy.
//
// if both t1 and t2 are nil, returns true
func Equal(t1, t2 Token) bool {
	if t1 == nil && t2 == nil {
		return true
	}

	// we already checked for t1 == t2 == nil, so safe to do this
	if t1 == nil || t2 == nil {
		return false
	}

	j1, err := json.Marshal(t1)
	if err != nil {
		return false
	}

	j2, err := json.Marshal(t2)
	if err != nil {
		return false
	}

	return bytes.Equal(j1, j2)
}

func (t *stdToken) Clone() (Token, error) {
	dst := New()

	dst.Options().Set(*(t.Options()))
	for _, k := range t.Keys() {
		var v any
		if err := t.Get(k, &v); err != nil {
			return nil, fmt.Errorf(`jwt.Clone: failed to get %s: %w`, k, err)
		}
		if err := dst.Set(k, v); err != nil {
			return nil, fmt.Errorf(`jwt.Clone failed to set %s: %w`, k, err)
		}
	}
	return dst, nil
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
//	jwt.RegisterCustomField(`x-birthday`, time.Time{})
//
// Then you can use a `time.Time` variable to extract the value
// of `x-birthday` field, instead of having to use `any`
// and later convert it to `time.Time`
//
//	var bday time.Time
//	_ = token.Get(`x-birthday`, &bday)
//
// If you need a more fine-tuned control over the decoding process,
// you can register a `CustomDecoder`. For example, below shows
// how to register a decoder that can parse RFC822 format string:
//
//	jwt.RegisterCustomField(`x-birthday`, jwt.CustomDecodeFunc(func(data []byte) (any, error) {
//	  return time.Parse(time.RFC822, string(data))
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

func getDefaultTruncation() time.Duration {
	return time.Duration(defaultTruncation.Load())
}
