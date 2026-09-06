// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compilecases contains utilities for compiler diagnostic test cases
package compilecases

import (
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Error is one expected diagnostic.
type Error = conformance.Error

// Set represents a collection of test cases.
type Set = conformance.Set[TestCase]

// DefaultModuleName is the name given to the first module of a case, and the
// module errors are reported against unless stated otherwise.
const DefaultModuleName = conformance.DefaultModuleName

// ModuleName returns the name given to the i-th module of a case.
func ModuleName(i int) string {
	return conformance.ModuleName(i)
}

// TestCase represents a single test case: a set of modules that must fail to
// compile, and the diagnostics that failure must produce.
type TestCase struct {
	Filename             string   `json:"-"                                yaml:"-"`                               // name of file that case was loaded from
	Note                 string   `json:"note"                             yaml:"note"`                            // globally unique identifier for this test case
	Modules              []string `json:"modules"                          yaml:"modules"`                         // policies to compile, named test-0.rego, test-1.rego, ...
	RegoVersion          string   `json:"rego_version,omitempty"           yaml:"rego_version,omitempty"`          // rego version to parse the modules as: v0, v1 (default), or v0-compat-v1
	Strict               bool     `json:"strict,omitempty"                 yaml:"strict,omitempty"`                // enable the compiler's strict mode
	ExperimentalKeywords bool     `json:"experimental_keywords,omitempty"  yaml:"experimental_keywords,omitempty"` // opt-in to experimental future keywords
	PrintStatements      bool     `json:"print_statements,omitempty"       yaml:"print_statements,omitempty"`      // keep print() calls instead of erasing them, as required to reach diagnostics about their operands
	WantErrors           []Error  `json:"want_errors"                      yaml:"want_errors"`                     // diagnostics the compilation must produce
	Exhaustive           bool     `json:"exhaustive,omitempty"             yaml:"exhaustive,omitempty"`            // require want_errors to be the complete set, not a subset
}

// Name returns the globally unique note identifying the case.
func (tc TestCase) Name() string {
	return tc.Note
}

// WithFilename returns a copy of tc stamped with the file it was loaded from.
func (tc TestCase) WithFilename(filename string) TestCase {
	tc.Filename = filename
	return tc
}

// Load returns the set of test cases under path.
func Load(path string) (Set, error) {
	return conformance.Load[TestCase](path)
}

// MustLoad returns the set of test cases under path or panics if an error occurs.
func MustLoad(path string) Set {
	return conformance.MustLoad[TestCase](path)
}
