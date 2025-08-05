package jwt

import (
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/option/v2"
)

type identInsecureNoSignature struct{}
type identKey struct{}
type identKeySet struct{}
type identTypedClaim struct{}
type identVerifyAuto struct{}

func toSignOptions(options ...Option) ([]jws.SignOption, error) {
	soptions := make([]jws.SignOption, 0, len(options))
	for _, option := range options {
		switch option.Ident() {
		case identInsecureNoSignature{}:
			soptions = append(soptions, jws.WithInsecureNoSignature())
		case identKey{}:
			var wk withKey
			if err := option.Value(&wk); err != nil {
				return nil, fmt.Errorf(`toSignOtpions: failed to convert option value to withKey: %w`, err)
			}
			var wksoptions []jws.WithKeySuboption
			for _, subopt := range wk.options {
				wksopt, ok := subopt.(jws.WithKeySuboption)
				if !ok {
					return nil, fmt.Errorf(`expected optional arguments in jwt.WithKey to be jws.WithKeySuboption, but got %T`, subopt)
				}
				wksoptions = append(wksoptions, wksopt)
			}

			soptions = append(soptions, jws.WithKey(wk.alg, wk.key, wksoptions...))
		case identSignOption{}:
			var sigOpt jws.SignOption
			if err := option.Value(&sigOpt); err != nil {
				return nil, fmt.Errorf(`failed to decode SignOption: %w`, err)
			}
			soptions = append(soptions, sigOpt)
		case identBase64Encoder{}:
			var enc jws.Base64Encoder
			if err := option.Value(&enc); err != nil {
				return nil, fmt.Errorf(`failed to decode Base64Encoder: %w`, err)
			}
			soptions = append(soptions, jws.WithBase64Encoder(enc))
		}
	}
	return soptions, nil
}

func toEncryptOptions(options ...Option) ([]jwe.EncryptOption, error) {
	soptions := make([]jwe.EncryptOption, 0, len(options))
	for _, option := range options {
		switch option.Ident() {
		case identKey{}:
			var wk withKey
			if err := option.Value(&wk); err != nil {
				return nil, fmt.Errorf(`toEncryptOptions: failed to convert option value to withKey: %w`, err)
			}
			var wksoptions []jwe.WithKeySuboption
			for _, subopt := range wk.options {
				wksopt, ok := subopt.(jwe.WithKeySuboption)
				if !ok {
					return nil, fmt.Errorf(`expected optional arguments in jwt.WithKey to be jwe.WithKeySuboption, but got %T`, subopt)
				}
				wksoptions = append(wksoptions, wksopt)
			}

			soptions = append(soptions, jwe.WithKey(wk.alg, wk.key, wksoptions...))
		case identEncryptOption{}:
			var encOpt jwe.EncryptOption
			if err := option.Value(&encOpt); err != nil {
				return nil, fmt.Errorf(`failed to decode EncryptOption: %w`, err)
			}
			soptions = append(soptions, encOpt)
		}
	}
	return soptions, nil
}

func toVerifyOptions(options ...Option) ([]jws.VerifyOption, error) {
	voptions := make([]jws.VerifyOption, 0, len(options))
	for _, option := range options {
		switch option.Ident() {
		case identKey{}:
			var wk withKey
			if err := option.Value(&wk); err != nil {
				return nil, fmt.Errorf(`toVerifyOptions: failed to convert option value to withKey: %w`, err)
			}
			var wksoptions []jws.WithKeySuboption
			for _, subopt := range wk.options {
				wksopt, ok := subopt.(jws.WithKeySuboption)
				if !ok {
					return nil, fmt.Errorf(`expected optional arguments in jwt.WithKey to be jws.WithKeySuboption, but got %T`, subopt)
				}
				wksoptions = append(wksoptions, wksopt)
			}

			voptions = append(voptions, jws.WithKey(wk.alg, wk.key, wksoptions...))
		case identKeySet{}:
			var wks withKeySet
			if err := option.Value(&wks); err != nil {
				return nil, fmt.Errorf(`failed to convert option value to withKeySet: %w`, err)
			}
			var wkssoptions []jws.WithKeySetSuboption
			for _, subopt := range wks.options {
				wkssopt, ok := subopt.(jws.WithKeySetSuboption)
				if !ok {
					return nil, fmt.Errorf(`expected optional arguments in jwt.WithKey to be jws.WithKeySetSuboption, but got %T`, subopt)
				}
				wkssoptions = append(wkssoptions, wkssopt)
			}

			voptions = append(voptions, jws.WithKeySet(wks.set, wkssoptions...))
		case identVerifyAuto{}:
			var vo jws.VerifyOption
			if err := option.Value(&vo); err != nil {
				return nil, fmt.Errorf(`failed to decode VerifyOption: %w`, err)
			}
			voptions = append(voptions, vo)
		case identKeyProvider{}:
			var kp jws.KeyProvider
			if err := option.Value(&kp); err != nil {
				return nil, fmt.Errorf(`failed to decode KeyProvider: %w`, err)
			}
			voptions = append(voptions, jws.WithKeyProvider(kp))
		case identBase64Encoder{}:
			var enc jws.Base64Encoder
			if err := option.Value(&enc); err != nil {
				return nil, fmt.Errorf(`failed to decode Base64Encoder: %w`, err)
			}
			voptions = append(voptions, jws.WithBase64Encoder(enc))
		}
	}
	return voptions, nil
}

