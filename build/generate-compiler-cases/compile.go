// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
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

	modules := make(map[string]*ast.Module, len(tc.Modules))
	for i, module := range tc.Modules {
		name := compilecases.ModuleName(i)
		parsed, perr := ast.ParseModuleWithOpts(name, module, popts)
		if perr != nil {
			return nil, fmt.Errorf("%s does not parse: %w", name, perr)
		}
		modules[name] = parsed
	}

	// Without this the compiler stops at CompileErrorLimitDefault and appends a
	// "too many errors" diagnostic of its own, so a case with more than ten would
	// record a truncated set. The runner lifts the limit for the same reason.
	c := ast.NewCompiler().
		SetErrorLimit(0).
		WithStrict(tc.StrictMode()).
		WithEnablePrintStatements(tc.PrintStatements)

	c.Compile(modules)

	reported := make([]conformance.Error, 0, len(c.Errors))
	for _, e := range c.Errors {
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

// compiledModules returns the Rego each of a case's modules compiles to, seeded
// by printing the compiled AST. Printing enters here and nowhere else: the
// comparison the runner performs is between ASTs, so this is a convenience for
// authoring a fixture, not part of the contract.
//
// formatModule checks its own output rather than assuming it: OPA's printer does
// not always produce text that parses back to the AST it came from, and a fixture
// that did not round-trip would assert something the compiler never produced.
func compiledModules(tc compilecases.TestCase) ([]string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, err
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

	c.Compile(modules)

	out := make([]string, 0, len(tc.Modules))
	for i := range tc.Modules {
		name := compilecases.ModuleName(i)

		text, ferr := formatModule(c.Modules[name], popts)
		if ferr != nil {
			return nil, fmt.Errorf("'want_modules' cannot be seeded from the compiled %s: %w", name, ferr)
		}

		out = append(out, strings.TrimRight(text, "\n"))
	}

	return out, nil
}
