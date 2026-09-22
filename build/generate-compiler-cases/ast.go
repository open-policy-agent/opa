// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// errNotMarshallable marks a compiled form OPA itself cannot marshal, which is a
// defect in OPA rather than in the case: a number written `.14` is stored as written
// and emitted as invalid JSON. Recorded on the case rather than failing the load, so
// one such module does not deny the whole corpus to a consumer reading ASTs.
var errNotMarshallable = errors.New("OPA cannot marshal the compiled form")

// generateAST fills the marshalled form of every expectation a case carries: each
// want entry, each want_stages entry, and a query case's compiled query.
//
// Additive — a Rego spelling stays where the case has one — and nothing lands in the
// corpus. It is what makes the corpus usable by an implementation that has an AST but
// neither a printer for OPA's canonical Rego nor a planner.
//
// The marshalling options are global state, so they are set once for the pass.
func generateAST(sets []CompilerSet) error {
	defer corpusgen.SetMarshalOptions(conformance.MarshalOptions(false, false))()

	for _, set := range sets {
		for _, tc := range set.Cases {
			err := caseAST(tc)
			if errors.Is(err, errNotMarshallable) {
				tc.ASTError = err.Error()
				continue
			}
			if err != nil {
				return fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
			}
		}
	}

	return nil
}

func caseAST(tc *CompilerTestCase) error {
	if tc.QueryCase() {
		return queryAST(tc)
	}

	if tc.Transform() {
		if err := fillAST(tc.TestCase, "", tc.Want); err != nil {
			return fmt.Errorf("want: %w", err)
		}
	}

	for _, stage := range tc.SortedStages() {
		if err := fillAST(tc.TestCase, stage, tc.WantStages[stage]); err != nil {
			return fmt.Errorf("want_stages.%s: %w", stage, err)
		}
	}

	return nil
}

// fillAST marshals what the case's modules compile to onto want, which the caller took
// from the case itself, so the entries stay one per module.
func fillAST(tc compilecases.TestCase, stage string, want []compilecases.Want) error {
	marshalled, err := marshalledModules(tc, stage)
	if err != nil {
		return err
	}

	for i := range want {
		want[i].AST = marshalled[i]
	}

	return nil
}

// marshalledModules is what each of a case's modules compiles to, marshalled, one per
// module. stage is empty for the whole pipeline.
//
// A diagnostic here is an error rather than something to record: a case asserting a
// compiled form has none, and one that cannot reach the stage it pins is a corpus
// defect the generator would already have refused to write.
func marshalledModules(tc compilecases.TestCase, stage string) ([]string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return nil, err
	}

	compiled, err := compileCaseToStage(tc, popts, stage)
	if err != nil {
		return nil, err
	}

	if len(compiled.Errors) > 0 {
		return nil, fmt.Errorf("compiling reports %d diagnostic(s), starting with %s",
			len(compiled.Errors), compiled.Errors[0])
	}

	out := make([]string, len(tc.Modules))
	for i := range tc.Modules {
		name := compilecases.ModuleName(i)

		marshalled, err := encodeModule(compiled.Modules[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w: %w", name, errNotMarshallable, err)
		}
		out[i] = marshalled
	}

	return out, nil
}

// queryAST marshals what the case's query compiles to. A query that reports
// diagnostics has no compiled form, and asserts those instead.
func queryAST(tc *CompilerTestCase) error {
	if tc.Query.Want == "" && tc.Query.WantAST == "" {
		return nil
	}

	body, err := compiledQueryBody(tc.TestCase)
	if err != nil {
		return err
	}

	marshalled, err := encodeBody(body)
	if err != nil {
		return fmt.Errorf("%w: %w", errNotMarshallable, err)
	}

	tc.Query.WantAST = marshalled

	return nil
}