type withKey struct {
	alg     jwa.KeyAlgorithm
	key     any
	options []Option
}

// WithKey is a multipurpose option. It can be used for either jwt.Sign, jwt.Parse (and
// its siblings), and jwt.Serializer methods. For signatures, please see the documentation
// for `jws.WithKey` for more details. For encryption, please see the documentation
// for `jwe.WithKey`.
//
// It is the caller's responsibility to match the suboptions to the operation that they
// are performing. For example, you are not allowed to do this, because the operation
// is to generate a signature, and yet you are passing options for jwe:
//
//	jwt.Sign(token, jwt.WithKey(alg, key, jweOptions...))
//
// In the above example, the creation of the option via `jwt.WithKey()` will work, but
// when `jwt.Sign()` is called, the fact that you passed JWE suboptions will be
// detected, and an error will occur.
func WithKey(alg jwa.KeyAlgorithm, key any, suboptions ...Option) SignEncryptParseOption {
	return &signEncryptParseOption{option.New(identKey{}, &withKey{
		alg:     alg,
		key:     key,
		options: suboptions,
	})}
}

type withKeySet struct {
	set     jwk.Set
	options []any
}

// WithKeySet forces the Parse method to verify the JWT message
// using one of the keys in the given key set.
//
// Key IDs (`kid`) in the JWS message and the JWK in the given `jwk.Set`
// must match in order for the key to be a candidate to be used for
// verification.
//
// This is for security reasons. If you must disable it, you can do so by
// specifying `jws.WithRequireKid(false)` in the suboptions. But we don't
// recommend it unless you know exactly what the security implications are
//
// When using this option, keys MUST have a proper 'alg' field
// set. This is because we need to know the exact algorithm that
// you (the user) wants to use to verify the token. We do NOT
// trust the token's headers, because they can easily be tampered with.
//
// However, there _is_ a workaround if you do understand the risks
// of allowing a library to automatically choose a signature verification strategy,
// and you do not mind the verification process having to possibly
// attempt using multiple times before succeeding to verify. See
// `jws.InferAlgorithmFromKey` option
//
// If you have only one key in the set, and are sure you want to
// use that key, you can use the `jwt.WithDefaultKey` option.
func WithKeySet(set jwk.Set, options ...any) ParseOption {
	return &parseOption{option.New(identKeySet{}, &withKeySet{
		set:     set,
		options: options,
	})}
}

// WithIssuer specifies that expected issuer value. If not specified,
// the value of issuer is not verified at all.
func WithIssuer(s string) ValidateOption {
	return WithValidator(issuerClaimValueIs(s))
}

// WithSubject specifies that expected subject value. If not specified,
// the value of subject is not verified at all.
func WithSubject(s string) ValidateOption {
	return WithValidator(ClaimValueIs(SubjectKey, s))
}

// WithJwtID specifies that expected jti value. If not specified,
// the value of jti is not verified at all.
func WithJwtID(s string) ValidateOption {
	return WithValidator(ClaimValueIs(JwtIDKey, s))
}

// WithAudience specifies that expected audience value.
// `Validate()` will return true if one of the values in the `aud` element
// matches this value. If not specified, the value of `aud` is not
// verified at all.
func WithAudience(s string) ValidateOption {
	return WithValidator(audienceClaimContainsString(s))
}

