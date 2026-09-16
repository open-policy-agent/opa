// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package conformance

import (
	"fmt"
	"strings"
)

// ParseOptions is how a corpus's Rego has to be parsed. Stated without reference
// to v1/ast, so a consumer can map it onto its own parser.
type ParseOptions struct {
	RegoVersion       string
	FutureKeywords    []string
	AllFutureKeywords bool
}

// DirectiveOption folds a directive import path into out, reporting whether imp
// was a directive at all.
//
// Both corpora carry import paths beside Rego that has nowhere to declare them: a
// compiled module the compiler stripped its directives from, a query, a body. at
// names the field for the error message.
func DirectiveOption(at, imp string, out *ParseOptions) (bool, error) {
	switch {
	case imp == "rego.v1":
		// Not v0-compat-v1: that mode requires the import, which the Rego this is
		// in effect for does not carry.
		out.RegoVersion = "v1"

	case imp == "future.keywords":
		out.AllFutureKeywords = true

	case strings.HasPrefix(imp, "future.keywords."):
		kw := strings.TrimPrefix(imp, "future.keywords.")
		if kw == "" || strings.Contains(kw, ".") {
			return false, fmt.Errorf("unrecognised '%s' entry %q", at, imp)
		}
		out.FutureKeywords = append(out.FutureKeywords, kw)

	default:
		return false, nil
	}

	return true, nil
}

// UnknownDirective is the error for an imports entry that is not a directive, in
// the fields where nothing else is accepted.
func UnknownDirective(at, imp string) error {
	return fmt.Errorf("unrecognised '%s' entry %q; "+
		"expected rego.v1, future.keywords or future.keywords.<keyword>", at, imp)
}
