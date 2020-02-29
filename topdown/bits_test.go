// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"math"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestBuiltinBitsOr(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic bitwise-or", []string{`p[x] { x := bits.or(7, 9) }`}, `[15]`},
		{"or with zero is value", []string{`p[x] { x := bits.or(50, 0) }`}, `[50]`},
		{"lhs (float) error", []string{`p = x { x := bits.or(7.2, 42) }`}, &Error{Code: TypeErr, Message: "bits.or: operand 1 must be integer number but got floating-point number"}},
		{
			"rhs (wrong type-type) error",
			[]string{`p = x { x := bits.or(7, "hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.or: invalid argument(s)")},
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinBitsAnd(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic bitwise-and", []string{`p[x] { x := bits.and(7, 9) }`}, `[1]`},
		{"and with zero is and", []string{`p[x] { x := bits.and(50, 0) }`}, `[0]`},
		{"lhs (float) error", []string{`p = x { x := bits.and(7.2, 42) }`}, &Error{Code: TypeErr, Message: "bits.and: operand 1 must be integer number but got floating-point number"}},
		{
			"rhs (wrong type-type) error",
			[]string{`p = x { x := bits.and(7, "hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.and: invalid argument(s)")},
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinBitsNegate(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic bitwise-negate", []string{`p[x] { x := bits.negate(42) }`}, `[-43]`},
		{"float error", []string{`p = x { x := bits.negate(7.2) }`}, &Error{Code: TypeErr, Message: "bits.negate: operand 1 must be integer number but got floating-point number"}},
		{
			"type error",
			[]string{`p = x { x := bits.negate("hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.negate: invalid argument(s)")},
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinBitsXOr(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic bitwise-xor", []string{`p[x] { x := bits.xor(42, 3) }`}, `[41]`},
		{"xor same is 0", []string{`p[x] { x := bits.xor(42, 42) }`}, `[0]`},
		{"lhs (float) error", []string{`p = x { x := bits.xor(7.2, 42) }`}, &Error{Code: TypeErr, Message: "bits.xor: operand 1 must be integer number but got floating-point number"}},
		{
			"rhs (wrong type-type) error",
			[]string{`p = x { x := bits.xor(7, "hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.xor: invalid argument(s)")},
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinBitsShiftLeft(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic shift-left", []string{`p[x] { x := bits.lsh(1, 3) }`}, `[8]`},
		{"lhs (float) error", []string{`p = x { x := bits.lsh(7.2, 42) }`}, &Error{Code: TypeErr, Message: "bits.lsh: operand 1 must be integer number but got floating-point number"}},
		{
			"rhs (wrong type-type) error",
			[]string{`p = x { x := bits.lsh(7, "hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.lsh: invalid argument(s)")},
		},
		{"rhs must be unsigned", []string{`p = x { x := bits.lsh(7, -1) }`}, &Error{Code: TypeErr, Message: "bits.lsh: operand 2 must be an unsigned integer number but got a negative integer"}},
		{
			"shift of max int32 doesn't overflow",
			[]string{fmt.Sprintf(`p = x { x := bits.lsh(%d, 1) }`, math.MaxInt32)},
			`4294967294`,
		},
		{
			"shift of max int64 doesn't overflow, but it's lossy do to conversion to exponent type (see discussion in #2160)",
			[]string{fmt.Sprintf(`p = x { x := bits.lsh(%d, 1) }`, math.MaxInt64)},
			`18446744074000000000`,
		},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestBuiltinBitsShiftRight(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"basic shift-right", []string{`p[x] { x := bits.rsh(8, 3) }`}, `[1]`},
		{"lhs (float) error", []string{`p = x { x := bits.rsh(7.2, 42) }`}, &Error{Code: TypeErr, Message: "bits.rsh: operand 1 must be integer number but got floating-point number"}},
		{
			"rhs (wrong type-type) error",
			[]string{`p = x { x := bits.rsh(7, "hi") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "bits.rsh: invalid argument(s)")},
		},
		{"rhs must be unsigned", []string{`p = x { x := bits.rsh(7, -1) }`}, &Error{Code: TypeErr, Message: "bits.rsh: operand 2 must be an unsigned integer number but got a negative integer"}},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}
