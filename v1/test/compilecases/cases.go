// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compilecases contains the schema and loader for the compiler conformance corpus:
// Rego modules in, and either the modules they compile to or the diagnostics compiling them
// produces. See README.md.
//
// The package implements no Rego: it holds the schema and reads the corpus, and nothing
// here parses or compiles. That is what lets validation stay independent of the
// implementation under test — a case OPA cannot parse is a failing test, not an invalid
// case — and it keeps the parser and compiler out of a consumer's dependencies. It is
// enforced by the runner living in package ast, which importing this package back would
// cycle. OPA's own runner is there, and the generator that fills in the fixtures is in
// build/generate-compiler-cases.
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

// RegoVersions are the rego versions a corpus can hold, which is the set of legal version
// directory names at its root.
var RegoVersions = conformance.RegoVersions

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
	Filename        string   `json:"-"                                yaml:"-"`                          // name of file that case was loaded from
	Note            string   `json:"note"                             yaml:"note"`                       // identifies the case, unique within its version directory
	Modules         []string `json:"modules,omitempty"                yaml:"modules,omitempty"`          // policies to compile, named test-0.rego, test-1.rego, ...
	Strict          string   `json:"strict,omitempty"                 yaml:"strict,omitempty"`           // enabled, disabled, or absent where strict mode does not change the outcome
	PrintStatements bool     `json:"print_statements,omitempty"       yaml:"print_statements,omitempty"` // keep print() calls instead of erasing them, as required to reach diagnostics about their operands

	// RegoVersion is the version the modules are parsed as, taken from the version
	// directory the case was loaded from rather than authored. Tagged out of the schema so
	// that a case stating one is rejected as an unknown field: the directory is the only
	// place it is said.
	RegoVersion string `json:"-"  yaml:"-"`

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
		// Stamped from the version directory, so this only fires on a case built by hand.
		return fmt.Errorf("unknown rego version %q, expected one of %v", tc.RegoVersion, RegoVersions)
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
		if _, err := w.ParserOptions(at, tc.WantBaseOptions()); err != nil {
			return err
		}
	}

	return nil
}

// WantOptions is how a Want entry's Module has to be parsed. Stated without
// reference to v1/ast, so a consumer can map it onto its own parser.
type WantOptions = conformance.ParseOptions

// ParseOptions is how tc's modules have to be read.
func (tc TestCase) ParseOptions() WantOptions {
	return WantOptions{
		RegoVersion: tc.RegoVersion,

		// Schema annotations are only honoured when they were parsed as annotations, so
		// attaching schemas asks for that too.
		ProcessAnnotations: len(tc.Schemas) > 0,
	}
}

// WantParserOptions is how the i-th entry of an expectation has to be parsed: the case's
// options with the entry's own directive imports folded in. stage names a want_stages key,
// or is empty for the full-pipeline want. An import it does not recognise is an error rather
// than a no-op.
func (tc TestCase) WantParserOptions(stage string, i int) (WantOptions, error) {
	want, at := tc.Want, "want"
	if stage != "" {
		want, at = tc.WantStages[stage], "want_stages."+stage
	}

	if i >= len(want) {
		return tc.WantBaseOptions(), nil
	}
	return want[i].ParserOptions(fmt.Sprintf("%s[%d]", at, i), tc.WantBaseOptions())
}

// QueryParserOptions are the options query.body and query.want are read with: the
// modules' own, plus the directives among the query's imports — the same list that is in
// scope for it, and the same derivation OPA makes for a query handed to it with
// `--import`.
func (tc TestCase) QueryParserOptions() (WantOptions, error) {
	out := tc.ParseOptions()
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

// WantBaseOptions is how an expected module is read before its own imports are folded in.
// Annotations always, since the compiler builds them from METADATA comments and
// Module.Compare compares them, so an expectation carrying one has to be read with them
// processed.
func (tc TestCase) WantBaseOptions() WantOptions {
	out := tc.ParseOptions()
	out.ProcessAnnotations = true
	return out
}

// ParserOptions is how w's Module has to be parsed: base, which is the case's
// WantBaseOptions, with w's own directive imports folded in. at names the entry for the
// error message.
//
// On Want rather than on the case because that is what it needs: a caller holding one entry
// does not have to fabricate a case around it to ask how the entry reads.
func (w Want) ParserOptions(at string, base WantOptions) (WantOptions, error) {
	out := base

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

// LoadFSByFile is LoadFS with the cases kept grouped by the file they came from.
func LoadFSByFile(fsys fs.FS, root string) ([]Set, error) {
	return conformance.LoadFSByFile[TestCase](fsys, root)
}
