// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package corpusgen

import (
	"fmt"

	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/util"
)

// EntryPointRefs returns one entrypoint per document the modules define, sorted, with the
// ref to plan each from. A module of nothing but functions has none.
func EntryPointRefs(modules map[string]*ast.Module) ([]string, []*ast.Term, error) {
	set := ast.NewSet()

	for _, name := range util.KeysSorted(modules) {
		mod := modules[name]
		for _, rule := range mod.Rules {
			// A function is not a document: `data.test.f` names nothing, and the query a
			// plan is built from — `result = data.test.f` — is rejected as a function used
			// as a value. Planning the package instead would succeed and produce a plan
			// with no functions in it, which is worse than none: it asserts nothing while
			// counting as coverage.
			if len(rule.Head.Args) > 0 {
				continue
			}
			ref := mod.Package.Path.Extend(rule.Head.Ref().GroundPrefix())
			set.Add(ast.NewTerm(nameable(ref)))
		}
	}

	sorted := set.Sorted()
	names := make([]string, sorted.Len())
	refs := make([]*ast.Term, sorted.Len())

	for i := range sorted.Len() {
		term := sorted.Elem(i)
		ep, err := term.Value.(ast.Ref).Ptr()
		if err != nil {
			return nil, nil, fmt.Errorf("%s cannot be named as an entrypoint path: %w", term, err)
		}
		names[i], refs[i] = ep, term
	}

	return names, refs, nil
}

// nameable truncates ref where an entrypoint path cannot express it. An entrypoint is a
// slash-separated path of strings, so a rule like `p[1] := "x"` is planned as the document
// that holds it, `data.test.p`, rather than not at all.
func nameable(ref ast.Ref) ast.Ref {
	for i, term := range ref[1:] {
		if _, ok := term.Value.(ast.String); !ok {
			return ref[:i+1]
		}
	}
	return ref
}

// PlanEntryPoints plans one query per entrypoint, each asking for the document that
// entrypoint names. names and refs are what EntryPointRefs returned, or what a case
// authored in its place.
func PlanEntryPoints(compiled *ast.Compiler, names []string, refs []*ast.Term) (*ir.Policy, error) {
	result := ast.VarTerm("result")
	queries := make([]planner.QuerySet, len(refs))

	for i := range refs {
		qc := compiled.QueryCompiler()
		query, err := qc.Compile(ast.NewBody(ast.Equality.Expr(result, refs[i])))
		if err != nil {
			return nil, fmt.Errorf("compiling the query for %s: %w", names[i], err)
		}
		queries[i] = planner.QuerySet{
			Name:          names[i],
			Queries:       []ast.Body{query},
			RewrittenVars: qc.RewrittenVars(),
		}
	}

	return Plan(compiled, queries)
}

// Plan plans queries against the compiled modules.
//
// The builtin declarations are OPA's own, not the consumer's: a plan calls whatever the
// module calls, so narrowing them here would fail generation rather than describe the
// consumer. Saying which built-ins are available is what a capabilities filter is for, and
// it filters the finished plan.
func Plan(compiled *ast.Compiler, queries []planner.QuerySet) (*ir.Policy, error) {
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
