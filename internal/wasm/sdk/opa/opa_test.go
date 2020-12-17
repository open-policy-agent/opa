// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package opa_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
)

func TestOPA(t *testing.T) {
	type Eval struct {
		NewPolicy string
		NewData   string
		Input     string
		Result    string
	}

	tests := []struct {
		Description string
		Policy      string
		Query       string
		Data        string
		Evals       []Eval
		WantErr     string // "" means no error expected
	}{
		{
			Description: "No input, no data, static policy",
			Policy:      `a = true`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				Eval{Result: `{{"x": true}}`},
				Eval{Result: `{{"x": true}}`},
			},
			WantErr: "",
		},
		{
			Description: "Only input changing",
			Policy:      `a = input`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				Eval{Input: "false", Result: `{{"x": false}}`},
				Eval{Input: "true", Result: `{{"x": true}}`},
			},
			WantErr: "",
		},
		{
			Description: "Only data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false}`,
			Evals: []Eval{
				Eval{Result: `{{"x": false}}`},
				Eval{NewData: `{"q": true}`, Result: `{{"x": true}}`},
			},
			WantErr: "",
		},
		{
			Description: "Only policy changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false, "r": true}`,
			Evals: []Eval{
				Eval{Result: `{{"x": false}}`},
				Eval{NewPolicy: `a = data.r`, Result: `{{"x": true}}`},
			},
			WantErr: "",
		},
		{
			Description: "Policy and data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": 0, "r": 1}`,
			Evals: []Eval{
				Eval{Result: `{{"x": 0}}`},
				Eval{NewPolicy: `a = data.r`, NewData: `{"q": 2, "r": 3}`, Result: `{{"x": 3}}`},
			},
			WantErr: "",
		},
		{
			Description: "Builtins",
			Policy:      `a = count(data.q) + sum(data.q)`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				Eval{NewData: `{"q": []}`, Result: `{{"x": 0}}`},
				Eval{NewData: `{"q": [1, 2]}`, Result: `{{"x": 5}}`},
			},
			WantErr: "",
		},
		{
			Description: "Undefined decision",
			Policy:      `a = true`,
			Query:       "data.p.b = x",
			Evals: []Eval{
				Eval{Result: `set()`},
			},
			WantErr: "",
		},
		{
			Description: "Runtime error/object insert conflict",
			Policy:      `a = { "a": y | y := [1, 2][_] }`,
			Query:       "data.p.a.a = x",
			Evals:       []Eval{{}},
			WantErr:     "module.rego:2:5: object insert conflict: internal error",
		},
		{
			Description: "Runtime error/var assignment conflict",
			Policy: `a = "b" { input > 1 }
a = "c" { input > 2 }`,
			Query: "data.p.a = x",
			Evals: []Eval{
				{Input: "3"},
			},
			WantErr: "module.rego:3:1: var assignment conflict: internal error",
		},
		// NOTE(sr): The next two test cases were used to replicate issue
		// https://github.com/open-policy-agent/opa/issues/2962 -- their raison d'être
		// is thus questionable, but it might be good to keep them around a bit.
		{
			Description: "Only input changing, regex.match",
			Policy: `
			default hello = false
			hello {
				regex.match("^world$", input.message)
			}`,
			Query: "data.p.hello = x",
			Evals: []Eval{
				Eval{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				Eval{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only input changing, glob.match",
			Policy: `
			default hello = false
			hello {
				glob.match("world", [":"], input.message)
			}`,
			Query: "data.p.hello = x",
			Evals: []Eval{
				Eval{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				Eval{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.match with pattern from input",
			Query:       `x = regex.match(input.re, "foo")`,
			Evals: []Eval{
				Eval{Input: `{"re": "^foo$"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.find_all_string_submatch_n with pattern from input",
			Query:       `x = regex.find_all_string_submatch_n(input.re, "-axxxbyc-", -1)`,
			Evals: []Eval{
				Eval{Input: `{"re": "a(x*)b(y|z)c"}`, Result: `{{"x":[["axxxbyc","xxx","y"]]}}`},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Description, func(t *testing.T) {
			policy := compileRegoToWasm(test.Policy, test.Query)
			data := []byte(test.Data)
			if len(data) == 0 {
				data = nil
			}
			instance, err := opa.New().
				WithPolicyBytes(policy).
				WithDataBytes(data).
				WithMemoryLimits(131070, 0).
				WithPoolSize(1). // Minimal pool size to test pooling.
				Init()
			if err != nil {
				t.Fatal(err)
			}

			// Execute each requested policy evaluation, with their inputs and updating data if requested.

			for _, eval := range test.Evals {
				switch {
				case eval.NewPolicy != "" && eval.NewData != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query)
					data := parseJSON(eval.NewData)
					if err := instance.SetPolicyData(policy, data); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewPolicy != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query)
					if err := instance.SetPolicy(policy); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewData != "":
					data := parseJSON(eval.NewData)
					if err := instance.SetData(*data); err != nil {
						t.Errorf(err.Error())
					}
				}

				r, err := instance.Eval(context.Background(), opa.EvalOpts{Input: parseJSON(eval.Input)})
				if err != nil {
					if test.WantErr == "" { // no error desired
						t.Fatal(err.Error())
					}
					if expected, actual := test.WantErr, err.Error(); expected != actual {
						t.Fatalf("expected error %q, got %q", expected, actual)
					}
					return
				}
				if test.WantErr != "" {
					t.Fatalf("expected error %q, got nil", test.WantErr)
				}

				expected := ast.MustParseTerm(eval.Result)
				if !ast.MustParseTerm(string(r.Result)).Equal(expected) {
					t.Errorf("\nExpected: %v\nGot: %v\n", expected, string(r.Result))
				}
			}

			instance.Close()
		})
	}
}