// WithClaimValue specifies the expected value for a given claim
func WithClaimValue(name string, v any) ValidateOption {
	return WithValidator(ClaimValueIs(name, v))
}

// WithTypedClaim allows a private claim to be parsed into the object type of
// your choice. It works much like the RegisterCustomField, but the effect
// is only applicable to the jwt.Parse function call which receives this option.
//
// While this can be extremely useful, this option should be used with caution:
// There are many caveats that your entire team/user-base needs to be aware of,
// and therefore in general its use is discouraged. Only use it when you know
// what you are doing, and you document its use clearly for others.
//
// First and foremost, this is a "per-object" option. Meaning that given the same
// serialized format, it is possible to generate two objects whose internal
// representations may differ. That is, if you parse one _WITH_ the option,
// and the other _WITHOUT_, their internal representation may completely differ.
// This could potentially lead to problems.
//
// Second, specifying this option will slightly slow down the decoding process
// as it needs to consult multiple definitions sources (global and local), so
// be careful if you are decoding a large number of tokens, as the effects will stack up.
//
// Finally, this option will also NOT work unless the tokens themselves support such
// parsing mechanism. For example, while tokens obtained from `jwt.New()` and
// `openid.New()` will respect this option, if you provide your own custom
// token type, it will need to implement the TokenWithDecodeCtx interface.
func WithTypedClaim(name string, object any) ParseOption {
	return &parseOption{option.New(identTypedClaim{}, claimPair{Name: name, Value: object})}
}

// WithRequiredClaim specifies that the claim identified the given name
// must exist in the token. Only the existence of the claim is checked:
// the actual value associated with that field is not checked.
func WithRequiredClaim(name string) ValidateOption {
	return WithValidator(IsRequired(name))
}

// WithMaxDelta specifies that given two claims `c1` and `c2` that represent time, the difference in
// time.Duration must be less than equal to the value specified by `d`. If `c1` or `c2` is the
// empty string, the current time (as computed by `time.Now` or the object passed via
// `WithClock()`) is used for the comparison.
//
// `c1` and `c2` are also assumed to be required, therefore not providing either claim in the
// token will result in an error.
//
// Because there is no way of reliably knowing how to parse private claims, we currently only
// support `iat`, `exp`, and `nbf` claims.
//
// If the empty string is passed to c1 or c2, then the current time (as calculated by time.Now() or
// the clock object provided via WithClock()) is used.
//
// For example, in order to specify that `exp` - `iat` should be less than 10*time.Second, you would write
//
//	jwt.Validate(token, jwt.WithMaxDelta(10*time.Second, jwt.ExpirationKey, jwt.IssuedAtKey))
//
// If AcceptableSkew of 2 second is specified, the above will return valid for any value of
// `exp` - `iat`  between 8 (10-2) and 12 (10+2).
func WithMaxDelta(dur time.Duration, c1, c2 string) ValidateOption {
	return WithValidator(MaxDeltaIs(c1, c2, dur))
}

// WithMinDelta is almost exactly the same as WithMaxDelta, but force validation to fail if
// the difference between time claims are less than dur.
//
// For example, in order to specify that `exp` - `iat` should be greater than 10*time.Second, you would write
//
//	jwt.Validate(token, jwt.WithMinDelta(10*time.Second, jwt.ExpirationKey, jwt.IssuedAtKey))
//
// The validation would fail if the difference is less than 10 seconds.
func WithMinDelta(dur time.Duration, c1, c2 string) ValidateOption {
	return WithValidator(MinDeltaIs(c1, c2, dur))
}

// WithVerifyAuto specifies that the JWS verification should be attempted
// by using the data available in the JWS message. Currently only verification
// method available is to use the keys available in the JWKS URL pointed
// in the `jku` field.
//
// Please read the documentation for `jws.VerifyAuto` for more details.
func WithVerifyAuto(f jwk.Fetcher, options ...jwk.FetchOption) ParseOption {
	return &parseOption{option.New(identVerifyAuto{}, jws.WithVerifyAuto(f, options...))}
}

func WithInsecureNoSignature() SignOption {
	return &signEncryptParseOption{option.New(identInsecureNoSignature{}, (any)(nil))}
}
