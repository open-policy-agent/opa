// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/internal/configpolicy"
	"github.com/open-policy-agent/opa/v1/ast"
)

// Policy is a Rego policy that validates a configuration and injects defaults,
// the same mechanism OPA uses for its own configuration. Custom plugins can use
// one from their factory's Validate function. Safe for concurrent use.
//
// The policy module is evaluated with the input
//
//	{"config": <the raw configuration>}
//
// and its package is expected to define these rules:
//
//	processed - the configuration with defaults injected (required)
//	errors    - a set of fatal error messages, returned as a single error
//	warnings  - a set of non-fatal messages, returned to the caller
//
// The module may import data.opa.config.util for the helpers OPA's own
// validation policies share.
type Policy struct {
	policy *configpolicy.Policy
}

// NewPolicy returns a Policy for the given Rego module (v1 syntax). name is
// used as the module's filename and in error messages. The result document is
// read from the module's package, so no separate query is needed. The module
// is compiled on first use.
func NewPolicy(name, module string) (*Policy, error) {
	parsed, err := ast.ParseModuleWithOpts(name, module, ast.ParserOptions{RegoVersion: ast.RegoV1})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	query := fmt.Sprintf("%v = x", parsed.Package.Path)
	return &Policy{policy: configpolicy.New(name, module, query)}, nil
}

// EvalPolicyInto evaluates p against the raw configuration (absent, empty and
// null are treated as an empty object) and decodes the resulting processed
// configuration into out. It returns any warnings the policy produced; errors
// the policy produced are returned as a single error. Decoding into out also
// rejects options of the wrong type that the policy does not check itself.
func EvalPolicyInto[T any](ctx context.Context, p *Policy, raw []byte, out *T) ([]string, error) {
	return configpolicy.EvalConfigInto(ctx, p.policy, raw, out)
}