func TestNamedEntrypoint(t *testing.T) {
	module := `package test
	
	a = 7
	b = a
	`

	ctx := context.Background()

	compiler := compile.New().
		WithTarget(compile.TargetWasm).
		WithEntrypoints("test/a", "test/b").
		WithBundle(&bundle.Bundle{
			Modules: []bundle.ModuleFile{
				{
					Path:   "policy.rego",
					URL:    "policy.rego",
					Raw:    []byte(module),
					Parsed: ast.MustParseModule(module),
				},
			},
		})

	err := compiler.Build(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	instance, _ := opa.New().
		WithPolicyBytes(compiler.Bundle().WasmModules[0].Raw).
		WithMemoryLimits(131070, 2*131070). // TODO: For some reason unlimited memory slows down the eval_ctx_new().
		WithPoolSize(1).
		Init()

	eps, err := instance.Entrypoints(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(eps) != 2 {
		t.Fatalf("Expected 2 entrypoints, got: %+v", eps)
	}

	a, err := instance.Eval(ctx, opa.EvalOpts{Entrypoint: eps["test/a"]})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	exp := ast.MustParseTerm(`{{"result":7}}`)
	actual := ast.MustParseTerm(string(a.Result))
	if !actual.Equal(exp) {
		t.Fatalf("Expected result for 'test/a' to be %s, got: %s", exp, actual)
	}

	b, err := instance.Eval(ctx, opa.EvalOpts{Entrypoint: eps["test/b"]})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actual = ast.MustParseTerm(string(b.Result))
	if !actual.Equal(exp) {
		t.Fatalf("Expected result for 'test/b' to be %s, got: %s", exp, actual)
	}
}

func BenchmarkWasmRego(b *testing.B) {
	policy := compileRegoToWasm("a = true", "data.p.a = x")
	instance, _ := opa.New().
		WithPolicyBytes(policy).
		WithMemoryLimits(131070, 2*131070). // TODO: For some reason unlimited memory slows down the eval_ctx_new().
		WithPoolSize(1).
		Init()

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	var input interface{} = make(map[string]interface{})

	for i := 0; i < b.N; i++ {
		if _, err := instance.Eval(ctx, opa.EvalOpts{Input: &input}); err != nil {
			panic(err)
		}
	}
}

func BenchmarkGoRego(b *testing.B) {
	pq := compileRego(`package p

a = true`, "data.p.a = x")

	b.ReportAllocs()
	b.ResetTimer()

	input := make(map[string]interface{})

	for i := 0; i < b.N; i++ {
		if _, err := pq.Eval(context.Background(), rego.EvalInput(input)); err != nil {
			panic(err)
		}
	}
}

func compileRegoToWasm(policy string, query string) []byte {
	module := fmt.Sprintf("package p\n%s", policy)
	cr, err := rego.New(
		rego.Query(query),
		rego.Module("module.rego", module),
	).Compile(context.Background(), rego.CompilePartial(false))
	if err != nil {
		panic(err)
	}

	return cr.Bytes
}

func compileRego(module string, query string) rego.PreparedEvalQuery {
	rego := rego.New(
		rego.Query(query),
		rego.Module("module.rego", module),
	)
	pq, err := rego.PrepareForEval(context.Background())
	if err != nil {
		panic(err)
	}

	return pq
}

func parseJSON(s string) *interface{} {
	if s == "" {
		return nil
	}

	v := util.MustUnmarshalJSON([]byte(s))
	return &v
}
