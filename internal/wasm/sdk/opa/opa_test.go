// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package opa_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	wasm_util "github.com/open-policy-agent/opa/internal/wasm/util"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
)

// control dumping in this file
const dump = false

func TestOPA(t *testing.T) {
	type Eval struct {
		NewPolicy string
		NewData   string
		Input     string
		Result    string
	}

	largeInput := `"` + strings.Repeat("a", 2*wasm_util.PageSize) + `"`

	tests := []struct {
		Description string
		Policy      string
		Query       string
		Data        string
		Evals       []Eval
		WantErr     string   // "" (or unset) means no error expected
		Memory      []uint32 // min, max; in pages
	}{
		{
			Description: "No input, no data, static policy",
			Policy:      `a = true`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				{Result: `{{"x": true}}`},
				{Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only input changing",
			Policy:      `a = input`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				{Input: "false", Result: `{{"x": false}}`},
				{Input: "true", Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false}`,
			Evals: []Eval{
				{Result: `{{"x": false}}`},
				{NewData: `{"q": true}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only policy changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false, "r": true}`,
			Evals: []Eval{
				{Result: `{{"x": false}}`},
				{NewPolicy: `a = data.r`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Policy and data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": 0, "r": 1}`,
			Evals: []Eval{
				{Result: `{{"x": 0}}`},
				{NewPolicy: `a = data.r`, NewData: `{"q": 2, "r": 3}`, Result: `{{"x": 3}}`},
			},
		},
		{
			Description: "Builtins",
			Policy:      `a = count(data.q) + sum(data.q)`, // builtin not implemented in wasm.
			Query:       "data.p.a = x",
			Evals: []Eval{
				{NewData: `{"q": []}`, Result: `{{"x": 0}}`},
				{NewData: `{"q": [1, 2]}`, Result: `{{"x": 5}}`},
			},
		},
		{
			Description: "Undefined decision",
			Policy:      `a = true`,
			Query:       "data.p.b = x",
			Evals: []Eval{
				{Result: `set()`},
			},
		},
		{
			Description: "Runtime error/object insert conflict",
			Policy:      `a = { "a": y | y := [1, 2][_] }`,
			Query:       "data.p.a.a = x",
			Evals:       []Eval{{}},
			WantErr:     "internal_error: module.rego:2:5: object insert conflict",
		},
		{
			Description: "Runtime error/var assignment conflict",
			Policy: `a = "b" { input > 1 }
a = "c" { input > 2 }`,
			Query: "data.p.a = x",
			Evals: []Eval{
				{Input: "3"},
			},
			WantErr: "internal_error: module.rego:3:1: var assignment conflict",
		},
		{
			Description: "Runtime error/else conflict-1",
			Query:       `data.p.q`,
			Policy: `
				q {
					false
				}
				else = true {
					true
				}
				q = false`,
			Evals:   []Eval{{}},
			WantErr: "internal_error: module.rego:9:5: var assignment conflict",
		},
		{
			Description: "Runtime error/else conflict-2",
			Query:       `data.p.q`,
			Policy: `
				q {
					false
				}
				else = false {
					true
				}
				q {
					false
				}
				else = true {
					true
				}`,
			Evals:   []Eval{{}},
			WantErr: "internal_error: module.rego:12:5: var assignment conflict",
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
				{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
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
				{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.match with pattern from input",
			Query:       `x = regex.match(input.re, "foo")`,
			Evals: []Eval{
				{Input: `{"re": "^foo$"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.find_all_string_submatch_n with pattern from input",
			Query:       `x = regex.find_all_string_submatch_n(input.re, "-axxxbyc-", -1)`,
			Evals: []Eval{
				{Input: `{"re": "a(x*)b(y|z)c"}`, Result: `{{"x":[["axxxbyc","xxx","y"]]}}`},
			},
		},
		{
			Description: "simplified",
			Query:       `x := "q"; y := data.p[x]`,
			Policy: `p = 1
			q = 2`,
			Evals: []Eval{
				{Result: `{{"y": 2, "x": "q"}}`},
			},
		},
		{
			Description: "mpd init problem (#3110)",
			Query:       `data.p.main = x`,
			Policy:      `main { numbers.range(1, 2)[_] == 2 }`,
			Evals: []Eval{
				{Result: `{{"x": true}}`},
				{Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Virtual extent, undefined data",
			Policy: `package a.b
			c = 3`,
			Query: `data == {"a": {"b": {"c": 3 }}}`,
			Evals: []Eval{
				{Result: `{{}}`},
			},
		},
		{
			Description: "input exceeds available memory, host fails to grow it",
			Policy: `package a.b
			p = true`,
			Query:  `data.a.b.p`,
			Memory: []uint32{2, 3},
			Evals: []Eval{
				{Input: largeInput},
			},
			WantErr: "input: failed to grow memory by `2` (max pages 3)",
		},
		{
			Description: "input exceeds available memory, parsing it hits maximum",
			Policy: `package a.b
			p = true`,
			Query:  `data.a.b.p`,
			Memory: []uint32{2, 4},
			Evals: []Eval{
				{Input: largeInput},
			},
			WantErr: "internal_error: opa_malloc: failed",
		},
		{
			Description: "input exceeds available memory, grows successfully",
			Policy: `package a.b
		p = true`,
			Query:  `data.a.b.p = x`,
			Memory: []uint32{2, 8},
			Evals: []Eval{
				{Input: largeInput, Result: `{{"x":true}}`},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Description, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			policy := compileRegoToWasm(test.Policy, test.Query, dump)
			data := []byte(test.Data)
			if len(data) == 0 {
				data = nil
			}
			o := opa.New().
				WithPolicyBytes(policy).
				WithDataBytes(data).
				WithPoolSize(1) // Minimal pool size to test pooling.
			if len(test.Memory) == 2 {
				o.WithMemoryLimits(test.Memory[0]*wasm_util.PageSize, test.Memory[1]*wasm_util.PageSize)
			}

			instance, err := o.Init()
			if err != nil {
				t.Fatal(err)
			}

			// Execute each requested policy evaluation, with their inputs and updating data if requested.

			for _, eval := range test.Evals {
				switch {
				case eval.NewPolicy != "" && eval.NewData != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query, dump)
					data := parseJSON(eval.NewData)
					if err := instance.SetPolicyData(ctx, policy, data); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewPolicy != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query, dump)
					if err := instance.SetPolicy(ctx, policy); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewData != "":
					data := parseJSON(eval.NewData)
					if err := instance.SetData(ctx, *data); err != nil {
						t.Errorf(err.Error())
					}
				}

				r, err := instance.Eval(ctx, opa.EvalOpts{Input: parseJSON(eval.Input)})
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

	instance, err := opa.New().
		WithPolicyBytes(compiler.Bundle().WasmModules[0].Raw).
		WithPoolSize(1).
		Init()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

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

// compileRegoToWasm is shared with the benchmarking functions in opa_bench_test.go;
// those function use helpers shared with topdown_bench_test.go, and they all use
// `package test` -- whereas the callers in this file don't provide the package at
// all and assume it'll be `p`.
func compileRegoToWasm(module string, query string, dump bool) []byte {
	if !strings.HasPrefix(module, "package") {
		module = fmt.Sprintf("package p\n%s", module)
	}
	opts := []func(*rego.Rego){
		rego.Query(query),
		rego.Module("module.rego", module),
	}
	if dump {
		opts = append(opts, rego.Dump(os.Stderr))
	}
	cr, err := rego.New(opts...).Compile(context.Background(), rego.CompilePartial(false))
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
