// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package identifier

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/identifier"
)

// TLSBased extracts the CN of the client's TLS ceritificate
type TLSBased = v1.TLSBased

// NewTLSBased returns a new TLSBased object.
func NewTLSBased(inner http.Handler) *TLSBased {
	return v1.NewTLSBased(inner)
}
