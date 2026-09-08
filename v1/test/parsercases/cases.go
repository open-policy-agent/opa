// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package parsercases contains the schema and loader for the parser conformance
// corpus: a Rego module in, an AST, an equivalent module, or diagnostics out.
//
// The package deliberately depends on nothing but the loader it shares with the
// other corpora. OPA's own runner lives in package ast, and the generator that
// writes the fixtures lives in build/generate-parser-cases.
package parsercases

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Error is one expected diagnostic.
type Error = conformance.Error

// Set represents a collection of test cases.
type Set = conformance.Set[TestCase]

// DefaultModuleName is the name given to the module of a case, and the module
// errors are reported against.
const DefaultModuleName = conformance.DefaultModuleName

// RegoVersions are the accepted values of a case's rego_version.
var RegoVersions = []string{"v0", "v1", "v0-compat-v1"}

// TestCase represents a single test case: one module to parse, and either the
// AST that parse must produce or the diagnostics it must fail with.
type TestCase struct {
	Filename string `json:"-"       yaml:"-"`      // name of file that case was loaded from
	Note     string `json:"note"    yaml:"note"`   // globally unique identifier for this test case
	Module   string `json:"module"  yaml:"module"` // the policy to parse, named test-0.rego

	RegoVersion string `json:"rego_version,omitempty"  yaml:"rego_version,omitempty"` // rego version to parse the module as: v0, v1 (default), or v0-compat-v1

	// FutureKeywords and AllFutureKeywords activate future keywords through a
	// parser option rather than through the module. Prefer an
	// `import future.keywords.<kw>` in the module, or the `import future.keywords`
	// wildcard: an import is part of the Rego, so any conforming parser honours it,
	// where these two are out-of-band options a consumer has to expose to run the
	// corpus at all. Reach for them only where the case cannot be expressed with
	// an import.
	FutureKeywords    []string `json:"future_keywords,omitempty"      yaml:"future_keywords,omitempty"`
	AllFutureKeywords bool     `json:"all_future_keywords,omitempty"  yaml:"all_future_keywords,omitempty"`

	ExperimentalKeywords bool `json:"experimental_keywords,omitempty"  yaml:"experimental_keywords,omitempty"` // opt-in to experimental future keywords, which have no import
	Annotations          bool `json:"annotations,omitempty"            yaml:"annotations,omitempty"`           // parse metadata comments into annotations
	Locations            bool `json:"locations,omitempty"              yaml:"locations,omitempty"`             // include row and col of every node in want_ast

	WantAST        string  `json:"want_ast,omitempty"         yaml:"want_ast,omitempty"`        // the AST the parse must produce, as JSON; generated, not authored
	WantEquivalent string  `json:"want_equivalent,omitempty"  yaml:"want_equivalent,omitempty"` // a second module that must parse to the same AST
	WantErrors     []Error `json:"want_errors,omitempty"      yaml:"want_errors,omitempty"`     // diagnostics the parse must produce
	Exhaustive     bool    `json:"exhaustive,omitempty"       yaml:"exhaustive,omitempty"`      // require want_errors to be the complete set, not a subset

	// EntryPoints is authored only to override the entrypoints that would be
	// derived from the module. Generating IR fills it in either way, so a
	// consumer never has to derive them itself.
	EntryPoints []string `json:"entrypoints,omitempty"  yaml:"entrypoints,omitempty"`
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

// Failure reports whether tc asserts that the module fails to parse.
func (tc TestCase) Failure() bool {
	return len(tc.WantErrors) > 0
}

// Validate returns an error if tc is not a well-formed case. A case is either a
// failure case, asserting want_errors, or a success case, asserting want_ast and
// optionally want_equivalent; nothing else is accepted.
func (tc TestCase) Validate() error {
	switch {
	case tc.Note == "":
		return errors.New("missing 'note'")
	case tc.Module == "":
		return errors.New("missing 'module'")
	case tc.RegoVersion != "" && !slices.Contains(RegoVersions, tc.RegoVersion):
		return fmt.Errorf("unknown 'rego_version' %q, expected one of %v", tc.RegoVersion, RegoVersions)
	}

	if err := checkTrailingWhitespace("module", tc.Module); err != nil {
		return err
	}
	if err := checkTrailingWhitespace("want_equivalent", tc.WantEquivalent); err != nil {
		return err
	}

	if tc.Failure() {
		switch {
		case tc.WantAST != "":
			return errors.New("'want_ast' is not expected on a case asserting 'want_errors'")
		case tc.WantEquivalent != "":
			return errors.New("'want_equivalent' is not expected on a case asserting 'want_errors'")
		case tc.Locations:
			return errors.New("'locations' has no effect on a case asserting 'want_errors'")
		}
		return nil
	}

	switch {
	case tc.WantAST == "":
		return errors.New("expected 'want_ast' or 'want_errors'; run `make generate` to fill one in")
	case tc.Exhaustive:
		return errors.New("'exhaustive' only applies to a case asserting 'want_errors'")
	case tc.WantEquivalent != "" && tc.Locations:
		// The two modules are structurally identical but positionally different,
		// so they cannot share a fixture.
		return errors.New("'want_equivalent' and 'locations' are mutually exclusive")
	}

	// Decoding the fixture checks that it is JSON at all — nothing else does — and
	// finds its top-level fields exactly, rather than by matching the text the
	// generator happens to write. Values stay raw, so nested structure is scanned
	// for syntax but not built.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tc.WantAST), &fields); err != nil {
		return fmt.Errorf("'want_ast' is not valid JSON: %w", err)
	}
	if _, ok := fields["comments"]; ok {
		return errors.New("'want_ast' records comments; the generator and runner both drop them, so this fixture is stale")
	}

	return nil
}

// checkTrailingWhitespace rejects Rego that carries trailing whitespace on a
// line. A YAML emitter will not write a block scalar for such a value, so the
// generator would have to render the policy as a single escaped line, and
// nothing this corpus can express depends on that whitespace.
func checkTrailingWhitespace(field, rego string) error {
	for i, line := range strings.Split(rego, "\n") {
		if strings.TrimRight(line, " \t") != line {
			return fmt.Errorf("%q line %d has trailing whitespace", field, i+1)
		}
	}
	return nil
}

// Load returns the set of test cases under path.
func Load(path string) (Set, error) {
	return validate(conformance.Load[TestCase](path))
}

// MustLoad returns the set of test cases under path or panics if an error occurs.
func MustLoad(path string) Set {
	set, err := Load(path)
	if err != nil {
		panic(err)
	}
	return set
}

// LoadFS returns the set of test cases under root in fsys.
func LoadFS(fsys fs.FS, root string) (Set, error) {
	return validate(conformance.LoadFS[TestCase](fsys, root))
}

func validate(set Set, err error) (Set, error) {
	if err != nil {
		return set, err
	}
	seen := make(map[string]string, len(set.Cases))
	for i := range set.Cases {
		tc := &set.Cases[i]
		if err := tc.Validate(); err != nil {
			return set, fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
		}
		if other, ok := seen[tc.Note]; ok {
			return set, fmt.Errorf("%s: %s: note is already used by %s", tc.Filename, tc.Note, other)
		}
		seen[tc.Note] = tc.Filename
	}
	return set, nil
}
