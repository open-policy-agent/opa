// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/version"
)

// coreValidationModule is the embedded policy that injects OPA's config
// defaults and reports unrecognized options.
//
//go:embed validate.rego
var coreValidationModule string

const coreValidationPolicyName = "opa/config/validate.rego"

// validationQuery yields a document with `processed`, `warnings` and `errors`.
// The result is bound to x. (The topdown package is used directly rather than
// the rego package to avoid a config -> rego -> bundle import cycle.)
const validationQuery = "data.opa.config = x"

// validationQueryBody is parsed once; the query never changes.
var validationQueryBody = ast.MustParseBody(validationQuery)

// The policy is static, so it is compiled once and reused across all calls.
var (
	compileOnce    sync.Once
	compiledPolicy *ast.Compiler
	compileErr     error
)

// evaluateConfigPolicy runs the validation policy against raw. It returns the
// config with defaults injected and any warnings; a non-empty set of policy
// `errors` is returned as a single error.
func evaluateConfigPolicy(ctx context.Context, raw any, id string) (map[string]any, []string, error) {
	compiler, err := compileValidationPolicy()
	if err != nil {
		return nil, nil, err
	}

	inputValue, err := ast.InterfaceToValue(map[string]any{
		"config": raw,
		"runtime": map[string]any{
			"id":      id,
			"version": version.Version,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}

	store := inmem.New()
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}
	defer store.Abort(ctx, txn)

	qrs, err := topdown.NewQuery(validationQueryBody).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(ast.NewTerm(inputValue)).
		Run(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}
	if len(qrs) != 1 {
		return nil, nil, errors.New("config validation: policy produced no result")
	}

	result, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}
	doc, ok := result.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("config validation: unexpected result type %T", result)
	}

	if errs := stringSet(doc["errors"]); len(errs) > 0 {
		slices.Sort(errs)
		return nil, nil, errors.New(strings.Join(errs, "; "))
	}

	processed, ok := doc["processed"].(map[string]any)
	if !ok {
		return nil, nil, errors.New("config validation: policy did not produce a processed configuration")
	}

	warnings := stringSet(doc["warnings"])
	slices.Sort(warnings)

	return processed, warnings, nil
}

// compileValidationPolicy parses and compiles the embedded core policy once.
func compileValidationPolicy() (*ast.Compiler, error) {
	compileOnce.Do(func() {
		popts := ast.ParserOptions{RegoVersion: ast.RegoV1}
		core, err := ast.ParseModuleWithOpts(coreValidationPolicyName, coreValidationModule, popts)
		if err != nil {
			compileErr = fmt.Errorf("config validation: %w", err)
			return
		}
		compiler := ast.NewCompiler()
		if compiler.Compile(map[string]*ast.Module{coreValidationPolicyName: core}); compiler.Failed() {
			compileErr = fmt.Errorf("config validation: %w", compiler.Errors)
			return
		}
		compiledPolicy = compiler
	})
	return compiledPolicy, compileErr
}

// stringSet turns a Rego set/array result into a []string, ignoring non-string
// members and returning nil when empty.
func stringSet(v any) []string {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
