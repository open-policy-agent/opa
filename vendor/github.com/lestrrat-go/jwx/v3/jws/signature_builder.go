package jws

import (
	"bytes"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws/jwsbb"
)

var signatureBuilderPool = pool.New[*signatureBuilder](allocSignatureBuilder, freeSignatureBuilder)

// signatureBuilder is a transient object that is used to build
// a single JWS signature.
//
// In a multi-signature JWS message, each message is paired with
// the following:
// - a signer (the object that takes a buffer and key and generates a signature)
// - a key (the key that is used to sign the payload)
// - protected headers (the headers that are protected by the signature)
// - public headers (the headers that are not protected by the signature)
//
// This object stores all of this information in one place.
//
// This object does NOT take care of any synchronization, because it is
// meant to be used in a single-threaded context.
type signatureBuilder struct {
	alg       jwa.SignatureAlgorithm
	signer    Signer
	signer2   Signer2
	key       any
	protected Headers
	public    Headers
}

func allocSignatureBuilder() *signatureBuilder {
	return &signatureBuilder{}
}

func freeSignatureBuilder(sb *signatureBuilder) *signatureBuilder {
	sb.alg = jwa.EmptySignatureAlgorithm()
	sb.signer = nil
	sb.signer2 = nil
	sb.key = nil
	sb.protected = nil
	sb.public = nil
	return sb
}

func (sb *signatureBuilder) Build(sc *signContext, payload []byte) (*Signature, error) {
	protected := sb.protected
	if protected == nil {
		protected = NewHeaders()
	}

	if err := protected.Set(AlgorithmKey, sb.alg); err != nil {
		return nil, signerr(`failed to set "alg" header: %w`, err)
	}

	if key, ok := sb.key.(jwk.Key); ok {
		if kid, ok := key.KeyID(); ok && kid != "" {
			if err := protected.Set(KeyIDKey, kid); err != nil {
				return nil, signerr(`failed to set "kid" header: %w`, err)
			}
		}
	}

	hdrs, err := mergeHeaders(sb.public, protected)
	if err != nil {
		return nil, signerr(`failed to merge headers: %w`, err)
	}

	// raw, json format headers
	hdrbuf, err := json.Marshal(hdrs)
	if err != nil {
		return nil, fmt.Errorf(`failed to marshal headers: %w`, err)
	}

	// check if we need to base64 encode the payload
	b64 := getB64Value(hdrs)
	if !b64 && !sc.detached {
		if bytes.IndexByte(payload, tokens.Period) != -1 {
			return nil, fmt.Errorf(`payload must not contain a "."`)
		}
	}

	combined := jwsbb.SignBuffer(nil, hdrbuf, payload, sc.encoder, b64)

	var sig Signature
	sig.protected = protected
	sig.headers = sb.public

	if sb.signer2 != nil {
		signature, err := sb.signer2.Sign(sb.key, combined)
		if err != nil {
			return nil, fmt.Errorf(`failed to sign payload: %w`, err)
		}
		sig.signature = signature
		return &sig, nil
	}

	if sb.signer == nil {
		panic("can't get here")
	}

	signature, err := sb.signer.Sign(combined, sb.key)
	if err != nil {
		return nil, fmt.Errorf(`failed to sign payload: %w`, err)
	}

	sig.signature = signature

	return &sig, nil
}
