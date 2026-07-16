// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"context"
	_ "embed"

	"github.com/open-policy-agent/opa/internal/configpolicy"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/version"
)

// coreValidationModule injects OPA's config defaults and reports unrecognized
// options.
//
//go:embed validate.rego
var coreValidationModule string

const coreValidationPolicyName = "opa/config/validate.rego"

// validationQuery binds the policy's result document (processed/warnings/errors) to x.
const validationQuery = "data.opa.config = x"

var validationPolicy = configpolicy.New(coreValidationPolicyName, coreValidationModule, validationQuery)

// evaluateConfigPolicy runs the policy against raw, returning the config with
// defaults injected and any warnings. Specs registered via RegisterConfigSpec
// are supplied so plugin-owned options are recognized.
func evaluateConfigPolicy(ctx context.Context, raw any, id string) (map[string]any, []string, error) {
	input := map[string]any{
		"config": raw,
		"runtime": map[string]any{
			"id":      id,
			"version": version.Version,
		},
		"specs": registeredConfigSpecs(),
	}
	return validationPolicy.Eval(ctx, input)
}

func compileValidationPolicy() (*ast.Compiler, error) {
	return validationPolicy.Compiler()
}
