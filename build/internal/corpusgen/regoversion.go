// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package corpusgen

import (
	"fmt"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// RegoVersion resolves a case's rego_version field. An absent value is v1, the
// version the corpora default to.
func RegoVersion(s string) (ast.RegoVersion, error) {
	switch s {
	case "", "v1":
		return ast.RegoV1, nil
	case "v0":
		return ast.RegoV0, nil
	case "v0-compat-v1":
		return ast.RegoV0CompatV1, nil
	}
	return ast.RegoUndefined, fmt.Errorf("unknown rego_version %q", s)
}

// RegoVersionRejected reports whether a case written for the rego_version s
// should be rejected by a consumer that supports only the versions in supported.
//
// Matching is exact: v0-compat-v1 is its own parsing mode, so a consumer that
// supports v1 is not held to it, and one that supports v0 is not either. An
// empty supported set rejects nothing, so that a zero value is a no-op rather
// than a filter that discards the whole corpus.
func RegoVersionRejected(s string, supported []ast.RegoVersion) bool {
	if len(supported) == 0 {
		return false
	}

	// A case that reached a filter has already been validated, so an unknown
	// version here is not reachable through the loaders. Rejecting rather than
	// passing it keeps an unrecognised value from being silently run as v1.
	v, err := RegoVersion(s)
	if err != nil {
		return true
	}

	return !slices.Contains(supported, v)
}
