package jwe

import (
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/option/v2"
)

// Specify contents of the protected header. Some fields such as
// "enc" and "zip" will be overwritten when encryption is performed.
//
// There is no equivalent for unprotected headers in this implementation
func WithProtectedHeaders(h Headers) EncryptOption {
	cloned, _ := h.Clone()
	return &encryptOption{option.New(identProtectedHeaders{}, cloned)}
}

type withKey struct {
	alg     jwa.KeyAlgorithm
	key     any
	headers Headers
}

type WithKeySuboption interface {
	Option
	withKeySuboption()
}

type withKeySuboption struct {
	Option
}

func (*withKeySuboption) withKeySuboption() {}

// WithPerRecipientHeaders is used to pass header values for each recipient.
// Note that these headers are by definition _unprotected_.
func WithPerRecipientHeaders(hdr Headers) WithKeySuboption {
	return &withKeySuboption{option.New(identPerRecipientHeaders{}, hdr)}
}

// WithKey is used to pass a static algorithm/key pair to either `jwe.Encrypt()` or `jwe.Decrypt()`.
// either a raw key or `jwk.Key` may be passed as `key`.
//
// The `alg` parameter is the identifier for the key encryption algorithm that should be used.
// It is of type `jwa.KeyAlgorithm` but in reality you can only pass `jwa.KeyEncryptionAlgorithm`
// types. It is this way so that the value in `(jwk.Key).Algorithm()` can be directly
// passed to the option. If you specify other algorithm types such as `jwa.SignatureAlgorithm`,
// then you will get an error when `jwe.Encrypt()` or `jwe.Decrypt()` is executed.
//
// Unlike `jwe.WithKeySet()`, the `kid` field does not need to match for the key
// to be tried.
func WithKey(alg jwa.KeyAlgorithm, key any, options ...WithKeySuboption) EncryptDecryptOption {
	var hdr Headers
	for _, option := range options {
		switch option.Ident() {
		case identPerRecipientHeaders{}:
			if err := option.Value(&hdr); err != nil {
				panic(`jwe.WithKey() requires Headers value for WithPerRecipientHeaders option`)
			}
		}
	}

	return &encryptDecryptOption{option.New(identKey{}, &withKey{
		alg:     alg,
		key:     key,
		headers: hdr,
	})}
}

func WithKeySet(set jwk.Set, options ...WithKeySetSuboption) DecryptOption {
	requireKid := true
	for _, option := range options {
		switch option.Ident() {
		case identRequireKid{}:
			if err := option.Value(&requireKid); err != nil {
				panic(`jwe.WithKeySet() requires bool value for WithRequireKid option`)
			}
		}
	}

	return WithKeyProvider(&keySetProvider{
		set:        set,
		requireKid: requireKid,
	})
}

// WithJSON specifies that the result of `jwe.Encrypt()` is serialized in
// JSON format.
//
// If you pass multiple keys to `jwe.Encrypt()`, it will fail unless
// you also pass this option.
func WithJSON(options ...WithJSONSuboption) EncryptOption {
	var pretty bool
	for _, option := range options {
		switch option.Ident() {
		case identPretty{}:
			if err := option.Value(&pretty); err != nil {
				panic(`jwe.WithJSON() requires bool value for WithPretty option`)
			}
		}
	}

	format := fmtJSON
	if pretty {
		format = fmtJSONPretty
	}
	return &encryptOption{option.New(identSerialization{}, format)}
}
