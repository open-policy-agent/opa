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

	popts := ast.ParserOptions{RegoVersion: v}
	if tc.ExperimentalKeywords {
		popts.Capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(true))
	}
	return popts, nil
}

// caseDiagnostics compiles a case's modules and returns the diagnostics, sorted.
// It is the counterpart of compiledModules, which returns what those modules
// compiled to.
// A module that does not parse is an error rather than a diagnostic: parse
// behaviour belongs to the parser corpus, and this corpus does not re-assert it.
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

	// The order the compiler reports in is not part of the contract — the runner
	// matches as a set — but a generated file has to be stable, and a sorted one
	// reads in source order.
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

// compileCase parses and compiles a case's modules.
//
// Without lifting the error limit the compiler stops at CompileErrorLimitDefault
// and appends a "too many errors" diagnostic of its own, so a case with more than
// ten would record a truncated set. The runner lifts it for the same reason.
func compileCase(tc compilecases.TestCase, popts ast.ParserOptions) (*ast.Compiler, error) {
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

	c.Compile(modules)

	return c, nil
}

// compiledWant returns what each of a case's modules compiles to: Rego where OPA's
// printer can express it, and a marshalled AST where it cannot. Printing enters here
// and nowhere else — the comparison the runner performs is between ASTs, so this is a
// convenience for authoring a fixture, not part of the contract.
//
// The choice is made per module. A compiled form that has no Rego spelling does not
// drag its neighbours into the AST form with it.
//
// formatModule checks its own output rather than assuming it: OPA's printer does not
// always produce text that parses back to the AST it came from, and a fixture that
// did not round-trip would assert something the compiler never produced.
func compiledWant(tc compilecases.TestCase) ([]compilecases.Want, []string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, nil, err
	}

	compiled, err := compileCase(tc, popts)
	if err != nil {
		return nil, nil, err
	}

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

// reasonFor is the short form of why a module could not be written as Rego, for the
// comment the entry carries.
func reasonFor(err error) string {
	if np, ok := errors.AsType[notPrintableError](err); ok {
		return np.Reason()
	}
	return err.Error()
}

// wantParserOptions is how the i-th Want entry's Module has to be parsed, as the
// schema reads it.
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

// directiveImports returns the directive imports a case's modules carry, as
// written. The compiler resolves each away once it has taken effect, so the printed
// output depends on them with nothing left to say so.
//
// Every one is reported, whether or not the compiled form still depends on it.
// Deciding which are redundant would mean modelling what the compiler does to each,
// and a case that quietly under-declares is worse than one asking a consumer for a
// directive it did not need — the extra is harmless, the omission is a fixture
// nobody can parse.
//
// An import under `future` or `rego` that the schema cannot interpret is an error
// rather than something to skip: a new directive the harness does not know about
// would otherwise vanish from the fixture silently.
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

			// Interpreted by the schema, so that what the generator writes and what a
			// consumer reads cannot drift apart.
			probe := compilecases.TestCase{Modules: []string{src}, Want: []compilecases.Want{{Imports: []string{path}}}}
			if _, err := probe.WantParserOptions(0); err != nil {
				return nil, fmt.Errorf("%s: %w; teach the generator and the schema what it means", name, err)
			}

			out[i] = append(out[i], path)
		}

		slices.Sort(out[i])
	}

	// Always one list per module, empty where there is nothing to declare: the
	// caller indexes it, and an entry with no imports simply writes none.
	return out, nil
}

// importPath renders a directive import the way it is written, which Ref.String does
// not: it renders the components after the first in bracket notation, so
// `future.keywords.every` comes back as `future.keywords["every"]`.
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
