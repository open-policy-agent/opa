package main

import (
	"strconv"
	"testing"

	"github.com/mna/pigeon/ast"
)

func compareGrammars(t *testing.T, src string, exp, got *ast.Grammar) bool {
	if (exp.Init != nil) != (got.Init != nil) {
		t.Errorf("%q: want Init? %t, got %t", src, exp.Init != nil, got.Init != nil)
		return false
	}
	if exp.Init != nil {
		if exp.Init.Val != got.Init.Val {
			t.Errorf("%q: want Init %q, got %q", src, exp.Init.Val, got.Init.Val)
			return false
		}
	}

	rn, rm := len(exp.Rules), len(got.Rules)
	if rn != rm {
		t.Errorf("%q: want %d rules, got %d", src, rn, rm)
		return false
	}

	for i, r := range got.Rules {
		if !compareRule(t, src+": "+exp.Rules[i].Name.Val, exp.Rules[i], r) {
			return false
		}
	}

	return true
}

func compareRule(t *testing.T, prefix string, exp, got *ast.Rule) bool {
	if exp.Name.Val != got.Name.Val {
		t.Errorf("%q: want rule name %q, got %q", prefix, exp.Name.Val, got.Name.Val)
		return false
	}
	if (exp.DisplayName != nil) != (got.DisplayName != nil) {
		t.Errorf("%q: want DisplayName? %t, got %t", prefix, exp.DisplayName != nil, got.DisplayName != nil)
		return false
	}
	if exp.DisplayName != nil {
		if exp.DisplayName.Val != got.DisplayName.Val {
			t.Errorf("%q: want DisplayName %q, got %q", prefix, exp.DisplayName.Val, got.DisplayName.Val)
			return false
		}
	}
	return compareExpr(t, prefix, 0, exp.Expr, got.Expr)
}

