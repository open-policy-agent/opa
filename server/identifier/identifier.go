// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package identifier provides handlers for associating identity information with incoming requests.
package identifier

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/identifier"
)

// Identity returns the identity of the caller associated with ctx.
func Identity(r *http.Request) (string, bool) {
	return v1.Identity(r)
}

// SetIdentity returns a new http.Request with the identity set to v.
func SetIdentity(r *http.Request, v string) *http.Request {
	return v1.SetIdentity(r, v)
}
