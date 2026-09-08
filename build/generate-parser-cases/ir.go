// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
	"github.com/open-policy-agent/opa/v1/util"
)

// generateIR compiles and plans every success case, populating WantIR and
// EntryPoints where that succeeds. EntryPoints is filled in even where the case
// authored no override, so that a consumer never has to derive it.
//
// A case that does not compile or does not plan gets no IR, and the reason is
// recorded on IRError; only a failure to read the case itself is an error. That
// reason is diagnostic, not an assertion: most of these are artifacts of the
// module a term or expression case was wrapped in — `p := <term>` leaves a free
// variable unsafe — rather than anything the case set out to say. Compiler
// diagnostics that are worth asserting are authored deliberately, in
// v1/test/compilecases.
func generateIR(sets []ParserSet) error {
	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.Failure() {
				continue
			}

			module, err := parseModule(tc.TestCase, tc.Module)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
			}

			entrypoints, policy, err := plan(tc, module)
			if err != nil {
				tc.IRError = err.Error()
				continue
			}

			tc.EntryPoints = entrypoints
			tc.WantIR = policy
		}
	}

	return nil
}

func plan(tc *ParserTestCase, module *ast.Module) ([]string, *ir.Policy, error) {
	v, err := corpusgen.RegoVersion(tc.RegoVersion)
	if err != nil {
		return nil, nil, err
	}

	caps := capabilities(tc.TestCase)

	c := ast.NewCompiler().
		WithDefaultRegoVersion(v).
		WithCapabilities(caps)
	c.Compile(map[string]*ast.Module{parsercases.DefaultModuleName: module})
	if c.Failed() {
		return nil, nil, c.Errors
	}

	entrypoints, refs, err := entrypointRefs(tc.EntryPoints, c.Modules[parsercases.DefaultModuleName])
	if err != nil {
		return nil, nil, err
	}
	if len(entrypoints) == 0 {
		return nil, nil, errors.New("no entrypoints")
	}

	result := ast.VarTerm("result")
	queries := make([]planner.QuerySet, len(refs))
	for i := range refs {
		qc := c.QueryCompiler()
		query, err := qc.Compile(ast.NewBody(ast.Equality.Expr(result, refs[i])))
		if err != nil {
			return nil, nil, err
		}
		queries[i] = planner.QuerySet{
			Name:          entrypoints[i],
			Queries:       []ast.Body{query},
			RewrittenVars: qc.RewrittenVars(),
		}
	}

	modules := make([]*ast.Module, 0, len(c.Modules))
	for _, name := range util.KeysSorted(c.Modules) {
		modules = append(modules, c.Modules[name])
	}

	builtins := make(map[string]*ast.Builtin, len(caps.Builtins))
	for _, bi := range caps.Builtins {
		builtins[bi.Name] = bi
	}

	policy, err := planner.New().
		WithQueries(queries).
		WithModules(modules).
		WithBuiltinDecls(builtins).
		Plan()
	if err != nil {
		return nil, nil, err
	}

	return entrypoints, policy, nil
}

// entrypointRefs returns one entrypoint per ground rule ref in the module,
// sorted, unless authored overrides them.
func entrypointRefs(authored []string, module *ast.Module) ([]string, []*ast.Term, error) {
	if len(authored) > 0 {
		refs := make([]*ast.Term, len(authored))
		for i, ep := range authored {
			ref, err := ast.PtrRef(ast.DefaultRootDocument.Copy(), ep)
			if err != nil {
				return nil, nil, err
			}
			refs[i] = ast.NewTerm(ref)
		}
		return authored, refs, nil
	}

	set := ast.NewSet()
	for _, rule := range module.Rules {
		set.Add(ast.NewTerm(module.Package.Path.Extend(rule.Head.Ref().GroundPrefix())))
	}

	sorted := set.Sorted()
	entrypoints := make([]string, sorted.Len())
	refs := make([]*ast.Term, sorted.Len())

	for i := range sorted.Len() {
		term := sorted.Elem(i)
		ep, err := term.Value.(ast.Ref).Ptr()
		if err != nil {
			return nil, nil, err
		}
		entrypoints[i] = ep
		refs[i] = term
	}

	return entrypoints, refs, nil
}