func compareExpr(t *testing.T, prefix string, ix int, exp, got ast.Expression) bool {
	ixPrefix := prefix + " (" + strconv.Itoa(ix) + ")"

	switch exp := exp.(type) {
	case *ast.ActionExpr:
		got, ok := got.(*ast.ActionExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if (exp.Code != nil) != (got.Code != nil) {
			t.Errorf("%q: want Code?: %t, got %t", ixPrefix, exp.Code != nil, got.Code != nil)
			return false
		}
		if exp.Code != nil {
			if exp.Code.Val != got.Code.Val {
				t.Errorf("%q: want code %q, got %q", ixPrefix, exp.Code.Val, got.Code.Val)
				return false
			}
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.AndCodeExpr:
		got, ok := got.(*ast.AndCodeExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if (exp.Code != nil) != (got.Code != nil) {
			t.Errorf("%q: want Code?: %t, got %t", ixPrefix, exp.Code != nil, got.Code != nil)
			return false
		}
		if exp.Code != nil {
			if exp.Code.Val != got.Code.Val {
				t.Errorf("%q: want code %q, got %q", ixPrefix, exp.Code.Val, got.Code.Val)
				return false
			}
		}

	case *ast.AndExpr:
		got, ok := got.(*ast.AndExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.AnyMatcher:
		got, ok := got.(*ast.AnyMatcher)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		// for completion's sake...
		if exp.Val != got.Val {
			t.Errorf("%q: want value %q, got %q", ixPrefix, exp.Val, got.Val)
		}

	case *ast.CharClassMatcher:
		got, ok := got.(*ast.CharClassMatcher)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if exp.IgnoreCase != got.IgnoreCase {
			t.Errorf("%q: want IgnoreCase %t, got %t", ixPrefix, exp.IgnoreCase, got.IgnoreCase)
			return false
		}
		if exp.Inverted != got.Inverted {
			t.Errorf("%q: want Inverted %t, got %t", ixPrefix, exp.Inverted, got.Inverted)
			return false
		}

		ne, ng := len(exp.Chars), len(got.Chars)
		if ne != ng {
			t.Errorf("%q: want %d Chars, got %d (%v)", ixPrefix, ne, ng, got.Chars)
			return false
		}
		for i, r := range exp.Chars {
			if r != got.Chars[i] {
				t.Errorf("%q: want Chars[%d] %#U, got %#U", ixPrefix, i, r, got.Chars[i])
				return false
			}
		}

		ne, ng = len(exp.Ranges), len(got.Ranges)
		if ne != ng {
			t.Errorf("%q: want %d Ranges, got %d", ixPrefix, ne, ng)
			return false
		}
		for i, r := range exp.Ranges {
			if r != got.Ranges[i] {
				t.Errorf("%q: want Ranges[%d] %#U, got %#U", ixPrefix, i, r, got.Ranges[i])
				return false
			}
		}

		ne, ng = len(exp.UnicodeClasses), len(got.UnicodeClasses)
		if ne != ng {
			t.Errorf("%q: want %d UnicodeClasses, got %d", ixPrefix, ne, ng)
			return false
		}
		for i, s := range exp.UnicodeClasses {
			if s != got.UnicodeClasses[i] {
				t.Errorf("%q: want UnicodeClasses[%d] %q, got %q", ixPrefix, i, s, got.UnicodeClasses[i])
				return false
			}
		}

	case *ast.ChoiceExpr:
		got, ok := got.(*ast.ChoiceExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		ne, ng := len(exp.Alternatives), len(got.Alternatives)
		if ne != ng {
			t.Errorf("%q: want %d Alternatives, got %d", ixPrefix, ne, ng)
			return false
		}

		for i, alt := range exp.Alternatives {
			if !compareExpr(t, prefix, ix+1, alt, got.Alternatives[i]) {
				return false
			}
		}

	case *ast.LabeledExpr:
		got, ok := got.(*ast.LabeledExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if (exp.Label != nil) != (got.Label != nil) {
			t.Errorf("%q: want Label?: %t, got %t", ixPrefix, exp.Label != nil, got.Label != nil)
			return false
		}
		if exp.Label != nil {
			if exp.Label.Val != got.Label.Val {
				t.Errorf("%q: want label %q, got %q", ixPrefix, exp.Label.Val, got.Label.Val)
				return false
			}
		}

		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.LitMatcher:
		got, ok := got.(*ast.LitMatcher)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if exp.IgnoreCase != got.IgnoreCase {
			t.Errorf("%q: want IgnoreCase %t, got %t", ixPrefix, exp.IgnoreCase, got.IgnoreCase)
			return false
		}
		if exp.Val != got.Val {
			t.Errorf("%q: want value %q, got %q", ixPrefix, exp.Val, got.Val)
			return false
		}

	case *ast.NotCodeExpr:
		got, ok := got.(*ast.NotCodeExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if (exp.Code != nil) != (got.Code != nil) {
			t.Errorf("%q: want Code?: %t, got %t", ixPrefix, exp.Code != nil, got.Code != nil)
			return false
		}
		if exp.Code != nil {
			if exp.Code.Val != got.Code.Val {
				t.Errorf("%q: want code %q, got %q", ixPrefix, exp.Code.Val, got.Code.Val)
				return false
			}
		}

	case *ast.NotExpr:
		got, ok := got.(*ast.NotExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.OneOrMoreExpr:
		got, ok := got.(*ast.OneOrMoreExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.RuleRefExpr:
		got, ok := got.(*ast.RuleRefExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		if (exp.Name != nil) != (got.Name != nil) {
			t.Errorf("%q: want Name?: %t, got %t", ixPrefix, exp.Name != nil, got.Name != nil)
			return false
		}
		if exp.Name != nil {
			if exp.Name.Val != got.Name.Val {
				t.Errorf("%q: want name %q, got %q", ixPrefix, exp.Name.Val, got.Name.Val)
				return false
			}
		}

	case *ast.SeqExpr:
		got, ok := got.(*ast.SeqExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		ne, ng := len(exp.Exprs), len(got.Exprs)
		if ne != ng {
			t.Errorf("%q: want %d Exprs, got %d", ixPrefix, ne, ng)
			return false
		}

		for i, expr := range exp.Exprs {
			if !compareExpr(t, prefix, ix+1, expr, got.Exprs[i]) {
				return false
			}
		}

	case *ast.ZeroOrMoreExpr:
		got, ok := got.(*ast.ZeroOrMoreExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	case *ast.ZeroOrOneExpr:
		got, ok := got.(*ast.ZeroOrOneExpr)
		if !ok {
			t.Errorf("%q: want expression type %T, got %T", ixPrefix, exp, got)
			return false
		}
		return compareExpr(t, prefix, ix+1, exp.Expr, got.Expr)

	default:
		t.Fatalf("unexpected expression type %T", exp)
	}
	return true
}
