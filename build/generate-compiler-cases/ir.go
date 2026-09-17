// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/util"
)

// QueryEntryPoint is the name given to the plan of a query case's query, which has no
// ref to be named after.
const QueryEntryPoint = "query"

// generateIR plans every case that asserts a compiled form, populating WantIR and
// EntryPoints where that succeeds. EntryPoints is filled in even though no case
// authors it, so that a consumer never has to derive it.
//
// A case that does not plan gets no IR and records the reason on IRError. That reason
// is diagnostic, not an assertion: a module of nothing but functions has no entrypoint,
// and the planner rejects constructs it has never supported. Only a failure to read the
// case itself is an error.
//
// A case asserting diagnostics is skipped silently — there is no compiled form to plan.
func generateIR(sets []CompilerSet) error {
	for _, set := range sets {
		for _, tc := range set.Cases {
			if !tc.Transform() && !queryCompiles(tc.TestCase) {
				continue
			}

			entrypoints, policy, err := planCase(tc.TestCase)
			if err != nil {
				tc.IRError = err.Error()
				continue
			}

			tc.EntryPoints, tc.WantIR = entrypoints, policy
		}
	}

	return nil
}

// queryCompiles reports whether tc is a query case asserting what its query compiles
// to, rather than the diagnostics compiling it produced.
func queryCompiles(tc compilecases.TestCase) bool {
	return tc.QueryCase() && (tc.Query.Want != "" || tc.Query.WantAST != "")
}

func planCase(tc compilecases.TestCase) ([]string, *ir.Policy, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, nil, err
	}

	compiled, err := compileCase(tc, popts)
	if err != nil {
		return nil, nil, err
	}
	if len(compiled.Errors) > 0 {
		return nil, nil, fmt.Errorf("compiling reports %d diagnostic(s), starting with %s",
			len(compiled.Errors), compiled.Errors[0])
	}

	if tc.QueryCase() {
		return planQuery(tc, compiled)
	}

	return planModules(compiled)
}

// planModules plans one entrypoint per document the case's modules define.
func planModules(compiled *ast.Compiler) ([]string, *ir.Policy, error) {
	entrypoints, refs, err := entrypointRefs(compiled)
	if err != nil {
		return nil, nil, err
	}
	if len(entrypoints) == 0 {
		return nil, nil, errors.New("no entrypoints: every rule is a function, and a function is not a document")
	}

	result := ast.VarTerm("result")
	queries := make([]planner.QuerySet, len(refs))

	for i := range refs {
		qc := compiled.QueryCompiler()
		query, err := qc.Compile(ast.NewBody(ast.Equality.Expr(result, refs[i])))
		if err != nil {
			return nil, nil, fmt.Errorf("compiling the query for %s: %w", entrypoints[i], err)
		}
		queries[i] = planner.QuerySet{
			Name:          entrypoints[i],
			Queries:       []ast.Body{query},
			RewrittenVars: qc.RewrittenVars(),
		}
	}

	policy, err := plan(compiled, queries)
	if err != nil {
		return nil, nil, err
	}

	return entrypoints, policy, nil
}

// planQuery plans the case's query, which is what a query case asserts; its modules are
// the environment the plan is generated against.
func planQuery(tc compilecases.TestCase, compiled *ast.Compiler) ([]string, *ir.Policy, error) {
	qc, body, qerrs, err := compileQuery(tc, compiled)
	if err != nil {
		return nil, nil, err
	}
	if len(qerrs) > 0 {
		return nil, nil, fmt.Errorf("the query reports %d diagnostic(s), starting with %s", len(qerrs), qerrs[0])
	}

	policy, err := plan(compiled, []planner.QuerySet{{
		Name:          QueryEntryPoint,
		Queries:       []ast.Body{body},
		RewrittenVars: qc.RewrittenVars(),
	}})
	if err != nil {
		return nil, nil, err
	}

	return []string{QueryEntryPoint}, policy, nil
}

// plan plans queries against the compiled modules.
//
// The builtin declarations are OPA's own, not the consumer's: a plan calls whatever the
// module calls, so narrowing them here would fail generation rather than describe the
// consumer. Saying which built-ins are available is what CapabilitiesFilter is for, and
// it filters the finished plan.
func plan(compiled *ast.Compiler, queries []planner.QuerySet) (*ir.Policy, error) {
	modules := make([]*ast.Module, 0, len(compiled.Modules))
	for _, name := range util.KeysSorted(compiled.Modules) {
		modules = append(modules, compiled.Modules[name])
	}

	caps := ast.CapabilitiesForThisVersion()
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
		return nil, fmt.Errorf("planning: %w", err)
	}

	return policy, nil
}

// entrypointRefs returns one entrypoint per ground rule ref across the case's compiled
// modules, sorted, with the refs to plan them from.
//
// A function is not one: `data.test.f` names no document, and the query the plan is
// built from — `result = data.test.f` — is rejected with "function data.test.f used as
// value". A module of nothing but functions therefore has no entrypoint. Planning its
// package instead would succeed and produce a plan with no functions in it, which is
// worse than none: it asserts nothing while counting as coverage.
func entrypointRefs(compiled *ast.Compiler) ([]string, []*ast.Term, error) {
	set := ast.NewSet()

	for _, name := range util.KeysSorted(compiled.Modules) {
		mod := compiled.Modules[name]
		for _, rule := range mod.Rules {
			if len(rule.Head.Args) > 0 {
				continue
			}
			ref := mod.Package.Path.Extend(rule.Head.Ref().GroundPrefix())
			set.Add(ast.NewTerm(nameable(ref)))
		}
	}

	sorted := set.Sorted()
	entrypoints := make([]string, sorted.Len())
	refs := make([]*ast.Term, sorted.Len())

	for i := range sorted.Len() {
		term := sorted.Elem(i)
		ep, err := term.Value.(ast.Ref).Ptr()
		if err != nil {
			return nil, nil, fmt.Errorf("%s cannot be named as an entrypoint path: %w", term, err)
		}
		entrypoints[i], refs[i] = ep, term
	}

	return entrypoints, refs, nil
}

// nameable truncates ref where an entrypoint path cannot express it. An entrypoint is a
// slash-separated path of strings, so a rule like `p[1] := "x"` is planned as the
// document that holds it, `data.test.p`, rather than not at all.
func nameable(ref ast.Ref) ast.Ref {
	for i, term := range ref[1:] {
		if _, ok := term.Value.(ast.String); !ok {
			return ref[:i+1]
		}
	}
	return ref
}
