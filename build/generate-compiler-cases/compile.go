// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

func parserOptions(tc compilecases.TestCase) (ast.ParserOptions, error) {
	v, err := corpusgen.RegoVersion(tc.RegoVersion)
	if err != nil {
		return ast.ParserOptions{}, err
	}

	popts := ast.ParserOptions{
		RegoVersion: v,
		// Schema annotations are only honoured when they were parsed as annotations, so
		// attaching schemas asks for that too.
		ProcessAnnotation: len(tc.Schemas) > 0,
	}
	if tc.ExperimentalKeywords {
		popts.Capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(true))
	}
	return popts, nil
}

// caseDiagnostics compiles a case's modules and returns the diagnostics, sorted. A
// module that does not parse is an error rather than a diagnostic: parse behaviour
// belongs to the parser corpus.
func caseDiagnostics(tc compilecases.TestCase) ([]conformance.Error, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, err
	}

	compiled, err := compileCase(tc, popts)
	if err != nil {
		return nil, err
	}

	reported := make([]conformance.Error, 0, len(compiled.Errors))
	for _, e := range compiled.Errors {
		reported = append(reported, caseError(e))
	}

	// The runner matches as a set, but a generated file has to be stable.
	slices.SortFunc(reported, func(a, b conformance.Error) int {
		return cmp.Or(
			cmp.Compare(a.ModuleOrDefault(), b.ModuleOrDefault()),
			cmp.Compare(a.Row, b.Row),
			cmp.Compare(a.Col, b.Col),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Message, b.Message),
		)
	})

	return reported, nil
}

func caseError(e *ast.Error) conformance.Error {
	out := conformance.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Module = e.Location.File
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	return out
}

// compileCase parses and compiles a case's modules. The error limit is lifted, or a
// case with more than ten diagnostics would record a truncated set plus OPA's own
// "too many errors"; the runner lifts it too.
func compileCase(tc compilecases.TestCase, popts ast.ParserOptions) (*ast.Compiler, error) {
	return compileCaseToStage(tc, popts, "")
}

// compileCaseToStage is compileCase, stopping after stage when one is named.
//
// The stage is checked against the compiler's own list first. WithOnlyStagesUpTo
// runs the whole pipeline when it does not recognise its argument, so a name that
// has drifted would otherwise record the full-pipeline form under a stage that no
// longer exists — and the difference gate would then drop the assertion as
// redundant. Failing here is what keeps a rename from quietly deleting coverage.
func compileCaseToStage(tc compilecases.TestCase, popts ast.ParserOptions, stage string) (*ast.Compiler, error) {
	if stage != "" && !slices.Contains(ast.AllStages(), ast.StageID(stage)) {
		return nil, fmt.Errorf("%q is not one of the compiler's stages", stage)
	}

	modules := make(map[string]*ast.Module, len(tc.Modules))
	for i, src := range tc.Modules {
		name := compilecases.ModuleName(i)
		parsed, perr := ast.ParseModuleWithOpts(name, src, popts)
		if perr != nil {
			return nil, fmt.Errorf("%s does not parse: %w", name, perr)
		}
		modules[name] = parsed
	}

	c := ast.NewCompiler().
		SetErrorLimit(0).
		WithStrict(tc.StrictMode()).
		WithEnablePrintStatements(tc.PrintStatements)

	ss, serr := schemaSet(tc)
	if serr != nil {
		return nil, serr
	}
	if ss != nil {
		c = c.WithSchemas(ss).WithUseTypeCheckAnnotations(true)
	}

	if stage != "" {
		c = c.WithOnlyStagesUpTo(ast.StageID(stage))
	}

	c.Compile(modules)

	return c, nil
}

// schemaSet builds the case's schemas, or nil where it carries none.
func schemaSet(tc compilecases.TestCase) (*ast.SchemaSet, error) {
	if len(tc.Schemas) == 0 {
		return nil, nil
	}

	ss := ast.NewSchemaSet()
	for _, path := range tc.SortedSchemas() {
		ref, err := ast.ParseRef(path)
		if err != nil {
			return nil, fmt.Errorf("schemas names %q, which is not a ref: %w", path, err)
		}

		var doc any
		if err := json.Unmarshal([]byte(tc.Schemas[path]), &doc); err != nil {
			return nil, fmt.Errorf("schemas[%s]: %w", path, err)
		}
		ss.Put(ref, doc)
	}

	return ss, nil
}

// compiledWant returns what each of a case's modules compiles to: Rego where OPA's
// printer can express it, marshalled AST where it cannot, chosen per module.
// formatModule checks its own output, so a fixture never asserts something the
// compiler did not produce.
func compiledWant(tc compilecases.TestCase) ([]compilecases.Want, []string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, nil, err
	}

	compiled, err := compileCase(tc, popts)
	if err != nil {
		return nil, nil, err
	}

	return wantFor(tc, popts, compiled)
}

