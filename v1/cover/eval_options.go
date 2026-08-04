// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import "github.com/open-policy-agent/opa/v1/rego"

// NoIndexingEvalOptions returns the rego.EvalOptions for a supplementary
// pass that traces into cov with rule indexing disabled. Print output and
// builtin errors are suppressed so they don't leak into the baseline pass.
func NoIndexingEvalOptions(cov *Cover) []rego.EvalOption {
	return []rego.EvalOption{
		rego.EvalQueryTracer(cov),
		rego.EvalRuleIndexing(false),
		rego.EvalPrintHook(nil),
		rego.EvalBuiltinErrorList(nil),
	}
}

// NoEarlyExitEvalOptions is NoIndexingEvalOptions' early-exit-disabled
// counterpart.
func NoEarlyExitEvalOptions(cov *Cover) []rego.EvalOption {
	return []rego.EvalOption{
		rego.EvalQueryTracer(cov),
		rego.EvalEarlyExit(false),
		rego.EvalPrintHook(nil),
		rego.EvalBuiltinErrorList(nil),
	}
}
