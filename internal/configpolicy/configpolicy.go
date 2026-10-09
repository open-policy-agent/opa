// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package configpolicy validates configuration with an embedded Rego policy.
//
// A policy produces a document with these (optional) entrypoints:
//
//	processed - the input config with defaults injected (required)
//	errors    - a set/array of fatal error strings (joined into one error)
//	warnings  - a set/array of non-fatal warning strings
//
// Every policy is compiled together with util.rego, so a policy can import
// data.opa.config.util for the helpers shared across the validation policies.
package configpolicy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/util"
)

// The shared helpers are compiled into every policy, so a validation policy can
// import data.opa.config.util rather than repeat them.
const utilModuleName = "opa/config/util.rego"

//go:embed util.rego
var utilModule string

// Policy is an embedded validation policy, compiled once on first use and
// evaluated repeatedly. Safe for concurrent use.
type Policy struct {
	name    string
	modules map[string]string
	query   ast.Body

	compileOnce sync.Once
	compiler    *ast.Compiler
	compileErr  error
}

// New returns a Policy for the given module: name is the module filename (and a
// prefix on infrastructure errors), source is its Rego, and query binds the
// result document to x (e.g. "data.opa.config = x").
func New(name, source, query string) *Policy {
	return NewWithModules(name, map[string]string{name: source}, query)
}

// NewWithModules is like New, but compiles several modules (keyed by filename)
// together; name only prefixes infrastructure errors.
func NewWithModules(name string, modules map[string]string, query string) *Policy {
	return &Policy{
		name:    name,
		modules: modules,
		query:   ast.MustParseBody(query),
	}
}

// Eval evaluates the policy against input (any value ast.InterfaceToValue
// accepts), returning the processed config and warnings; a non-empty set of
// policy errors is returned as a single error.
func (p *Policy) Eval(ctx context.Context, input any) (map[string]any, []string, error) {
	doc, err := p.eval(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	warnings, err := reported(doc)
	if err != nil {
		return nil, nil, err
	}

	processed, ok := doc["processed"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("%s: policy did not produce a processed configuration", p.name)
	}

	return processed, warnings, nil
}

// Check is like Eval for a policy that only reports errors and warnings: it
// does not require (or return) a processed configuration. An undefined result
// document means nothing was reported. Unlike Eval, it is meant for policies
// OPA doesn't own, so errors and warnings that aren't sets of strings are
// rejected rather than ignored: a malformed errors rule must not pass validation.
func (p *Policy) Check(ctx context.Context, input any) ([]string, error) {
	doc, err := p.eval(ctx, input)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"errors", "warnings"} {
		if err := p.checkStrings(doc, key); err != nil {
			return nil, err
		}
	}
	return reported(doc)
}

// checkStrings returns an error unless doc[key] is absent or a set (or array)
// of strings.
func (p *Policy) checkStrings(doc map[string]any, key string) error {
	v, ok := doc[key]
	if !ok {
		return nil
	}
	items, ok := v.([]any)
	if !ok || slices.ContainsFunc(items, func(item any) bool { _, ok := item.(string); return !ok }) {
		return fmt.Errorf("%s: %s must be a set of strings", p.name, key)
	}
	return nil
}

// reported returns the result document's sorted warnings, or its errors sorted
// and joined into a single error.
func reported(doc map[string]any) ([]string, error) {
	if errs := StringSet(doc["errors"]); len(errs) > 0 {
		slices.Sort(errs)
		return nil, errors.New(strings.Join(errs, "; "))
	}

	warnings := StringSet(doc["warnings"])
	slices.Sort(warnings)

	return warnings, nil
}

// eval runs the query and returns the result document bound to x, or an empty
// document when the query is undefined.
func (p *Policy) eval(ctx context.Context, input any) (map[string]any, error) {
	compiler, err := p.Compiler()
	if err != nil {
		return nil, err
	}

	inputValue, err := ast.InterfaceToValue(input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}

	store := inmem.New()
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}
	defer store.Abort(ctx, txn)

	qrs, err := topdown.NewQuery(p.query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(ast.NewTerm(inputValue)).
		Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}
	if len(qrs) == 0 {
		return map[string]any{}, nil
	}

	result, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}
	doc, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: unexpected result type %T", p.name, result)
	}
	return doc, nil
}

// EvalConfigInto decodes raw config bytes (absent/empty/null → empty object),
// evaluates the policy with input {"config": <raw>} to inject defaults, and
// decodes the processed config into out, returning any warnings. Field types the
// policy does not check are enforced by the typed unmarshal into out, so a
// mistyped option surfaces here rather than in the policy.
func EvalConfigInto[T any](ctx context.Context, p *Policy, raw []byte, out *T) ([]string, error) {
	rawConfig, err := unmarshalRawConfig(raw)
	if err != nil {
		return nil, err
	}
	if _, ok := rawConfig.(map[string]any); !ok {
		// Caught here rather than in the policy, which would fail to produce a
		// processed config and report the far less obvious infrastructure error.
		return nil, fmt.Errorf("%s: config must be an object", p.name)
	}

	processed, warnings, err := p.Eval(ctx, map[string]any{"config": rawConfig})
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(processed)
	if err != nil {
		return nil, err
	}
	if err := util.Unmarshal(bs, out); err != nil {
		return nil, err
	}
	return warnings, nil
}

// unmarshalRawConfig decodes raw config bytes, treating absent or null config as
// an empty object.
func unmarshalRawConfig(raw []byte) (any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var rawConfig any
	if err := util.Unmarshal(raw, &rawConfig); err != nil {
		return nil, err
	}
	if rawConfig == nil {
		return map[string]any{}, nil
	}
	return rawConfig, nil
}

// Compiler compiles the policy once and returns the reusable result. Exported so
// callers can run ad-hoc queries against the policy (e.g. drift-guard tests).
func (p *Policy) Compiler() (*ast.Compiler, error) {
	p.compileOnce.Do(func() {
		popts := ast.ParserOptions{RegoVersion: ast.RegoV1}
		helpers, err := ast.ParseModuleWithOpts(utilModuleName, utilModule, popts)
		if err != nil {
			p.compileErr = fmt.Errorf("%s: %w", utilModuleName, err)
			return
		}
		modules := map[string]*ast.Module{utilModuleName: helpers}
		for name, source := range p.modules {
			module, err := ast.ParseModuleWithOpts(name, source, popts)
			if err != nil {
				p.compileErr = fmt.Errorf("%s: %w", p.name, err)
				return
			}
			modules[name] = module
		}
		compiler := ast.NewCompiler()
		if compiler.Compile(modules); compiler.Failed() {
			p.compileErr = fmt.Errorf("%s: %w", p.name, compiler.Errors)
			return
		}
		p.compiler = compiler
	})
	return p.compiler, p.compileErr
}

// StringSet converts a Rego set/array result to a []string, ignoring non-string
// members and returning nil when empty.
func StringSet(v any) []string {
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
