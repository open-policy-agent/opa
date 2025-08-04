package jws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws/jwsbb"
)

// verifyContext holds the state during JWS verification
type verifyContext struct {
	parseOptions    []ParseOption
	dst             *Message
	detachedPayload []byte
	keyProviders    []KeyProvider
	keyUsed         any
	validateKey     bool
	encoder         Base64Encoder
	//nolint:containedctx
	ctx context.Context
}

var verifyContextPool = pool.New[*verifyContext](allocVerifyContext, freeVerifyContext)

func allocVerifyContext() *verifyContext {
	return &verifyContext{
		encoder: base64.DefaultEncoder(),
		ctx:     context.Background(),
	}
}

func freeVerifyContext(vc *verifyContext) *verifyContext {
	vc.parseOptions = vc.parseOptions[:0]
	vc.dst = nil
	vc.detachedPayload = nil
	vc.keyProviders = vc.keyProviders[:0]
	vc.keyUsed = nil
	vc.validateKey = false
	vc.encoder = base64.DefaultEncoder()
	vc.ctx = context.Background()
	return vc
}

func (vc *verifyContext) ProcessOptions(options []VerifyOption) error {
	//nolint:forcetypeassert
	for _, option := range options {
		switch option.Ident() {
		case identMessage{}:
			if err := option.Value(&vc.dst); err != nil {
				return verifyerr(`invalid value for option WithMessage: %w`, err)
			}
		case identDetachedPayload{}:
			if err := option.Value(&vc.detachedPayload); err != nil {
				return verifyerr(`invalid value for option WithDetachedPayload: %w`, err)
			}
		case identKey{}:
			var pair *withKey
			if err := option.Value(&pair); err != nil {
				return verifyerr(`invalid value for option WithKey: %w`, err)
			}
			vc.keyProviders = append(vc.keyProviders, &staticKeyProvider{
				alg: pair.alg.(jwa.SignatureAlgorithm),
				key: pair.key,
			})
		case identKeyProvider{}:
			var kp KeyProvider
			if err := option.Value(&kp); err != nil {
				return verifyerr(`failed to retrieve key-provider option value: %w`, err)
			}
			vc.keyProviders = append(vc.keyProviders, kp)
		case identKeyUsed{}:
			if err := option.Value(&vc.keyUsed); err != nil {
				return verifyerr(`failed to retrieve key-used option value: %w`, err)
			}
		case identContext{}:
			if err := option.Value(&vc.ctx); err != nil {
				return verifyerr(`failed to retrieve context option value: %w`, err)
			}
		case identValidateKey{}:
			if err := option.Value(&vc.validateKey); err != nil {
				return verifyerr(`failed to retrieve validate-key option value: %w`, err)
			}
		case identSerialization{}:
			vc.parseOptions = append(vc.parseOptions, option.(ParseOption))
		case identBase64Encoder{}:
			if err := option.Value(&vc.encoder); err != nil {
				return verifyerr(`failed to retrieve base64-encoder option value: %w`, err)
			}
		default:
			return verifyerr(`invalid jws.VerifyOption %q passed`, `With`+strings.TrimPrefix(fmt.Sprintf(`%T`, option.Ident()), `jws.ident`))
		}
	}

	if len(vc.keyProviders) < 1 {
		return verifyerr(`no key providers have been provided (see jws.WithKey(), jws.WithKeySet(), jws.WithVerifyAuto(), and jws.WithKeyProvider()`)
	}

	return nil
}

func (vc *verifyContext) VerifyMessage(buf []byte) ([]byte, error) {
	msg, err := Parse(buf, vc.parseOptions...)
	if err != nil {
		return nil, verifyerr(`failed to parse jws: %w`, err)
	}
	defer msg.clearRaw()

	if vc.detachedPayload != nil {
		if len(msg.payload) != 0 {
			return nil, verifyerr(`can't specify detached payload for JWS with payload`)
		}

		msg.payload = vc.detachedPayload
	}

	verifyBuf := pool.ByteSlice().Get()

	// Because deferred functions bind to the current value of the variable,
	// we can't just use `defer pool.ByteSlice().Put(verifyBuf)` here.
	// Instead, we use a closure to reference the _variable_.
	// it would be better if we could call it directly, but there are
	// too many place we may return from this function
	defer func() {
		pool.ByteSlice().Put(verifyBuf)
	}()

	errs := pool.ErrorSlice().Get()
	defer func() {
		pool.ErrorSlice().Put(errs)
	}()
	for idx, sig := range msg.signatures {
		var rawHeaders []byte
		if rbp, ok := sig.protected.(interface{ rawBuffer() []byte }); ok {
			if raw := rbp.rawBuffer(); raw != nil {
				rawHeaders = raw
			}
		}

		if rawHeaders == nil {
			protected, err := json.Marshal(sig.protected)
			if err != nil {
				return nil, verifyerr(`failed to marshal "protected" for signature #%d: %w`, idx+1, err)
			}
			rawHeaders = protected
		}

		verifyBuf = verifyBuf[:0]
		verifyBuf = jwsbb.SignBuffer(verifyBuf, rawHeaders, msg.payload, vc.encoder, msg.b64)
		for i, kp := range vc.keyProviders {
			var sink algKeySink
			if err := kp.FetchKeys(vc.ctx, &sink, sig, msg); err != nil {
				return nil, verifyerr(`key provider %d failed: %w`, i, err)
			}

			for _, pair := range sink.list {
				// alg is converted here because pair.alg is of type jwa.KeyAlgorithm.
				// this may seem ugly, but we're trying to avoid declaring separate
				// structs for `alg jwa.KeyEncryptionAlgorithm` and `alg jwa.SignatureAlgorithm`
				//nolint:forcetypeassert
				alg := pair.alg.(jwa.SignatureAlgorithm)
				key := pair.key

				if err := vc.tryKey(verifyBuf, alg, key, msg, sig); err != nil {
					errs = append(errs, verifyerr(`failed to verify signature #%d with key %T: %w`, idx+1, key, err))
					continue
				}

				return msg.payload, nil
			}
		}
		errs = append(errs, verifyerr(`signature #%d could not be verified with any of the keys`, idx+1))
	}
	return nil, verifyerr(`could not verify message using any of the signatures or keys: %w`, errors.Join(errs...))
}

func (vc *verifyContext) tryKey(verifyBuf []byte, alg jwa.SignatureAlgorithm, key any, msg *Message, sig *Signature) error {
	if vc.validateKey {
		if err := validateKeyBeforeUse(key); err != nil {
			return fmt.Errorf(`failed to validate key before signing: %w`, err)
		}
	}

	verifier, err := VerifierFor(alg)
	if err != nil {
		return fmt.Errorf(`failed to get verifier for algorithm %q: %w`, alg, err)
	}

	if err := verifier.Verify(key, verifyBuf, sig.signature); err != nil {
		return verificationError{err}
	}

	// Verification succeeded
	if vc.keyUsed != nil {
		if err := blackmagic.AssignIfCompatible(vc.keyUsed, key); err != nil {
			return fmt.Errorf(`failed to assign used key (%T) to %T: %w`, key, vc.keyUsed, err)
		}
	}

	if vc.dst != nil {
		*(vc.dst) = *msg
	}

	return nil
}
