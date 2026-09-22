// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// TestTranslateParserOptionsCarryEveryField fails when a field is added to the schema's
// ParseOptions and translateParserOptions does not carry it.
//
// The runner has the same translation and the same test. Neither package can import the
// other, so this is what keeps the generator from writing fixtures under options the
// runner does not check them under.
func TestTranslateParserOptionsCarryEveryField(t *testing.T) {
	carried := map[string]func(*testing.T){
		"RegoVersion": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{RegoVersion: "v0"},
				func(p ast.ParserOptions) bool { return p.RegoVersion == ast.RegoV0 })
		},
		"FutureKeywords": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{FutureKeywords: []string{"in"}},
				func(p ast.ParserOptions) bool { return slices.Equal(p.FutureKeywords, []string{"in"}) })
		},
		"AllFutureKeywords": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{AllFutureKeywords: true},
				func(p ast.ParserOptions) bool { return p.AllFutureKeywords })
		},
		"ExperimentalKeywords": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{ExperimentalKeywords: true}, func(p ast.ParserOptions) bool {
				// Asserts the flag reaches the capabilities, not an observable difference:
				// ast.experimentalFutureKeywords is empty in this build, so capabilities
				// built with the opt-in and without it are equal. This becomes a real
				// check the day OPA adds an experimental keyword.
				return p.Capabilities != nil && slices.Equal(
					slices.Sorted(slices.Values(p.Capabilities.FutureKeywords)),
					slices.Sorted(slices.Values(ast.CapabilitiesForThisVersion(
						ast.CapabilitiesExperimentalKeywords(true)).FutureKeywords)))
			})
		},
		"ProcessAnnotations": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{ProcessAnnotations: true},
				func(p ast.ParserOptions) bool { return p.ProcessAnnotation })
		},
		"SkipRules": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{SkipRules: true},
				func(p ast.ParserOptions) bool { return p.SkipRules })
		},
	}

	for _, field := range conformance.ParseOptionFields() {
		check, ok := carried[field]
		if !ok {
			t.Errorf("conformance.ParseOptions.%s is not carried by translateParserOptions; carry it, and say here how", field)
			continue
		}
		t.Run(field, check)
	}
}

func assertCarried(t *testing.T, opts conformance.ParseOptions, carried func(ast.ParserOptions) bool) {
	t.Helper()

	popts, err := translateParserOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !carried(popts) {
		t.Errorf("the field was not carried onto %+v", popts)
	}
}
