package jws

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

type signContext struct {
	format      int
	detached    bool
	validateKey bool
	payload     []byte
	encoder     Base64Encoder
	none        *signatureBuilder // special signature builder
	sigbuilders []*signatureBuilder
}

var signContextPool = pool.New[*signContext](allocSignContext, freeSignContext)

func allocSignContext() *signContext {
	return &signContext{
		format:      fmtCompact,
		sigbuilders: make([]*signatureBuilder, 0, 1),
		encoder:     base64.DefaultEncoder(),
	}
}

func freeSignContext(ctx *signContext) *signContext {
	ctx.format = fmtCompact
	for _, sb := range ctx.sigbuilders {
		signatureBuilderPool.Put(sb)
	}
	ctx.sigbuilders = ctx.sigbuilders[:0]
	ctx.detached = false
	ctx.validateKey = false
	ctx.encoder = base64.DefaultEncoder()
	ctx.none = nil
	ctx.payload = nil

	return ctx
}

func (sc *signContext) ProcessOptions(options []SignOption) error {
	for _, option := range options {
		switch option.Ident() {
		case identSerialization{}:
			if err := option.Value(&sc.format); err != nil {
				return signerr(`failed to retrieve serialization option value: %w`, err)
			}
		case identInsecureNoSignature{}:
			var data withInsecureNoSignature
			if err := option.Value(&data); err != nil {
				return signerr(`failed to retrieve insecure-no-signature option value: %w`, err)
			}
			sb := signatureBuilderPool.Get()
			sb.alg = jwa.NoSignature()
			sb.protected = data.protected
			sb.signer = noneSigner{}
			sc.none = sb
			sc.sigbuilders = append(sc.sigbuilders, sb)
		case identKey{}:
			var data *withKey
			if err := option.Value(&data); err != nil {
				return signerr(`jws.Sign: invalid value for WithKey option: %w`, err)
			}

			alg, ok := data.alg.(jwa.SignatureAlgorithm)
			if !ok {
				return signerr(`expected algorithm to be of type jwa.SignatureAlgorithm but got (%[1]q, %[1]T)`, data.alg)
			}

			// No, we don't accept "none" here.
			if alg == jwa.NoSignature() {
				return signerr(`"none" (jwa.NoSignature) cannot be used with jws.WithKey`)
			}

			sb := signatureBuilderPool.Get()
			sb.alg = alg
			sb.protected = data.protected
			sb.key = data.key
			sb.public = data.public

			s2, err := SignerFor(alg)
			if err == nil {
				sb.signer2 = s2
			} else {
				s1, err := legacySignerFor(alg)
				if err != nil {
					sb.signer2 = defaultSigner{alg: alg}
				} else {
					sb.signer = s1
				}
			}

			sc.sigbuilders = append(sc.sigbuilders, sb)
		case identDetachedPayload{}:
			if sc.payload != nil {
				return signerr(`payload must be nil when jws.WithDetachedPayload() is specified`)
			}
			if err := option.Value(&sc.payload); err != nil {
				return signerr(`failed to retrieve detached payload option value: %w`, err)
			}
			sc.detached = true
		case identValidateKey{}:
			if err := option.Value(&sc.validateKey); err != nil {
				return signerr(`failed to retrieve validate-key option value: %w`, err)
			}
		case identBase64Encoder{}:
			if err := option.Value(&sc.encoder); err != nil {
				return signerr(`failed to retrieve base64-encoder option value: %w`, err)
			}
		}
	}
	return nil
}

func (sc *signContext) PopulateMessage(m *Message) error {
	m.payload = sc.payload
	m.signatures = make([]*Signature, 0, len(sc.sigbuilders))

	for i, sb := range sc.sigbuilders {
		// Create signature for each builders
		if sc.validateKey {
			if err := validateKeyBeforeUse(sb.key); err != nil {
				return fmt.Errorf(`failed to validate key for signature %d: %w`, i, err)
			}
		}

		sig, err := sb.Build(sc, m.payload)
		if err != nil {
			return fmt.Errorf(`failed to build signature %d: %w`, i, err)
		}

		m.signatures = append(m.signatures, sig)
	}

	return nil
}
