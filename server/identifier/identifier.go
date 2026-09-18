// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package identifier provides handlers for associating identity information with incoming requests.
package identifier

import (
	"crypto/x509"
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/identifier"
)

// ClientCertificates returns the ClientCertificates of the caller associated with ctx.
func ClientCertificates(r *http.Request) ([]*x509.Certificate, bool) {
	return v1.ClientCertificates(r)
}

// SetClientCertificates returns a new http.Request with the ClientCertificates set to v.
func SetClientCertificates(r *http.Request, v []*x509.Certificate) *http.Request {
	return v1.SetClientCertificates(r, v)
}

// Identity returns the identity of the caller associated with ctx.
func Identity(r *http.Request) (string, bool) {
	return v1.Identity(r)
}

// SetIdentity returns a new http.Request with the identity set to v.
func SetIdentity(r *http.Request, v string) *http.Request {
	return v1.SetIdentity(r, v)
}

// TLSBased extracts the CN of the client's TLS ceritificate
type TLSBased = v1.TLSBased

// NewTLSBased returns a new TLSBased object.
func NewTLSBased(inner http.Handler) *TLSBased {
	return v1.NewTLSBased(inner)
}

// TokenBased extracts Bearer tokens from the request.
type TokenBased = v1.TokenBased

// NewTokenBased returns a new TokenBased object.
func NewTokenBased(inner http.Handler) *TokenBased {
	return v1.NewTokenBased(inner)
}
