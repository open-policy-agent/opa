// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestBuiltinSprintf(t *testing.T) {
	tests := []struct {
		note   string
		format string
		args   *ast.Array
		exp    string
	}{
		{
			note:   "integer",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("42")),
			exp:    "42",
		},
		{
			note:   "integer, multiple args",
			format: "%d-%d",
			args:   ast.NewArray(ast.NumberTerm("42"), ast.NumberTerm("-1")),
			exp:    "42--1",
		},
		{
			note:   "integer too large for int64",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("1208925819614629174706175")),
			exp:    "1208925819614629174706175",
		},
		{
			note:   "float",
			format: "%f",
			args:   ast.NewArray(ast.NumberTerm("0.1")),
			exp:    "0.100000",
		},
		{
			// https://github.com/open-policy-agent/opa/issues/9187
			note:   "float with zero fraction",
			format: "float: %f, %3.1f, %.3f, %f, %3.1f, %.3f",
			args: ast.NewArray(
				ast.NumberTerm("0.1"), ast.NumberTerm("10.2"), ast.NumberTerm(".5"),
				ast.NumberTerm("0.0"), ast.NumberTerm("100.0"), ast.NumberTerm(".0"),
			),
			exp: "float: 0.100000, 10.2, 0.500, 0.000000, 100.0, 0.000",
		},
		{
			note:   "float in exponent notation",
			format: "%f",
			args:   ast.NewArray(ast.NumberTerm("1e2")),
			exp:    "100.000000",
		},
		{
			note:   "float too large for float64",
			format: "%s",
			args:   ast.NewArray(ast.NumberTerm("1e400")),
			exp:    "1e400",
		},
		{
			// The single argument case is served by an optimized path, and must
			// agree with the general one.
			note:   "float with integer verb",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("1.0")),
			exp:    "%!d(float64=1)",
		},
		{
			note:   "float with integer verb, multiple args",
			format: "%d-%d",
			args:   ast.NewArray(ast.NumberTerm("1.0"), ast.NumberTerm("2")),
			exp:    "%!d(float64=1)-2",
		},
		{
			note:   "string",
			format: "%s",
			args:   ast.NewArray(ast.StringTerm("foo")),
			exp:    "foo",
		},
		{
			note:   "composite value",
			format: "%v",
			args:   ast.NewArray(ast.ArrayTerm(ast.NumberTerm("1"), ast.StringTerm("foo"))),
			exp:    `[1, "foo"]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var result *ast.Term

			operands := []*ast.Term{ast.StringTerm(tc.format), ast.NewTerm(tc.args)}
			err := builtinSprintf(BuiltinContext{}, operands, func(t *ast.Term) error {
				result = t
				return nil
			})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if exp := ast.StringTerm(tc.exp); ast.Compare(exp, result) != 0 {
				t.Fatalf("Expected result:\n\n%s\n\ngot:\n\n%s", exp, result)
			}
		})
	}
}
