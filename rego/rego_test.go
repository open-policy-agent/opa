// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func TestRegoEvalExpressionValue(t *testing.T) {

	module := `package test

	arr = [1,false,true]
	f(x) = x
	g(x, y) = x + y
	h(x) = false`

	tests := []struct {
		query    string
		expected string
	}{
		{
			query:    "1",
			expected: "[[1]]",
		},
		{
			query:    "1+2",
			expected: "[[3]]",
		},
		{
			query:    "1+(2*3)",
			expected: "[[7]]",
		},
		{
			query:    "data.test.arr[0]",
			expected: "[[1]]",
		},
		{
			query:    "data.test.arr[1]",
			expected: "[[false]]",
		},
		{
			query:    "data.test.f(1)",
			expected: "[[1]]",
		},
		{
			query:    "data.test.f(1,x)",
			expected: "[[true]]",
		},
		{
			query:    "data.test.g(1,2)",
			expected: "[[3]]",
		},
		{
			query:    "data.test.g(1,2,x)",
			expected: "[[true]]",
		},
		{
			query:    "false",
			expected: "[[false]]",
		},
		{
			query:    "1 == 2",
			expected: "[[false]]",
		},
		{
			query:    "data.test.h(1)",
			expected: "[[false]]",
		},
		{
			query:    "data.test.g(1,2) != 3",
			expected: "[[false]]",
		},
		{
			query:    "data.test.arr[i]",
			expected: "[[1], [true]]",
		},
		{
			query:    "[x | data.test.arr[_] = x]",
			expected: "[[[1, false, true]]]",
		},
		{
			query:    "a = 1; b = 2; a > b",
			expected: `[]`,
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			r := New(
				Query(tc.query),
				Module("", module),
			)

			rs, err := r.Eval(ctx)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			result := []interface{}{}

			for i := range rs {
				values := []interface{}{}
				for j := range rs[i].Expressions {
					values = append(values, rs[i].Expressions[j].Value)
				}
				result = append(result, values)
			}

			if !reflect.DeepEqual(result, util.MustUnmarshalJSON([]byte(tc.expected))) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.expected, result)
			}
		})
	}
}

func TestRegoInputs(t *testing.T) {
	tests := map[string]struct {
		input    interface{}
		expected string
	}{
		"map":  {map[string]bool{"foo": true}, `[[{"foo": true}]]`},
		"int":  {1, `[[1]]`},
		"bool": {false, `[[false]]`},
		"struct": {struct {
			Foo string `json:"baz"`
		}{"bar"}, `[[{"baz":"bar"}]]`},
		"pointer to struct": {&struct {
			Foo string `json:"baz"`
		}{"bar"}, `[[{"baz":"bar"}]]`},
		"pointer to pointer to struct": {
			func() interface{} {
				a := &struct {
					Foo string `json:"baz"`
				}{"bar"}
				return &a
			}(), `[[{"baz":"bar"}]]`},
		"slice":              {[]string{"a", "b"}, `[[["a", "b"]]]`},
		"nil":                {nil, `[[null]]`},
		"slice of interface": {[]interface{}{"a", 2, true}, `[[["a", 2, true]]]`},
	}
	ctx := context.Background()

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			r := New(
				Query("input"),
				Input(tc.input),
			)
			rs, err := r.Eval(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			result := []interface{}{}
			for i := range rs {
				values := []interface{}{}
				for j := range rs[i].Expressions {
					values = append(values, rs[i].Expressions[j].Value)
				}
				result = append(result, values)
			}

			if !reflect.DeepEqual(result, util.MustUnmarshalJSON([]byte(tc.expected))) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.expected, result)
			}
		})
	}
}

func TestRegoRewrittenVarsCapture(t *testing.T) {

	ctx := context.Background()

	r := New(
		Query("a := 1; a != 0; a"),
	)

	rs, err := r.Eval(ctx)
	if err != nil || len(rs) != 1 {
		t.Fatalf("Unexpected result: %v (err: %v)", rs, err)
	}

	if !reflect.DeepEqual(rs[0].Bindings["a"], json.Number("1")) {
		t.Fatal("Expected a to be 1 but got:", rs[0].Bindings["a"])
	}

}

func TestRegoCancellation(t *testing.T) {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	topdown.RegisterFunctionalBuiltin1("test.sleep", func(a ast.Value) (ast.Value, error) {
		d, _ := time.ParseDuration(string(a.(ast.String)))
		time.Sleep(d)
		return ast.Null{}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	r := New(Query(`test.sleep("1s")`))
	rs, err := r.Eval(ctx)
	cancel()

	if err == nil {
		t.Fatalf("Expected cancellation error but got: %v", rs)
	} else if topdownErr, ok := err.(*topdown.Error); !ok || topdownErr.Code != topdown.CancelErr {
		t.Fatalf("Got unexpected error: %v", err)
	}
}

func TestRegoMetrics(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m))
	ctx := context.Background()
	_, err := r.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exp := []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_eval_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_module_parse_ns",
		"timer_rego_module_compile_ns",
	}

	all := m.All()

	for _, name := range exp {
		if _, ok := all[name]; !ok {
			t.Errorf("expected to find %v but did not", name)
		}
	}
}

func TestRegoCatchPathConflicts(t *testing.T) {
	r := New(
		Query("data"),
		Module("test.rego", "package x\np=1"),
		Store(inmem.NewFromObject(map[string]interface{}{
			"x": map[string]interface{}{"p": 1},
		})),
	)

	ctx := context.Background()
	_, err := r.Eval(ctx)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPartialRewriteEquals(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	ctx := context.Background()
	pq, err := r.Partial(ctx)

	if err != nil {
		t.Fatalf("unexpected error from Rego.Partial(): %s", err.Error())
	}

	// Expect to not have any "support" in the resulting queries
	if len(pq.Support) > 0 {
		t.Errorf("expected to not have any Support in PartialQueries: %+v", pq)
	}

	expectedQuery := "input.x = 1"
	if len(pq.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(pq.Queries), pq)
	}
	if pq.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'", expectedQuery, pq.Queries[0].String())
	}
}
