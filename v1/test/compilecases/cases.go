// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compilecases contains the schema and loader for the compiler
// conformance corpus: Rego modules in, diagnostics out.
//
// The package deliberately depends on nothing but the loader it shares with the
// other corpora. OPA's own runner lives in package ast, and the generator that
// fills in the fixtures lives in build/generate-compiler-cases.
package compilecases

import (
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

// DefaultModuleName is the name given to the first module of a case, and the
// module errors are reported against unless stated otherwise.
const DefaultModuleName = conformance.DefaultModuleName

// RegoVersions are the accepted values of a case's rego_version.
var RegoVersions = []string{"v0", "v1", "v0-compat-v1"}

// Accepted values of a case's strict field. They say whether the case's
// expectations depend on the setting, so a consumer whose own strict mode is not
// switchable can tell which cases it can run. Absent means immaterial; OPA's runner
// compiles those with strict mode off.
const (
	StrictEnabled  = "enabled"  // compile with strict mode on; the expectations depend on it
	StrictDisabled = "disabled" // compile with strict mode off; the expectations depend on it
)

// Strictnesses are the accepted values of a case's strict field.
var Strictnesses = []string{StrictEnabled, StrictDisabled}

// ModuleName returns the name given to the i-th module of a case.
func ModuleName(i int) string {
	return conformance.ModuleName(i)
}

// TestCase represents a single test case: a set of modules, and either the
// diagnostics compiling them must produce or the assertion that they compile.
type TestCase struct {
	Filename             string   `json:"-"                                yaml:"-"`                               // name of file that case was loaded from
	Note                 string   `json:"note"                             yaml:"note"`                            // globally unique identifier for this test case
	Modules              []string `json:"modules"                          yaml:"modules"`                         // policies to compile, named test-0.rego, test-1.rego, ...
	RegoVersion          string   `json:"rego_version,omitempty"           yaml:"rego_version,omitempty"`          // rego version to parse the modules as: v0, v1 (default), or v0-compat-v1
	Strict               string   `json:"strict,omitempty"                 yaml:"strict,omitempty"`                // enabled, disabled, or absent where strict mode does not change the outcome
	ExperimentalKeywords bool     `json:"experimental_keywords,omitempty"  yaml:"experimental_keywords,omitempty"` // opt-in to experimental future keywords
	PrintStatements      bool     `json:"print_statements,omitempty"       yaml:"print_statements,omitempty"`      // keep print() calls instead of erasing them, as required to reach diagnostics about their operands

	WantErrors []Error `json:"want_errors,omitempty"  yaml:"want_errors,omitempty"` // diagnostics the compilation must produce
	Exhaustive bool    `json:"exhaustive,omitempty"   yaml:"exhaustive,omitempty"`  // require want_errors to be the complete set, not a subset

	// Want is what compiling produces, one entry per module, in the same order.
	// Generated, not authored: run `make generate` and review the diff.
	Want []Want `json:"want,omitempty"  yaml:"want,omitempty"`
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

// Failure reports whether tc asserts that the modules fail to compile.
func (tc TestCase) Failure() bool {
	return len(tc.WantErrors) > 0
}

// Want is what one module compiles to: Rego where OPA's printer can express it,
// marshalled AST where it cannot. Exactly one of the two, per module, so that an
// unprintable module does not drag its neighbours into the AST form.
type Want struct {
	// Module is compared as an AST. The Rego is only how it is written down.
	Module string `json:"module,omitempty"  yaml:"module,omitempty"`

	// Imports are the directive imports the input module carried, which the compiler
	// dropped and which have to be in effect to parse Module. Per entry: they do not
	// carry across modules, and `not` means different things with and without its
	// import.
	Imports []string `json:"imports,omitempty"  yaml:"imports,omitempty"`

	// AST is the same assertion marshalled, for a compiled form with no Rego
	// spelling that parses back to it.
	AST string `json:"ast,omitempty"  yaml:"ast,omitempty"`
}

// Transform reports whether tc asserts what its modules compile to.
func (tc TestCase) Transform() bool {
	return len(tc.Want) > 0
}

// StrictMode reports whether the case must be compiled with the compiler's
// strict mode on.
func (tc TestCase) StrictMode() bool {
	return tc.Strict == StrictEnabled
}

// Validate returns an error if tc is not a well-formed case. A case asserts
// diagnostics with want_errors, or what its modules compile to with want_modules
// or want_ast, or both; an absent want_errors is itself the assertion that nothing
// is reported.
func (tc TestCase) Validate() error {
	switch {
	case tc.Note == "":
		return errors.New("missing 'note'")
	case len(tc.Modules) == 0:
		return errors.New("missing 'modules'")
	case tc.RegoVersion != "" && !slices.Contains(RegoVersions, tc.RegoVersion):
		return fmt.Errorf("unknown 'rego_version' %q, expected one of %v", tc.RegoVersion, RegoVersions)
	case tc.Strict != "" && !slices.Contains(Strictnesses, tc.Strict):
		return fmt.Errorf("unknown 'strict' %q, expected one of %v, or absent where strict mode does not change the outcome",
			tc.Strict, Strictnesses)
	case tc.Exhaustive && !tc.Failure():
		return errors.New("'exhaustive' only applies to a case asserting 'want_errors'")
	case tc.Transform() && len(tc.Want) != len(tc.Modules):
		return fmt.Errorf("'want' has %d entries for %d modules; it takes one per module, in the same order",
			len(tc.Want), len(tc.Modules))
	case !tc.Failure() && !tc.Transform():
		return errors.New("expected 'want_errors', or 'want' where the modules compile; run `make generate` to fill one in")
	}

	for i, want := range tc.Want {
		switch {
		case want.Module == "" && want.AST == "":
			return fmt.Errorf("'want[%d]' has neither 'module' nor 'ast'", i)
		case want.Module != "" && want.AST != "":
			return fmt.Errorf("'want[%d]' has both 'module' and 'ast', which are two spellings of one assertion", i)
		case want.AST != "" && len(want.Imports) > 0:
			return fmt.Errorf("'want[%d].imports' only applies to an entry asserting 'module'", i)
		}
	}

	for i, want := range tc.Want {
		if err := conformance.CheckTrailingWhitespace(fmt.Sprintf("want[%d].module", i), want.Module); err != nil {
			return err
		}
		if _, err := tc.WantParserOptions(i); err != nil {
			return err
		}
	}

	for i, module := range tc.Modules {
		if err := conformance.CheckTrailingWhitespace(ModuleName(i), module); err != nil {
			return err
		}
	}

	for _, e := range tc.WantErrors {
		if e.Module == "" {
			continue
		}
		if !slices.Contains(tc.ModuleNames(), e.Module) {
			return fmt.Errorf("'want_errors' names module %q, which the case does not define", e.Module)
		}
	}

	return nil
}

// WantOptions is how a Want entry's Module has to be parsed. Stated without
// reference to v1/ast, so a consumer can map it onto its own parser.
type WantOptions struct {
	RegoVersion       string
	FutureKeywords    []string
	AllFutureKeywords bool
}

// WantParserOptions interprets the imports on the i-th Want entry. An import it does
// not recognise is an error rather than a no-op.
func (tc TestCase) WantParserOptions(i int) (WantOptions, error) {
	out := WantOptions{RegoVersion: tc.RegoVersion}

	if i >= len(tc.Want) {
		return out, nil
	}

	for _, imp := range tc.Want[i].Imports {
		switch {
		case imp == "rego.v1":
			// Not v0-compat-v1: that mode requires the import the printed form no
			// longer carries.
			out.RegoVersion = "v1"

		case imp == "future.keywords":
			out.AllFutureKeywords = true

		case strings.HasPrefix(imp, "future.keywords."):
			kw := strings.TrimPrefix(imp, "future.keywords.")
			if kw == "" || strings.Contains(kw, ".") {
				return WantOptions{}, fmt.Errorf("unrecognised 'want[%d].imports' entry %q", i, imp)
			}
			out.FutureKeywords = append(out.FutureKeywords, kw)

		default:
			return WantOptions{}, fmt.Errorf("unrecognised 'want[%d].imports' entry %q; "+
				"expected rego.v1, future.keywords or future.keywords.<keyword>", i, imp)
		}
	}

	return out, nil
}

// ModuleNames returns the names the case's modules are compiled under.
func (tc TestCase) ModuleNames() []string {
	names := make([]string, len(tc.Modules))
	for i := range tc.Modules {
		names[i] = ModuleName(i)
	}
	return names
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