// compiledWantAtStage is compiledWant for the modules as they stand once the
// pipeline stops after stage.
//
// A diagnostic reported before the stage is reached is an error rather than
// something to record: the field asserts a form, and there is only one of those if
// the pipeline got that far cleanly. Failing generation is the point — a case that
// cannot reach the stage it pins is a corpus defect, and finding out at load time
// would turn it into a silent skip.
func compiledWantAtStage(tc compilecases.TestCase, stage string) ([]compilecases.Want, []string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, nil, err
	}

	compiled, err := compileCaseToStage(tc, popts, stage)
	if err != nil {
		return nil, nil, err
	}

	if len(compiled.Errors) > 0 {
		return nil, nil, fmt.Errorf("compiling up to %s reports %d diagnostic(s), starting with %s",
			stage, len(compiled.Errors), compiled.Errors[0])
	}

	return wantFor(tc, popts, compiled)
}

func wantFor(tc compilecases.TestCase, popts ast.ParserOptions, compiled *ast.Compiler) ([]compilecases.Want, []string, error) {
	imports, err := directiveImports(tc, popts)
	if err != nil {
		return nil, nil, err
	}

	want := make([]compilecases.Want, len(tc.Modules))
	reasons := make([]string, len(tc.Modules))

	for i := range tc.Modules {
		name := compilecases.ModuleName(i)
		mod := compiled.Modules[name]

		wantOpts, oerr := wantParserOptions(tc, imports, i)
		if oerr != nil {
			return nil, nil, oerr
		}

		text, ferr := formatModule(mod, wantOpts)
		if ferr == nil {
			want[i] = compilecases.Want{
				Module:  strings.TrimRight(text, "\n") + "\n",
				Imports: imports[i],
			}
			continue
		}

		marshalled, merr := marshalModule(mod)
		if merr != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, merr)
		}

		want[i] = compilecases.Want{AST: marshalled}
		reasons[i] = reasonFor(ferr)
	}

	return want, reasons, nil
}

// reasonFor is the short form of why, for the comment the entry carries.
func reasonFor(err error) string {
	if np, ok := errors.AsType[notPrintableError](err); ok {
		return np.Reason()
	}
	return err.Error()
}

// wantParserOptions is how the i-th entry's Module has to be parsed, as the schema
// reads it.
func wantParserOptions(tc compilecases.TestCase, imports [][]string, i int) (ast.ParserOptions, error) {
	with := tc
	with.Want = make([]compilecases.Want, len(imports))
	for j := range imports {
		with.Want[j] = compilecases.Want{Imports: imports[j]}
	}

	opts, err := with.WantParserOptions(i)
	if err != nil {
		return ast.ParserOptions{}, err
	}

	version, err := corpusgen.RegoVersion(opts.RegoVersion)
	if err != nil {
		return ast.ParserOptions{}, err
	}

	popts := ast.ParserOptions{
		RegoVersion:       version,
		FutureKeywords:    opts.FutureKeywords,
		AllFutureKeywords: opts.AllFutureKeywords,
		ProcessAnnotation: true,
	}
	if tc.ExperimentalKeywords {
		popts.Capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(true))
	}

	return popts, nil
}

// marshalModule renders a compiled module as the AST form of a Want entry.
func marshalModule(mod *ast.Module) (string, error) {
	restore := astJSON.GetOptions()
	astJSON.SetOptions(conformance.MarshalOptions(false, false))
	defer astJSON.SetOptions(restore)

	mod.Comments = nil

	bs, err := json.Marshal(mod)
	if err != nil {
		return "", err
	}

	return conformance.FormatAST(bs)
}

// directiveImports returns the directive imports each of a case's modules carries, as
// written, one list per module.
//
// Every one is reported, whether or not the compiled form still depends on it:
// working out which are redundant would mean modelling what the compiler does to
// each, and under-declaring leaves a fixture nobody can parse. An import the schema
// cannot interpret is an error, so a new directive cannot vanish silently.
func directiveImports(tc compilecases.TestCase, popts ast.ParserOptions) ([][]string, error) {
	out := make([][]string, len(tc.Modules))

	for i, src := range tc.Modules {
		name := compilecases.ModuleName(i)

		mod, err := ast.ParseModuleWithOpts(name, src, popts)
		if err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", name, err)
		}

		for _, imp := range mod.Imports {
			ref, ok := imp.Path.Value.(ast.Ref)
			if !ok || len(ref) == 0 {
				continue
			}
			if !ast.FutureRootDocument.Equal(ref[0]) && !ast.RegoRootDocument.Equal(ref[0]) {
				continue
			}

			path, ok := importPath(ref)
			if !ok {
				return nil, fmt.Errorf("%s: cannot read the import path `%s`; teach the generator what it means", name, ref)
			}
			if slices.Contains(out[i], path) {
				continue
			}

			// Interpreted by the schema, so writer and reader cannot drift apart.
			probe := compilecases.TestCase{Modules: []string{src}, Want: []compilecases.Want{{Imports: []string{path}}}}
			if _, err := probe.WantParserOptions(0); err != nil {
				return nil, fmt.Errorf("%s: %w; teach the generator and the schema what it means", name, err)
			}

			out[i] = append(out[i], path)
		}

		slices.Sort(out[i])
	}

	return out, nil
}

// importPath renders a directive import as written. Ref.String uses bracket notation
// for the components after the first.
func importPath(ref ast.Ref) (string, bool) {
	v, ok := ref[0].Value.(ast.Var)
	if !ok {
		return "", false
	}

	parts := []string{string(v)}
	for _, term := range ref[1:] {
		s, ok := term.Value.(ast.String)
		if !ok {
			return "", false
		}
		parts = append(parts, string(s))
	}

	return strings.Join(parts, "."), true
}
