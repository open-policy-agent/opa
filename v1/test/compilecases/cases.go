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
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
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

// Stages are the compiler stages a case can pin an intermediate assertion to, in
// pipeline order.
//
// A copy of ast.AllStages() to avoid dependency cycle.
var Stages = []string{
	"ResolveRefs",
	"InitLocalVarGen",
	"RewriteRuleHeadRefs",
	"CheckKeywordOverrides",
	"CheckDuplicateImports",
	"RemoveImports",
	"SetModuleTree",
	"SetRuleTree",
	"RewriteLocalVars",
	"RewriteTemplateStrings",
	"CheckVoidCalls",
	"RewritePrintCalls",
	"RewriteExprTerms",
	"ParseMetadataBlocks",
	"SetAnnotationSet",
	"RewriteRegoMetadataCalls",
	"SetGraph",
	"RewriteComprehensionTerms",
	"RewriteRefsInHead",
	"RewriteWithValues",
	"CheckRuleConflicts",
	"CheckUndefinedFuncs",
	"CheckSafetyRuleHeads",
	"CheckSafetyRuleBodies",
	"RewriteEquals",
	"RewriteDynamicTerms",
	"RewriteTestRulesForTracing",
	"CheckRecursion",
	"CheckTypes",
	"CheckUnsafeBuiltins",
	"CheckDeprecatedBuiltins",
	"BuildRuleIndices",
	"BuildComprehensionIndices",
	"BuildRequiredCapabilities",
}

// StageIndex returns stage's position in the pipeline, or -1 where the corpus does
// not know it.
func StageIndex(stage string) int {
	return slices.Index(Stages, stage)
}

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

	// Schemas are the JSON Schemas the modules refer to from their metadata
	// annotations, keyed by the reference written there; e.g. `schema.input`.
	Schemas map[string]string `json:"schemas,omitempty"  yaml:"schemas,omitempty"`

	WantErrors []Error `json:"want_errors,omitempty"  yaml:"want_errors,omitempty"` // diagnostics the compilation must produce
	Exhaustive bool    `json:"exhaustive,omitempty"   yaml:"exhaustive,omitempty"`  // require want_errors to be the complete set, not a subset

	// Want is what compiling produces, one entry per module, in the same order.
	// Generated, not authored: run `make generate` and review the diff.
	Want []Want `json:"want,omitempty"  yaml:"want,omitempty"`

	// WantStages is what the modules look like when the pipeline stops after a named
	// stage, keyed by stage name, one entry per module.
	//
	// Additive, and never a conformance requirement: Want and WantErrors always
	// describe the whole pipeline, so an implementation that is not split into OPA's
	// stages ignores this field and loses nothing. One that is can assert the tighter
	// intermediate form. Carried only where that form differs from the full-pipeline
	// one, so its presence means the stage does something the endpoint hides.
	//
	// Generated, not authored: name the stage and run `make generate`.
	WantStages map[string][]Want `json:"want_stages,omitempty"  yaml:"want_stages,omitempty"`
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

// SortedSchemas returns the schema references tc attaches, in a stable order. Ranging
// the map directly is never right: the order reaches the generated file.
func (tc TestCase) SortedSchemas() []string {
	refs := slices.Collect(maps.Keys(tc.Schemas))
	slices.Sort(refs)
	return refs
}

// SortedStages returns the stages tc pins an assertion to, in pipeline order.
// Ranging WantStages directly is never right: the order reaches the generated file
// and the failure output, and Go randomises it.
func (tc TestCase) SortedStages() []string {
	stages := slices.Collect(maps.Keys(tc.WantStages))
	slices.SortFunc(stages, func(a, b string) int {
		return cmp.Or(cmp.Compare(StageIndex(a), StageIndex(b)), strings.Compare(a, b))
	})
	return stages
}

// StrictMode reports whether the case must be compiled with the compiler's
// strict mode on.
func (tc TestCase) StrictMode() bool {
	return tc.Strict == StrictEnabled
}

// Validate returns an error if tc is not a well-formed case. A case asserts
// diagnostics with want_errors, or what its modules compile to with want or its ast
// form, or both; an absent want_errors is itself the assertion that nothing is
// reported. want_stages is additive and asserts neither on its own.
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
	case !tc.Failure() && !tc.Transform():
		return errors.New("expected 'want_errors', or 'want' where the modules compile; run `make generate` to fill one in")
	}

	if tc.Transform() {
		if err := tc.validateWant("want", tc.Want); err != nil {
			return err
		}
	}

	for _, stage := range tc.SortedStages() {
		if StageIndex(stage) < 0 {
			return fmt.Errorf("'want_stages' names %q, which is not a compiler stage the corpus knows", stage)
		}
		if err := tc.validateWant("want_stages."+stage, tc.WantStages[stage]); err != nil {
			return err
		}
	}

	for _, ref := range tc.SortedSchemas() {
		if strings.TrimSpace(ref) == "" {
			return errors.New("'schemas' has an entry with no reference")
		}
		var doc any
		if err := json.Unmarshal([]byte(tc.Schemas[ref]), &doc); err != nil {
			return fmt.Errorf("'schemas[%s]' is not a JSON Schema document: %w", ref, err)
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

// validateWant checks one list of expectations, whether it is the full-pipeline
// want or the one pinned to a stage. field names it for the error.
func (tc TestCase) validateWant(field string, want []Want) error {
	if len(want) == 0 {
		return fmt.Errorf("'%s' has no entries; run `make generate` to fill it in", field)
	}

	if len(want) != len(tc.Modules) {
		return fmt.Errorf("'%s' has %d entries for %d modules; it takes one per module, in the same order",
			field, len(want), len(tc.Modules))
	}

	for i, w := range want {
		at := fmt.Sprintf("%s[%d]", field, i)

		switch {
		case w.Module == "" && w.AST == "":
			return fmt.Errorf("'%s' has neither 'module' nor 'ast'", at)
		case w.Module != "" && w.AST != "":
			return fmt.Errorf("'%s' has both 'module' and 'ast', which are two spellings of one assertion", at)
		case w.AST != "" && len(w.Imports) > 0:
			return fmt.Errorf("'%s.imports' only applies to an entry asserting 'module'", at)
		}

		if err := conformance.CheckTrailingWhitespace(at+".module", w.Module); err != nil {
			return err
		}
		if _, err := tc.wantOptions(at, w); err != nil {
			return err
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
	if i >= len(tc.Want) {
		return WantOptions{RegoVersion: tc.RegoVersion}, nil
	}
	return tc.wantOptions(fmt.Sprintf("want[%d]", i), tc.Want[i])
}

// WantStageParserOptions is WantParserOptions for the i-th entry of the assertion
// pinned to stage.
func (tc TestCase) WantStageParserOptions(stage string, i int) (WantOptions, error) {
	want := tc.WantStages[stage]
	if i >= len(want) {
		return WantOptions{RegoVersion: tc.RegoVersion}, nil
	}
	return tc.wantOptions(fmt.Sprintf("want_stages.%s[%d]", stage, i), want[i])
}

func (tc TestCase) wantOptions(at string, w Want) (WantOptions, error) {
	out := WantOptions{RegoVersion: tc.RegoVersion}

	for _, imp := range w.Imports {
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
				return WantOptions{}, fmt.Errorf("unrecognised '%s.imports' entry %q", at, imp)
			}
			out.FutureKeywords = append(out.FutureKeywords, kw)

		default:
			return WantOptions{}, fmt.Errorf("unrecognised '%s.imports' entry %q; "+
				"expected rego.v1, future.keywords or future.keywords.<keyword>", at, imp)
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
