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
	Modules              []string `json:"modules,omitempty"                yaml:"modules,omitempty"`               // policies to compile, named test-0.rego, test-1.rego, ...
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

	// Query, where present, makes this a query case: what it asserts is what the *query*
	// compiles to, and the modules are the environment it is compiled in. Nil for every
	// other case.
	Query *QuerySpec `json:"query,omitempty"  yaml:"query,omitempty"`

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

// QuerySpec is a query to compile against a case's modules, with the context it is
// compiled in and what it must compile to.
type QuerySpec struct {
	// Body is the query, as Rego.
	Body string `json:"body"  yaml:"body"`

	// Package is the package the query is compiled relative to.
	Package string `json:"package,omitempty"  yaml:"package,omitempty"`

	// Imports is in effect for Body and Want; including directive imports like future.keywords
	Imports []string `json:"imports,omitempty"  yaml:"imports,omitempty"`

	// Want is what compiling Body produces, as Rego. Compared as an AST, like
	// Want.Module: the text is only how it is written down. Absent where the case asserts
	// want_errors instead.
	//
	// Generated, not authored: run `make generate` and review the diff.
	Want string `json:"want,omitempty"  yaml:"want,omitempty"`

	// WantAST is the same assertion marshalled, for a compiled query with no Rego
	// spelling that parses back to it.
	WantAST string `json:"want_ast,omitempty"  yaml:"want_ast,omitempty"`
}

// Transform reports whether tc asserts what its modules compile to.
func (tc TestCase) Transform() bool {
	return len(tc.Want) > 0
}

// QueryCase reports whether tc compiles a query rather than asserting what its modules
// compile to.
func (tc TestCase) QueryCase() bool {
	return tc.Query != nil
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
	case len(tc.Modules) == 0 && !tc.QueryCase():
		return errors.New("missing 'modules'")
	case tc.RegoVersion != "" && !slices.Contains(RegoVersions, tc.RegoVersion):
		return fmt.Errorf("unknown 'rego_version' %q, expected one of %v", tc.RegoVersion, RegoVersions)
	case tc.Strict != "" && !slices.Contains(Strictnesses, tc.Strict):
		return fmt.Errorf("unknown 'strict' %q, expected one of %v, or absent where strict mode does not change the outcome",
			tc.Strict, Strictnesses)
	case tc.Exhaustive && !tc.Failure():
		return errors.New("'exhaustive' only applies to a case asserting 'want_errors'")
	case tc.QueryCase() && tc.Transform():
		return errors.New("a case compiling a 'query' asserts 'query.want', not 'want': its modules are the environment the query is compiled in")
	case tc.QueryCase() && tc.Query.Body == "":
		// Absent, not blank: a body of nothing but whitespace is a case in its own right.
		return errors.New("'query' needs a 'body' to compile")
	case tc.QueryCase() && tc.Query.Want != "" && tc.Query.WantAST != "":
		return errors.New("'query' has both 'want' and 'want_ast', which are two spellings of one assertion")
	case tc.QueryCase() && !tc.Failure() && tc.Query.Want == "" && tc.Query.WantAST == "":
		return errors.New("expected 'want_errors', or 'query.want' where the query compiles; run `make generate` to fill one in")
	case !tc.QueryCase() && !tc.Failure() && !tc.Transform():
		return errors.New("expected 'want_errors', or 'want' where the modules compile; run `make generate` to fill one in")
	}

	// Not query.body: the check exists so a module can be written as a block scalar, and a
	// query is a scalar either way. One that is nothing but whitespace is a case in its
	// own right — an empty query cannot be compiled.
	if tc.QueryCase() {
		if err := conformance.CheckTrailingWhitespace("query.want", tc.Query.Want); err != nil {
			return err
		}
		// A directive among the query's imports decides how its body reads, so a malformed
		// one is rejected here rather than at the parse it would silently change.
		if _, err := tc.QueryParserOptions(); err != nil {
			return err
		}
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
type WantOptions = conformance.ParseOptions

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

// QueryParserOptions are the options query.body and query.want are read with, taken from
// the directives among the query's own imports — the same list that is in scope for it,
// and the same derivation OPA makes for a query handed to it with `--import`.
func (tc TestCase) QueryParserOptions() (WantOptions, error) {
	out := WantOptions{RegoVersion: tc.RegoVersion}
	if !tc.QueryCase() {
		return out, nil
	}

	for _, imp := range tc.Query.Imports {
		// A non-directive is a ref in scope, which is not this function's business.
		if _, err := conformance.DirectiveOption("query.imports", imp, &out); err != nil {
			return WantOptions{}, err
		}
	}

	return out, nil
}

func (tc TestCase) wantOptions(at string, w Want) (WantOptions, error) {
	out := WantOptions{RegoVersion: tc.RegoVersion}

	for _, imp := range w.Imports {
		directive, err := conformance.DirectiveOption(at+".imports", imp, &out)
		if err != nil {
			return WantOptions{}, err
		}
		if !directive {
			return WantOptions{}, conformance.UnknownDirective(at+".imports", imp)
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
