// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"context"
	_ "embed"
	"fmt"

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
		"config":  raw,
		"runtime": runtimeInput(id),
		"specs":   registeredConfigSpecs(),
	}
	return validationPolicy.Eval(ctx, input)
}

func runtimeInput(id string) map[string]any {
	return map[string]any{
		"id":      id,
		"version": version.Version,
	}
}

func compileValidationPolicy() (*ast.Compiler, error) {
	return validationPolicy.Compiler()
}

// customValidationQuery binds the result document (errors/warnings) of the
// user-supplied validation policies to x.
const customValidationQuery = "data.system.config = x"

var customValidationPackage = ast.MustParseRef("data.system.config")

// ValidationPolicy holds user-supplied Rego modules that validate the
// configuration beyond OPA's built-in checks, e.g. to enforce organizational
// requirements. At least one module must declare package system.config, whose
// errors and warnings sets are reported like the built-in ones: errors are
// fatal, warnings are attached to the parsed Config. The modules see the
// configuration with OPA's defaults injected as input.config, and may import
// data.opa.config.util for the helpers the built-in policies use.
type ValidationPolicy struct {
	policy *configpolicy.Policy
}

// NewValidationPolicy compiles modules, the Rego source of each keyed by its
// filename, into a ValidationPolicy.
func NewValidationPolicy(modules map[string]string) (*ValidationPolicy, error) {
	p := configpolicy.NewWithModules("config validation policy", modules, customValidationQuery)
	compiler, err := p.Compiler()
	if err != nil {
		return nil, err
	}
	for name := range modules {
		if compiler.Modules[name].Package.Path.Equal(customValidationPackage) {
			return &ValidationPolicy{policy: p}, nil
		}
	}
	return nil, fmt.Errorf("config validation policy: no module declares package %v", customValidationPackage[1:])
}

// check evaluates the policy against the processed configuration.
func (vp *ValidationPolicy) check(ctx context.Context, processed map[string]any, id string) ([]string, error) {
	return vp.policy.Check(ctx, map[string]any{
		"config":  processed,
		"runtime": runtimeInput(id),
	})
}
