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
