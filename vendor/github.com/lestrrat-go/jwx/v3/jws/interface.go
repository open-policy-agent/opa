package jws

import (
	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/jws/legacy"
)

type Signer = legacy.Signer
type Verifier = legacy.Verifier
type HMACSigner = legacy.HMACSigner
type HMACVerifier = legacy.HMACVerifier

// Base64Encoder is an interface that can be used when encoding JWS message
// components to base64. This is useful when you want to use a non-standard
// base64 encoder while generating or verifying signatures. By default JWS
// uses raw url base64 encoding (without padding), but there are apparently
// some cases where you may want to use a base64 encoders that uses padding.
//
// For example, apparently AWS ALB User Claims is provided in JWT format,
// but it uses a base64 encoding with padding.
type Base64Encoder = base64.Encoder

type DecodeCtx interface {
	CollectRaw() bool
}

// Message represents a full JWS encoded message. Flattened serialization
// is not supported as a struct, but rather it's represented as a
// Message struct with only one `signature` element.
//
// Do not expect to use the Message object to verify or construct a
// signed payload with. You should only use this when you want to actually
// programmatically view the contents of the full JWS payload.
//
// As of this version, there is one big incompatibility when using Message
// objects to convert between compact and JSON representations.
// The protected header is sometimes encoded differently from the original
// message and the JSON serialization that we use in Go.
//
// For example, the protected header `eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9`
// decodes to
//
//	{"typ":"JWT",
//	  "alg":"HS256"}
//
// However, when we parse this into a message, we create a jws.Header object,
// which, when we marshal into a JSON object again, becomes
//
//	{"typ":"JWT","alg":"HS256"}
//
// Notice that serialization lacks a line break and a space between `"JWT",`
// and `"alg"`. This causes a problem when verifying the signatures AFTER
// a compact JWS message has been unmarshaled into a jws.Message.
//
// jws.Verify() doesn't go through this step, and therefore this does not
// manifest itself. However, you may see this discrepancy when you manually
// go through these conversions, and/or use the `jwx` tool like so:
//
//	jwx jws parse message.jws | jwx jws verify --key somekey.jwk --stdin
//
// In this scenario, the first `jwx jws parse` outputs a parsed jws.Message
// which is marshaled into JSON. At this point the message's protected
// headers and the signatures don't match.
//
// To sign and verify, use the appropriate `Sign()` and `Verify()` functions.
type Message struct {
	dc         DecodeCtx
	payload    []byte
	signatures []*Signature
	b64        bool // true if payload should be base64 encoded
}

type Signature struct {
	encoder   Base64Encoder
	dc        DecodeCtx
	headers   Headers // Unprotected Headers
	protected Headers // Protected Headers
	signature []byte  // Signature
	detached  bool
}
