// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package parsercases contains the schema and loader for the parser conformance
// corpus: a Rego module or query in, an AST, an equivalent module, or diagnostics
// out.
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

	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Error is one expected diagnostic.
type Error = conformance.Error

// Set represents a collection of test cases.
type Set = conformance.Set[TestCase]

// DefaultModuleName is the name given to the module of a case, and the module
// errors are reported against.
const DefaultModuleName = conformance.DefaultModuleName

// RegoVersions are the rego versions a corpus can hold, which is the set of legal version
// directory names at its root.
var RegoVersions = conformance.RegoVersions

// TestCase represents a single test case: one module or one query to parse, and
// either the AST that parse must produce or the diagnostics it must fail with.
type TestCase struct {
	Filename string `json:"-"                 yaml:"-"`                // name of file that case was loaded from
	Note     string `json:"note"              yaml:"note"`             // identifies the case, unique within its version directory
	Module   string `json:"module,omitempty"  yaml:"module,omitempty"` // the policy to parse, named test-0.rego

	// Body is a query to parse instead of a module: one or more expressions, which
	// is what the compiler corpus's query cases are handed. Exclusive with Module.
	Body string `json:"body,omitempty"  yaml:"body,omitempty"`

	// Imports are the directives in effect for Body, which has nowhere to declare
	// them itself — future.keywords.<kw>, future.keywords, rego.v1. Body cases
	// only: a module carries its own. An entry that is not a directive is an error,
	// since parsing a body resolves no references.
	Imports []string `json:"imports,omitempty"  yaml:"imports,omitempty"`

	// RegoVersion is the version the module is parsed as, taken from the version
	// directory the case was loaded from rather than authored. Tagged out of the schema so
	// that a case stating one is rejected as an unknown field: the directory is the only
	// place it is said.
	RegoVersion string `json:"-"  yaml:"-"`

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
	// consumer never has to derive them itself. Module cases only.
	EntryPoints []string `json:"entrypoints,omitempty"  yaml:"entrypoints,omitempty"`
}

// Name returns the note identifying the case, which is unique within its version
// directory.
func (tc TestCase) Name() string {
	return tc.Note
}

// WithSource returns a copy of tc stamped with the file it was loaded from and the rego
// version of the directory holding it.
func (tc TestCase) WithSource(filename, regoVersion string) TestCase {
	tc.Filename, tc.RegoVersion = filename, regoVersion
	return tc
}

// Failure reports whether tc asserts that the module fails to parse.
func (tc TestCase) Failure() bool {
	return len(tc.WantErrors) > 0
}

// BodyCase reports whether tc parses a query rather than a module.
func (tc TestCase) BodyCase() bool {
	return tc.Body != ""
}

// Rego returns the policy tc parses, whichever entry point it is for.
func (tc TestCase) Rego() string {
	if tc.BodyCase() {
		return tc.Body
	}
	return tc.Module
}

// ParseOptions is how tc's Rego has to be read: its version, the keyword and
// annotation opt-ins, whether it is a body rather than a module, and the directives
// among a body's imports folded in.
func (tc TestCase) ParseOptions() (conformance.ParseOptions, error) {
	out := conformance.ParseOptions{
		RegoVersion:          tc.RegoVersion,
		FutureKeywords:       slices.Clone(tc.FutureKeywords),
		AllFutureKeywords:    tc.AllFutureKeywords,
		ExperimentalKeywords: tc.ExperimentalKeywords,
		ProcessAnnotations:   tc.Annotations,
		SkipRules:            tc.BodyCase(),
	}

	for _, imp := range tc.Imports {
		directive, err := conformance.DirectiveOption("imports", imp, &out)
		if err != nil {
			return conformance.ParseOptions{}, err
		}
		if !directive {
			return conformance.ParseOptions{}, conformance.UnknownDirective("imports", imp)
		}
	}

	return out, nil
}

// Validate returns an error if tc is not a well-formed case. A case parses either
// a module or a body, and is either a failure case, asserting want_errors, or a
// success case, asserting want_ast and optionally want_equivalent; nothing else is
// accepted.
func (tc TestCase) Validate() error {
	switch {
	case tc.Note == "":
		return errors.New("missing 'note'")
	case tc.Module == "" && tc.Body == "":
		return errors.New("missing 'module' or 'body'")
	case tc.Module != "" && tc.Body != "":
		return errors.New("'module' and 'body' are mutually exclusive")
	case tc.RegoVersion != "" && !slices.Contains(RegoVersions, tc.RegoVersion):
		// Stamped from the version directory, so this only fires on a case built by hand.
		return fmt.Errorf("unknown rego version %q, expected one of %v", tc.RegoVersion, RegoVersions)
	}

	for _, f := range []struct{ field, rego string }{
		{"module", tc.Module},
		{"body", tc.Body},
		{"want_equivalent", tc.WantEquivalent},
	} {
		if err := conformance.CheckTrailingWhitespace(f.field, f.rego); err != nil {
			return err
		}
	}

	if err := tc.validateEntryPoint(); err != nil {
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

	return tc.validateWantAST()
}

// validateEntryPoint checks the fields that only one of the two entry points has a
// use for.
func (tc TestCase) validateEntryPoint() error {
	if !tc.BodyCase() {
		if len(tc.Imports) > 0 {
			return errors.New("'imports' is only expected on a case asserting 'body'; a module declares its own")
		}
		return nil
	}

	switch {
	case tc.Annotations:
		// ParseBody reports "expected body but got *ast.Annotations".
		return errors.New("'annotations' has no effect on a case asserting 'body'")
	case len(tc.EntryPoints) > 0:
		return errors.New("'entrypoints' is not expected on a case asserting 'body'; there is no module to plan")
	}

	_, err := tc.ParseOptions()
	return err
}

// validateWantAST checks that the fixture is JSON at all — nothing else does — and
// that it is the shape the case's entry point produces. Values stay raw, so nested
// structure is scanned for syntax but not built.
func (tc TestCase) validateWantAST() error {
	if tc.BodyCase() {
		var exprs []json.RawMessage
		if err := json.Unmarshal([]byte(tc.WantAST), &exprs); err != nil {
			return fmt.Errorf("'want_ast' is not a JSON array of expressions: %w", err)
		}
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tc.WantAST), &fields); err != nil {
		return fmt.Errorf("'want_ast' is not valid JSON: %w", err)
	}
	if _, ok := fields["comments"]; ok {
		return errors.New("'want_ast' records comments; the generator and runner both drop them, so this fixture is stale")
	}

	return nil
}

// Load returns the set of test cases in the corpus rooted at path.
func Load(path string) (Set, error) {
	return conformance.Load[TestCase](path)
}

// MustLoad returns the set of test cases in the corpus rooted at path, or panics if an
// error occurs.
func MustLoad(path string) Set {
	return conformance.MustLoad[TestCase](path)
}

// LoadFS returns the set of test cases in the corpus rooted at root in fsys.
func LoadFS(fsys fs.FS, root string) (Set, error) {
	return conformance.LoadFS[TestCase](fsys, root)
}
