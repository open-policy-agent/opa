// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ir"
)

type testPlugin struct {
	state string
}

func (*testPlugin) Start(context.Context) error {
	return nil
}

func (*testPlugin) Stop(context.Context) {
}

func (*testPlugin) Reconfigure(context.Context, interface{}) {
}

func (*testPlugin) IsTarget(t string) bool {
	return t == "foo"
}

func (*testPlugin) PrepareForEval(context.Context, *ir.Policy, ...PrepareOption) (TargetPluginEval, error) {
	return &testPlugin{state: "newstate"}, nil
}

func (t *testPlugin) Eval(_ context.Context, _ *EvalContext, rt ast.Value) (ast.Value, error) {
	if rt != nil {
		return ast.NewSet(ast.NewTerm(ast.NewObject(
			[2]*ast.Term{ast.StringTerm("^term1"), ast.ObjectTerm([2]*ast.Term{ast.StringTerm(t.state), ast.NewTerm(rt)})},
		))), nil
	}
	return ast.NewSet(ast.NewTerm(ast.NewObject(
		[2]*ast.Term{ast.StringTerm("^term1"), ast.StringTerm(t.state)},
	))), nil
}

func TestTargetViaPlugin(t *testing.T) {
	tp := testPlugin{}
	RegisterPlugin("rego.target.foo", &tp)
	defer resetPlugins()
	r := New(
		Query("input"),
		Input("original-input"),
		Target("foo"),
		Runtime(ast.StringTerm("runtime")),
	)
	assertEval(t, r, `[[{"newstate": "runtime"}]]`)
}

type defaultPlugin struct {
	testPlugin
}

func (*defaultPlugin) IsTarget(t string) bool { return t == "" || t == "foo" }

func TestTargetViaDefaultPlugin(t *testing.T) {
	t.Run("no target", func(t *testing.T) {
		tp := defaultPlugin{testPlugin{}}
		RegisterPlugin("rego.target.foo", &tp)
		defer resetPlugins()
		r := New(
			Query("input"),
			Input("original-input"),
		)
		assertEval(t, r, `[["newstate"]]`)
	})

	t.Run("other target NOT overridden", func(t *testing.T) {
		tp := defaultPlugin{testPlugin{}}
		RegisterPlugin("rego.target.foo", &tp)
		defer resetPlugins()
		r := New(
			Query("input"),
			Input("original-input"),
			Target("rego"),
		)
		assertEval(t, r, `[["original-input"]]`)
	})
}

func resetPlugins() {
	targetPlugins = map[string]TargetPlugin{}
}
