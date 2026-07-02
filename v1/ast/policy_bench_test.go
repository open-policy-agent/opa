package ast

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast/location"
)

// 12.03 ns/op	      48 B/op	       1 allocs/op
func BenchmarkCommentString(b *testing.B) {
	comment := &Comment{
		Text:     []byte("This is a sample comment for benchmarking."),
		Location: &location.Location{},
	}

	for b.Loop() {
		_ = comment.String()
	}
}

// Before using TextAppenders and pre-allocating buffers:
// simple_expr-16                              6196365       181.1 ns/op      64 B/op       4 allocs/op
// negated_expr_with_with_modifier-16          3400506       353.3 ns/op     200 B/op       9 allocs/op
// complex_expr-16                             1584336       760.4 ns/op     673 B/op      20 allocs/op
// Now:
// simple_expr-16                              8809898       120.9 ns/op      24 B/op       1 allocs/op
// negated_expr_with_with_modifier-16          5300791       227.3 ns/op      48 B/op       1 allocs/op
// complex_expr-16                             3071887       382.8 ns/op      96 B/op       1 allocs/op
func BenchmarkExprString(b *testing.B) {
	tests := []struct {
		note string
		expr *Expr
	}{
		{
			note: "simple expr",
			expr: MustParseExpr(`input.x == 10`),
		},
		{
			note: "negated expr with with modifier",
			expr: MustParseExpr(`not input.y != "hello" with input.z as 5`),
		},
		{
			note: "complex expr",
			expr: MustParseExpr(`count({x | x := input.arr[_]; x > 10}) == 3 with input.arr as [5, 15, 25, 8, 30]`),
		},
	}

	for _, tc := range tests {
		b.Run(tc.note, func(b *testing.B) {
			for b.Loop() {
				_ = tc.expr.String()
			}
		})
	}
}

// All zero allocs
func BenchmarkExprAppendText(b *testing.B) {
	tests := []struct {
		note string
		expr *Expr
	}{
		{
			note: "simple expr",
			expr: MustParseExpr(`input.x == 10`),
		},
		{
			note: "negated expr with with modifier",
			expr: MustParseExpr(`not input.y != "hello" with input.z as 5`),
		},
		{
			note: "complex expr",
			expr: MustParseExpr(`count({x | x := input.arr[_]; x > 10}) == 3 with input.arr as [5, 15, 25, 8, 30]`),
		},
	}

	for _, tc := range tests {
		b.Run(tc.note, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			for b.Loop() {
				buf, _ = tc.expr.AppendText(buf)
				buf = buf[:0]
			}
		})
	}
}

func BenchmarkExprMarshalJSON(b *testing.B) {
	tests := []struct {
		note string
		expr *Expr
	}{
		{
			note: "simple expr",
			expr: MustParseExpr(`input.x == 10`),
		},
		{
			note: "negated expr with with modifier",
			expr: MustParseExpr(`not input.y != "hello" with input.z as 5`),
		},
		{
			note: "complex expr",
			expr: MustParseExpr(`count({x | x := input.arr[_]; x > 10}) == 3 with input.arr as [5, 15, 25, 8, 30]`),
		},
	}
	for _, tc := range tests {
		b.Run(tc.note, func(b *testing.B) {
			for b.Loop() {
				_, _ = json.Marshal(tc.expr)
			}
		})
	}
}

func BenchmarkRuleMarshalJSON(b *testing.B) {
	module := MustParseModule(`
		package test

		allow if { input.user == "admin" }

		deny if {
			input.action == "delete"
			not input.admin
		}

		complex_rule if {
			some i
			input.items[i].value > 100
			input.items[i].enabled
		}
	`)

	tests := []struct {
		note string
		rule *Rule
	}{
		{
			note: "simple rule",
			rule: module.Rules[0],
		},
		{
			note: "rule with multiple exprs",
			rule: module.Rules[1],
		},
		{
			note: "rule with some decl",
			rule: module.Rules[2],
		},
	}

	for _, tc := range tests {
		b.Run(tc.note, func(b *testing.B) {
			for b.Loop() {
				_, _ = json.Marshal(tc.rule)
			}
		})
	}
}

func BenchmarkWithMarshalJSON(b *testing.B) {
	module := MustParseModule(`
		package test
		allow if { input.x with input as {"x": true} }
	`)

	with := module.Rules[0].Body[0].With[0]

	for b.Loop() {
		_, _ = json.Marshal(with)
	}
}
