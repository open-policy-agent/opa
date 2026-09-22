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
	"github.com/open-policy-agent/opa/v1/test/compilecases"
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
	entrypoints, refs, err := corpusgen.EntryPointRefs(compiled.Modules)
	if err != nil {
		return nil, nil, err
	}
	if len(entrypoints) == 0 {
		return nil, nil, errors.New("no entrypoints: every rule is a function, and a function is not a document")
	}

	policy, err := corpusgen.PlanEntryPoints(compiled, entrypoints, refs)
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

	policy, err := corpusgen.Plan(compiled, []planner.QuerySet{{
		Name:          QueryEntryPoint,
		Queries:       []ast.Body{body},
		RewrittenVars: qc.RewrittenVars(),
	}})
	if err != nil {
		return nil, nil, err
	}

	return []string{QueryEntryPoint}, policy, nil
}
