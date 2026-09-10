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

// Accepted values of a case's strict field. Strict mode is a boolean in the
// compiler; these say whether the case's expectations depend on which way it is
// set, which is what lets a consumer whose own strict mode is not switchable
// decide which cases it can run.
//
// An absent value means strict mode is immaterial: the case asserts the same
// thing either way, so any consumer can run it however its own compiler is
// configured. OPA's runner compiles those with strict mode off, the compiler's
// default.
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

	// Want is what compiling produces, one entry per entry in Modules and in the
	// same order.
	//
	// Generated, not authored: write the modules and the configuration, run
	// `make generate`, and review the diff.
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

// Want is what one module compiles to, as Rego where OPA's printer can express it
// and as a marshalled AST where it cannot. Exactly one of the two.
//
// The two live in one entry, rather than in lists a case has to keep aligned by
// index, so that a module needing the AST form does not drag its neighbours into it —
// and so that Imports sits with the module it belongs to.
type Want struct {
	// Module is the Rego the corresponding input module compiles to. It is parsed
	// and compared as an AST: how an implementation arrives at that AST, and whether
	// it can print it back, is its own business.
	Module string `json:"module,omitempty"  yaml:"module,omitempty"`

	// Imports are the directive imports the input module carried, as written, which
	// have to be in effect to parse Module.
	//
	// The compiler resolves a directive away once it has taken effect:
	// `import future.keywords.or` disappears and the compiled form uses `or` as an
	// operator with no import in sight, and `import rego.v1` disappears from a v0
	// module whose compiled form then only parses as v1. There is no import to write
	// back — adding one would produce a module whose AST has an import the compiled
	// one does not.
	//
	// Per entry because directives do not carry across modules: one module may
	// import `future.keywords.not` while its neighbour uses `not` as ordinary
	// negation, and activating it for both would read the neighbour's `not q` as a
	// Not node instead of a negated expression.
	Imports []string `json:"imports,omitempty"  yaml:"imports,omitempty"`

	// AST is the same assertion as Module, marshalled, for a compiled form that has
	// no Rego spelling that parses back to it. Less legible and no weaker, so the
	// generator reaches for it only where the round-trip fails.
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

// WantOptions is how one Want entry's Module has to be parsed: the case's own
// rego_version unless a directive import overrides it, plus whatever keyword
// activation those imports ask for.
//
// Expressed without reference to v1/ast, so that both OPA's runner and a consumer's
// own tooling can map it onto their parser without this package depending on either.
type WantOptions struct {
	RegoVersion       string   // v0, v1 or v0-compat-v1
	FutureKeywords    []string // keywords to activate by name
	AllFutureKeywords bool     // activate every future keyword the version has
}

// WantParserOptions interprets the imports declared on the i-th Want entry. An
// import it does not recognise is an error: these are directives, and one the corpus
// cannot explain is one a consumer cannot honour.
func (tc TestCase) WantParserOptions(i int) (WantOptions, error) {
	out := WantOptions{RegoVersion: tc.RegoVersion}

	if i >= len(tc.Want) {
		return out, nil
	}

	for _, imp := range tc.Want[i].Imports {
		switch {
		case imp == "rego.v1":
			// The dialect the import selected, which the compiled module keeps and
			// the printed form no longer says. v0-compat-v1 is not it: that mode
			// requires the import, and the printed form does not carry one.
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
