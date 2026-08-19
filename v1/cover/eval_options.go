// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

// NoIndexingEvalOptions returns the rego.EvalOptions for a supplementary
// pass that traces into cov with rule indexing disabled, replaying the
// non-deterministic builtin results captured in ndbc.
func NoIndexingEvalOptions(cov *Cover, ndbc builtins.NDBCache) []rego.EvalOption {
	return supplementaryEvalOptions(cov, ndbc, rego.EvalRuleIndexing(false))
}

// NoEarlyExitEvalOptions is NoIndexingEvalOptions' early-exit-disabled
// counterpart.
func NoEarlyExitEvalOptions(cov *Cover, ndbc builtins.NDBCache) []rego.EvalOption {
	return supplementaryEvalOptions(cov, ndbc, rego.EvalEarlyExit(false))
}

// supplementaryEvalOptions returns the options common to every supplementary
// coverage pass: it traces into cov, and suppresses print output and builtin
// errors so they don't leak into the baseline pass. When ndbc is non-nil the
// pass shares the baseline's non-deterministic builtin cache so non-determinism
// doesn't produce different coverage between passes.
func supplementaryEvalOptions(cov *Cover, ndbc builtins.NDBCache, extra ...rego.EvalOption) []rego.EvalOption {
	opts := []rego.EvalOption{
		rego.EvalQueryTracer(cov),
		rego.EvalPrintHook(nil),
		rego.EvalBuiltinErrorList(nil),
	}
	if ndbc != nil {
		opts = append(opts, rego.EvalNDBuiltinCache(ndbc))
	}
	return append(opts, extra...)
}
