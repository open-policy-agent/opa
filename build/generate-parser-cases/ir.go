// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
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

			if tc.BodyCase() {
				tc.IRError = "no plan for a body case: there is no module to compile"
				continue
			}

			node, err := parseCase(tc.TestCase)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
			}
			module := node.(*ast.Module)

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
	popts, err := parserOptions(tc.TestCase)
	if err != nil {
		return nil, nil, err
	}

	c := ast.NewCompiler().
		WithDefaultRegoVersion(popts.RegoVersion).
		WithCapabilities(popts.Capabilities)
	c.Compile(map[string]*ast.Module{parsercases.DefaultModuleName: module})
	if c.Failed() {
		return nil, nil, c.Errors
	}

	entrypoints, refs, err := entrypointRefs(tc.EntryPoints, c.Modules)
	if err != nil {
		return nil, nil, err
	}
	if len(entrypoints) == 0 {
		return nil, nil, errors.New("no entrypoints: every rule is a function, and a function is not a document")
	}

	policy, err := corpusgen.PlanEntryPoints(c, entrypoints, refs)
	if err != nil {
		return nil, nil, err
	}

	return entrypoints, policy, nil
}

// entrypointRefs returns the entrypoints to plan for, which are the documents the module
// defines unless the case authored an override.
func entrypointRefs(authored []string, modules map[string]*ast.Module) ([]string, []*ast.Term, error) {
	if len(authored) == 0 {
		return corpusgen.EntryPointRefs(modules)
	}

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
